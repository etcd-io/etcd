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
	"time"

	"go.uber.org/zap"
)

func (s *store) scheduleCompaction(compactMainRev int64, keep map[revision]struct{}) bool {
	totalStart := time.Now()
	defer func() { dbCompactionTotalMs.Observe(float64(time.Since(totalStart) / time.Millisecond)) }()
	keyCompactions := 0
	defer func() { dbCompactionKeysCounter.Add(float64(keyCompactions)) }()

	end := make([]byte, 8)
	binary.BigEndian.PutUint64(end, uint64(compactMainRev+1))

	batchsize := int64(1000)
	last := make([]byte, 8+1+8)

	compactKeys := make([][]byte, 0, batchsize)
	for {
		var rev revision

		start := time.Now()
		rtx := s.b.ConcurrentReadTx()
		rtx.RLock()

		compactKeys = compactKeys[:0]
		batchCompactions := 0
		// Find keys to remove in read transaction to maximize concurrency.
		keys, _ := rtx.UnsafeRange(keyBucketName, last, end, batchsize)
		for _, key := range keys {
			rev = bytesToRev(key)
			if _, ok := keep[rev]; !ok {
				ckey := make([]byte, len(key))
				copy(ckey, key)
				compactKeys = append(compactKeys, ckey)
				batchCompactions++
			}
		}
		keyCompactions += batchCompactions
		revToBytes(revision{main: rev.main, sub: rev.sub + 1}, last)
		rtx.RUnlock()

		if len(compactKeys) > 0 {
			batchStart := time.Now()
			err := s.b.Compact(keyBucketName, compactKeys)
			if err != nil {
				if s.lg != nil {
					s.lg.Error(
						"compaction failed",
						zap.Int64("compact-revision", compactMainRev),
					)
				} else {
					plog.Errorf("compaction failed for %d: %v", compactMainRev, err)
				}
			}
			if s.lg != nil {
				s.lg.Info(
					"finished scheduled compaction batch",
					zap.Int64("compact-revision", compactMainRev),
					zap.Int("key-count", batchCompactions),
					zap.Duration("took", time.Since(totalStart)),
				)
			} else {
				plog.Printf("finished scheduled compaction batch of %d keys at %d (took %v)", batchCompactions, compactMainRev, time.Since(batchStart))
			}
		}

		if len(keys) < int(batchsize) {
			tx := s.b.BatchTx()
			tx.Lock()
			rbytes := make([]byte, 8+1+8)
			revToBytes(revision{main: compactMainRev}, rbytes)
			tx.UnsafePut(metaBucketName, finishedCompactKeyName, rbytes)
			tx.Unlock()
			if s.lg != nil {
				s.lg.Info(
					"finished scheduled compaction",
					zap.Int64("compact-revision", compactMainRev),
					zap.Int("key-count", keyCompactions),
					zap.Duration("took", time.Since(totalStart)),
				)
			} else {
				plog.Printf("finished scheduled compaction of %d keys at %d (took %v)", keyCompactions, compactMainRev, time.Since(totalStart))
			}
			return true
		}

		dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))

		select {
		case <-time.After(10 * time.Millisecond):
		case <-s.stopc:
			return false
		}
	}
}
