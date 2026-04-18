// Copyright 2022 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mvcc

import (
	"hash"
	"hash/crc32"
	"sort"
	"sync"

	"go.uber.org/zap"

	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

const (
	hashStorageMaxSize = 10
)

func unsafeHashByRev(tx backend.UnsafeReader, compactRevision, revision int64, keep map[Revision]struct{}) (KeyValueHash, error) {
	h := newKVHasher(compactRevision, revision, keep)
	err := tx.UnsafeForEach(schema.Key, func(k, v []byte) error {
		h.WriteKeyValue(k, v)
		return nil
	})
	return h.Hash(), err
}

type kvHasher struct {
	hash            hash.Hash32
	compactRevision int64
	revision        int64
	keep            map[Revision]struct{}
}

func newKVHasher(compactRev, rev int64, keep map[Revision]struct{}) kvHasher {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	h.Write(schema.Key.Name())
	return kvHasher{
		hash:            h,
		compactRevision: compactRev,
		revision:        rev,
		keep:            keep,
	}
}

// shouldWriteKeyValue determines whether a key-value pair should be included in the hash calculation
func (h *kvHasher) shouldWriteKeyValue(k []byte) bool {
	kr := BytesToRev(k)
	upper := Revision{Main: h.revision + 1}
	if !upper.GreaterThan(kr) {
		return false
	}

	isTombstone := BytesToBucketKey(k).tombstone

	lower := Revision{Main: h.compactRevision + 1}
	// skip revisions that are scheduled for deletion
	// due to compacting; don't skip if there isn't one.
	if lower.GreaterThan(kr) && len(h.keep) > 0 {
		if _, ok := h.keep[kr]; !ok {
			return false
		}
	}

	// When performing compaction, if the compacted revision is a
	// tombstone, older versions (<= 3.5.15 or <= 3.4.33) will delete
	// the tombstone. But newer versions (> 3.5.15 or > 3.4.33) won't
	// delete it. So we should skip the tombstone in such cases when
	// computing the hash to ensure that both older and newer versions
	// can always generate the same hash values.
	if kr.Main == h.compactRevision && isTombstone {
		return false
	}

	return true
}

func (h *kvHasher) WriteKeyValue(k, v []byte) {
	if !h.shouldWriteKeyValue(k) {
		return
	}

	h.hash.Write(k)
	h.hash.Write(v)
}

func (h *kvHasher) Hash() KeyValueHash {
	return KeyValueHash{Hash: h.hash.Sum32(), CompactRevision: h.compactRevision, Revision: h.revision}
}

type KeyValueHash struct {
	Hash            uint32
	CompactRevision int64
	Revision        int64
}

// KeyRevisionHash represents the hash of a single key at a specific revision
type KeyRevisionHash struct {
	Key    string `json:"key"`
	Hash   uint32 `json:"hash"`
	ModRev int64  `json:"modRevision"`
}

// DetailedHashResult contains detailed hash calculation results
type DetailedHashResult struct {
	TotalHash       KeyValueHash
	KeyHashes       []KeyRevisionHash
	CompactRevision int64
	Revision        int64
	KeyCount        int
}

type HashStorage interface {
	// Hash computes the hash of the whole backend keyspace,
	// including key, lease, and other buckets in storage.
	// This is designed for testing ONLY!
	// Do not rely on this in production with ongoing transactions,
	// since Hash operation does not hold MVCC locks.
	// Use "HashByRev" method instead for "key" bucket consistency checks.
	Hash() (hash uint32, revision int64, err error)

	// HashByRev computes the hash of all MVCC revisions up to a given revision.
	HashByRev(rev int64) (hash KeyValueHash, currentRev int64, err error)

	// HashByRevWithCompactRev computes the hash of all MVCC revisions up to a given revision
	// with an optional custom compact revision. If compactRev <= 0, uses the store's compact revision.
	HashByRevWithCompactRev(rev int64, compactRev int64) (hash KeyValueHash, currentRev int64, err error)

	// HashByRevDetailed computes the hash of all MVCC revisions up to a given revision and compact revision
	// returns detailed information including individual key hashes.
	HashByRevDetailed(rev int64, compactRev int64) (result DetailedHashResult, currentRev int64, err error)

	// Store adds hash value in local cache, allowing it to be returned by HashByRev.
	Store(valueHash KeyValueHash)

	// Hashes returns list of up to `hashStorageMaxSize` newest previously stored hashes.
	Hashes() []KeyValueHash
}

type hashStorage struct {
	store  *store
	hashMu sync.RWMutex
	hashes []KeyValueHash
	lg     *zap.Logger
}

func NewHashStorage(lg *zap.Logger, s *store) HashStorage {
	return &hashStorage{
		store: s,
		lg:    lg,
	}
}

func (s *hashStorage) Hash() (hash uint32, revision int64, err error) {
	return s.store.hash()
}

func (s *hashStorage) HashByRev(rev int64) (KeyValueHash, int64, error) {
	return s.HashByRevWithCompactRev(rev, 0)
}

func (s *hashStorage) HashByRevWithCompactRev(rev int64, compactRev int64) (KeyValueHash, int64, error) {
	s.hashMu.RLock()
	for _, h := range s.hashes {
		// For cache, only return cached results when both revision and compactRev match
		if rev == h.Revision && (compactRev <= 0 || compactRev == h.CompactRevision) {
			s.hashMu.RUnlock()

			s.store.revMu.RLock()
			currentRev := s.store.currentRev
			s.store.revMu.RUnlock()
			return h, currentRev, nil
		}
	}
	s.hashMu.RUnlock()

	return s.store.hashByRevWithCompactRev(rev, compactRev)
}

func (s *hashStorage) HashByRevDetailed(rev int64, compactRev int64) (DetailedHashResult, int64, error) {
	return s.store.hashByRevDetailed(rev, compactRev)
}

func (s *hashStorage) Store(hash KeyValueHash) {
	s.lg.Info("storing new hash",
		zap.Uint32("hash", hash.Hash),
		zap.Int64("revision", hash.Revision),
		zap.Int64("compact-revision", hash.CompactRevision),
	)
	s.hashMu.Lock()
	defer s.hashMu.Unlock()
	s.hashes = append(s.hashes, hash)
	sort.Slice(s.hashes, func(i, j int) bool {
		return s.hashes[i].Revision < s.hashes[j].Revision
	})
	if len(s.hashes) > hashStorageMaxSize {
		s.hashes = s.hashes[len(s.hashes)-hashStorageMaxSize:]
	}
}

func (s *hashStorage) Hashes() []KeyValueHash {
	s.hashMu.RLock()
	// Copy out hashes under lock just to be safe
	hashes := make([]KeyValueHash, 0, len(s.hashes))
	hashes = append(hashes, s.hashes...)
	s.hashMu.RUnlock()
	return hashes
}

// kvHasherDetailed supports detailed hash calculation, including individual key hashes
type kvHasherDetailed struct {
	kvHasher
	keyHashes []KeyRevisionHash
}

func newKVHasherDetailed(compactRev, rev int64, keep map[Revision]struct{}) *kvHasherDetailed {
	return &kvHasherDetailed{
		kvHasher:  newKVHasher(compactRev, rev, keep),
		keyHashes: make([]KeyRevisionHash, 0),
	}
}

func (h *kvHasherDetailed) WriteKeyValue(k, v []byte) {
	if !h.shouldWriteKeyValue(k) {
		return
	}

	// Calculate hash for individual key+value
	keyHash := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	keyHash.Write(k)
	keyHash.Write(v)

	var rkv revKeyValue
	if err := rkv.kv.Unmarshal(v); err != nil {
		return
	}

	// Add to detailed hash list
	keyRevHash := KeyRevisionHash{
		Key:    string(rkv.kv.Key),
		Hash:   keyHash.Sum32(),
		ModRev: BytesToRev(k).Main,
	}
	h.keyHashes = append(h.keyHashes, keyRevHash)

	h.hash.Write(k)
	h.hash.Write(v)
}

func (h *kvHasherDetailed) KeyHashes() []KeyRevisionHash {
	return h.keyHashes
}

// unsafeHashByRevDetailed computes detailed hash information, including individual key hashes
//
// WARNING: This function loads all key-value pairs into memory to compute individual hashes.
// It may consume significant memory for large datasets. Use this method only for
// debugging, testing, or tooling purposes, not in production environments with large datasets.
func unsafeHashByRevDetailed(tx backend.UnsafeReader, compactRevision, revision int64, keep map[Revision]struct{}) (DetailedHashResult, error) {
	h := newKVHasherDetailed(compactRevision, revision, keep)
	err := tx.UnsafeForEach(schema.Key, func(k, v []byte) error {
		h.WriteKeyValue(k, v)
		return nil
	})

	if err != nil {
		return DetailedHashResult{}, err
	}

	totalHash := h.Hash()
	keyHashes := h.KeyHashes()

	return DetailedHashResult{
		TotalHash:       totalHash,
		KeyHashes:       keyHashes,
		CompactRevision: compactRevision,
		Revision:        revision,
		KeyCount:        len(keyHashes),
	}, nil
}
