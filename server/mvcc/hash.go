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

	"go.etcd.io/etcd/server/v3/mvcc/backend"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
	"go.uber.org/zap"
)

const (
	hashStorageMaxSize = 10
)

func unsafeHashByRev(tx backend.ReadTx, compactRevision, revision int64, keep map[revision]struct{}) (KeyValueHash, error) {
	h := newKVHasher(compactRevision, revision, keep)
	err := tx.UnsafeForEach(buckets.Key, func(k, v []byte) error {
		h.WriteKeyValue(k, v)
		return nil
	})
	return h.Hash(), err
}

type kvHasher struct {
	hash            hash.Hash32
	compactRevision int64
	revision        int64
	keep            map[revision]struct{}
}

func newKVHasher(compactRev, rev int64, keep map[revision]struct{}) kvHasher {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	h.Write(buckets.Key.Name())
	return kvHasher{
		hash:            h,
		compactRevision: compactRev,
		revision:        rev,
		keep:            keep,
	}
}

func (h *kvHasher) WriteKeyValue(k, v []byte) {
	kr := bytesToRev(k)
	upper := revision{main: h.revision + 1}
	if !upper.GreaterThan(kr) {
		return
	}
	lower := revision{main: h.compactRevision + 1}
	// skip revisions that are scheduled for deletion
	// due to compacting; don't skip if there isn't one.
	if lower.GreaterThan(kr) && len(h.keep) > 0 {
		if _, ok := h.keep[kr]; !ok {
			return
		}
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

type HashStorage interface {
	// Hash computes the hash of the KV's backend.
	Hash() (hash uint32, revision int64, err error)

	// HashByRev computes the hash of all MVCC revisions up to a given revision.
	HashByRev(rev int64) (hash KeyValueHash, currentRev int64, err error)

	// Store adds hash value in local cache, allowing it can be returned by HashByRev.
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

func newHashStorage(lg *zap.Logger, s *store) *hashStorage {
	return &hashStorage{
		store: s,
		lg:    lg,
	}
}

func (s *hashStorage) Hash() (hash uint32, revision int64, err error) {
	return s.store.hash()
}

func (s *hashStorage) HashByRev(rev int64) (KeyValueHash, int64, error) {
	s.hashMu.RLock()
	for _, h := range s.hashes {
		if rev == h.Revision {
			s.hashMu.RUnlock()

			s.store.revMu.RLock()
			currentRev := s.store.currentRev
			s.store.revMu.RUnlock()
			return h, currentRev, nil
		}
	}
	s.hashMu.RUnlock()

	return s.store.hashByRev(rev)
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
	for _, hash := range s.hashes {
		hashes = append(hashes, hash)
	}
	s.hashMu.RUnlock()
	return hashes
}
