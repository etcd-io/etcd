// Copyright 2015 The etcd Authors
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
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"

	"go.etcd.io/etcd/server/v3/storage/schema"
)

func (s *store) scheduleCompaction(compactMainRev, prevCompactRev int64) (KeyValueHash, error) {
	totalStart := time.Now()
	keep, _ := s.kvindex.Compact(compactMainRev)
	indexCompactionPauseMs.Observe(float64(time.Since(totalStart) / time.Millisecond))

	totalStart = time.Now()
	defer func() { dbCompactionTotalMs.Observe(float64(time.Since(totalStart) / time.Millisecond)) }()
	keyCompactions := 0
	defer func() { dbCompactionKeysCounter.Add(float64(keyCompactions)) }()
	defer func() { dbCompactionLast.Set(float64(time.Now().Unix())) }()

	end := make([]byte, 8)
	binary.BigEndian.PutUint64(end, uint64(compactMainRev+1))

	batchNum := s.cfg.CompactionBatchLimit
	h := newKVHasher(prevCompactRev, compactMainRev, keep)
	last := make([]byte, 8+1+8)
	for {
		var rev Revision

		start := time.Now()

		tx := s.b.BatchTx()
		tx.LockOutsideApply()
		// gofail: var compactAfterAcquiredBatchTxLock struct{}
		keys, values := tx.UnsafeRange(schema.Key, last, end, int64(batchNum))
		for i := range keys {
			rev = BytesToRev(keys[i])
			if _, ok := keep[rev]; !ok {
				tx.UnsafeDelete(schema.Key, keys[i])
				keyCompactions++
			}
			h.WriteKeyValue(keys[i], values[i])
		}

		if len(keys) < batchNum {
			// gofail: var compactBeforeSetFinishedCompact struct{}
			UnsafeSetFinishedCompact(tx, compactMainRev)
			tx.Unlock()
			dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))
			// gofail: var compactAfterSetFinishedCompact struct{}
			hash := h.Hash()
			size, sizeInUse := s.b.Size(), s.b.SizeInUse()
			s.lg.Info(
				"finished scheduled compaction",
				zap.Int64("compact-revision", compactMainRev),
				zap.Duration("took", time.Since(totalStart)),
				zap.Int("number-of-keys-compacted", keyCompactions),
				zap.Uint32("hash", hash.Hash),
				zap.Int64("current-db-size-bytes", size),
				zap.String("current-db-size", humanize.Bytes(uint64(size))),
				zap.Int64("current-db-size-in-use-bytes", sizeInUse),
				zap.String("current-db-size-in-use", humanize.Bytes(uint64(sizeInUse))),
			)
			return hash, nil
		}

		tx.Unlock()
		// update last
		last = RevToBytes(Revision{Main: rev.Main, Sub: rev.Sub + 1}, last)
		// Immediately commit the compaction deletes instead of letting them accumulate in the write buffer
		// gofail: var compactBeforeCommitBatch struct{}
		s.b.ForceCommit()
		// gofail: var compactAfterCommitBatch struct{}
		dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))

		select {
		case <-time.After(s.cfg.CompactionSleepInterval):
		case <-s.stopc:
			return KeyValueHash{}, fmt.Errorf("interrupted due to stop signal")
		}
	}
}

func (s *store) scheduleCompactionRangeFree(compactMainRev, prevCompactRev int64) (KeyValueHash, error) {
	totalStart := time.Now()
	// Use Compact to get revisions to delete and perform index compaction
	keep, toDelete := s.kvindex.Compact(compactMainRev)
	indexCompactionPauseMs.Observe(float64(time.Since(totalStart) / time.Millisecond))

	totalStart = time.Now()
	defer func() { dbCompactionTotalMs.Observe(float64(time.Since(totalStart) / time.Millisecond)) }()
	keyCompactions := 0
	defer func() { dbCompactionKeysCounter.Add(float64(keyCompactions)) }()
	defer func() { dbCompactionLast.Set(float64(time.Now().Unix())) }()

	batchNum := s.cfg.CompactionBatchLimit
	h := newKVHasher(prevCompactRev, compactMainRev, keep)

	// Convert toDelete map to sorted slice for deterministic iteration
	revs := make([]Revision, 0, len(toDelete))
	for rev := range toDelete {
		revs = append(revs, rev)
	}
	// Sort revisions by main and sub revision
	sort.Slice(revs, func(i, j int) bool {
		if revs[i].Main != revs[j].Main {
			return revs[i].Main < revs[j].Main
		}
		return revs[i].Sub < revs[j].Sub
	})

	// Start a goroutine to compute hash in parallel using ReadTx
	kvHashCh := make(chan KeyValueHash, 1)
	// The underlying bbolt read tx should be able to see all the revisions at the moment,
	// including the revisions that will be deleted in compaction.
	readTx := s.b.ReadTx()
	readTx.RLock()
	go func(ch chan<- KeyValueHash) {
		defer readTx.RUnlock()
		end := make([]byte, 8)
		binary.BigEndian.PutUint64(end, uint64(compactMainRev+1))
		last := make([]byte, 8+1+8)

		for {
			keys, values := readTx.UnsafeRange(schema.Key, last, end, int64(batchNum))

			for i := range keys {
				h.WriteKeyValue(keys[i], values[i])
			}

			// last batch
			if len(keys) < batchNum {
				break
			}

			// update last
			rev := BytesToRev(keys[len(keys)-1])
			last = RevToBytes(Revision{Main: rev.Main, Sub: rev.Sub + 1}, last)
			readTx = s.b.ReadTx()
		}

		ch <- h.Hash()
	}(kvHashCh)

	// Process deletions in batches
	deletedCount := 0
	start := time.Now()
	tx := s.b.BatchTx()
	tx.LockOutsideApply()
	// gofail: var compactAfterAcquiredBatchTxLock struct{}
	keyBytes := make([]byte, 8+1+8)

	for _, rev := range revs {
		// Delete the revision directly
		RevToBytes(rev, keyBytes)
		tx.UnsafeDelete(schema.Key, keyBytes)
		keyCompactions++
		deletedCount++

		// Commit in batches
		if deletedCount%batchNum == 0 {
			tx.Unlock()
			// gofail: var compactBeforeCommitBatch struct{}
			s.b.ForceCommit()
			// gofail: var compactAfterCommitBatch struct{}
			dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))

			select {
			case <-time.After(s.cfg.CompactionSleepInterval):
			case <-s.stopc:
				return KeyValueHash{}, fmt.Errorf("interrupted due to stop signal")
			}

			// Start a new batch
			start = time.Now()
			tx = s.b.BatchTx()
			tx.LockOutsideApply()
		}
	}

	// gofail: var compactBeforeSetFinishedCompact struct{}
	UnsafeSetFinishedCompact(tx, compactMainRev)
	tx.Unlock()
	dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))
	// gofail: var compactAfterSetFinishedCompact struct{}

	// Wait for hash computation to complete
	hash := <-kvHashCh

	size, sizeInUse := s.b.Size(), s.b.SizeInUse()
	s.lg.Info(
		"finished scheduled compaction",
		zap.Int64("compact-revision", compactMainRev),
		zap.Duration("took", time.Since(totalStart)),
		zap.Int("number-of-keys-compacted", keyCompactions),
		zap.Uint32("hash", hash.Hash),
		zap.Int64("current-db-size-bytes", size),
		zap.String("current-db-size", humanize.Bytes(uint64(size))),
		zap.Int64("current-db-size-in-use-bytes", sizeInUse),
		zap.String("current-db-size-in-use", humanize.Bytes(uint64(sizeInUse))),
	)
	return hash, nil
}
