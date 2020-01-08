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
	"time"

	"go.uber.org/zap"
)

func (s *store) scheduleCompaction(compactMainRev int64, unwanted map[revision]struct{}, unwantedTomb map[revision]struct{}) bool {
	totalStart := time.Now()
	defer func() { dbCompactionTotalMs.Observe(float64(time.Since(totalStart) / time.Millisecond)) }()
	keyCompactions := 0
	defer func() { dbCompactionKeysCounter.Add(float64(keyCompactions)) }()

	compactKeys := make([][]byte, 0)
	// aggregate the normal unwanted kv and unwanted tomb kv
	for rev := range unwanted {
		rbytes := newRevBytes()
		revToBytes(rev, rbytes)
		compactKeys = append(compactKeys, rbytes)
	}
	for rev := range unwantedTomb {
		rbytes := newRevBytes()
		revToBytes(rev, rbytes)
		ibytes := appendMarkTombstone(nil, rbytes)
		compactKeys = append(compactKeys, ibytes)
	}

	startIndex := 0
	for {
		start := time.Now()
		tx := s.b.BatchTx()
		tx.Lock()
		batchNum := 0

		for i := startIndex; i < len(compactKeys); i++ {
			tx.UnsafeDelete(keyBucketName, compactKeys[i])
			keyCompactions++
			batchNum++

			if batchNum == s.cfg.CompactionBatchLimit {
				startIndex = i + 1
				break
			}
		}

		if batchNum < s.cfg.CompactionBatchLimit {
			rbytes := make([]byte, 8+1+8)
			revToBytes(revision{main: compactMainRev}, rbytes)
			tx.UnsafePut(metaBucketName, finishedCompactKeyName, rbytes)
			tx.Unlock()
			s.lg.Info(
				"finished scheduled compaction",
				zap.Int64("compact-revision", compactMainRev),
				zap.Duration("took", time.Since(totalStart)),
			)
			return true
		}

		tx.Unlock()
		// Immediately commit the compaction deletes instead of letting them accumulate in the write buffer
		s.b.ForceCommit()
		dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))

		select {
		case <-time.After(10 * time.Millisecond):
		case <-s.stopc:
			return false
		}
	}
}
