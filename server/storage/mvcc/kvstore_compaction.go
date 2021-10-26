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
	"go.uber.org/zap"
	"time"

	"go.etcd.io/etcd/server/v3/storage/schema"
)

func (s *store) scheduleCompaction(compactMainRev int64, keep map[revision]struct{}) bool {
	totalStart := time.Now()
	defer func() { dbCompactionTotalMs.Observe(float64(time.Since(totalStart) / time.Millisecond)) }()
	s.keyCompactions = 0
	defer func() { dbCompactionKeysCounter.Add(float64(s.keyCompactions)) }()
	defer func() { dbCompactionLast.Set(float64(time.Now().Unix())) }()

	if s.lastCompactMainRev == -1 {
		s.lg.Info("begin scheduleCompactionNewest")
		if !s.scheduleCompactionNewest(0, compactMainRev, keep) {
			return false
		}
	} else {
		s.lg.Info("begin scheduleCompactionOld")
		if !s.scheduleCompactionOld(keep) {
			return false
		}
		if !s.scheduleCompactionNewest(s.lastCompactMainRev, compactMainRev, keep) {
			return false
		}
	}

	s.lg.Info(
		"finished scheduled compaction",
		zap.Int64("compact-revision", compactMainRev),
		zap.Int64("compact-keys", s.keyCompactions),
		zap.Duration("took", time.Since(totalStart)),
	)

	s.lastKeep = s.lastKeep[:0]
	for key := range keep {
		s.lastKeep = append(s.lastKeep, key)
	}
	s.lastCompactMainRev = compactMainRev

	return true
}

func (s *store) scheduleCompactionOld(keep map[revision]struct{}) bool {
	batchNum := s.cfg.CompactionBatchLimit / 10
	batchInterval := s.cfg.CompactionSleepInterval

	lastIndex := 0
	keyByte := make([]byte, 8+1+8)
	for {
		start := time.Now()

		tx := s.b.BatchTx()
		tx.Lock()

		num := 0
		for lastIndex < len(s.lastKeep) {
			key := s.lastKeep[lastIndex]
			revToBytes(key, keyByte)
			if _, ok := keep[key]; !ok {
				tx.UnsafeDelete(schema.Key, keyByte)
				s.keyCompactions++
			}
			num++
			lastIndex++
			if num >= batchNum {
				break
			}
		}

		tx.Unlock()
		// Immediately commit the compaction deletes instead of letting them accumulate in the write buffer
		s.b.ForceCommit()

		dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))

		if lastIndex >= len(s.lastKeep) {
			return true
		}

		select {
		case <-time.After(batchInterval):
		case <-s.stopc:
			return false
		}
	}
}

func (s *store) scheduleCompactionNewest(lastCompactRev, compactMainRev int64, keep map[revision]struct{}) bool {
	end := make([]byte, 8)
	binary.BigEndian.PutUint64(end, uint64(compactMainRev+1))

	batchNum := s.cfg.CompactionBatchLimit
	batchInterval := s.cfg.CompactionSleepInterval

	last := make([]byte, 8+1+8)
	revToBytes(revision{main: lastCompactRev, sub: 0}, last)

	for {
		var rev revision

		start := time.Now()

		tx := s.b.BatchTx()
		tx.Lock()
		keys, _ := tx.UnsafeRange(schema.Key, last, end, int64(batchNum))
		for _, key := range keys {
			rev = bytesToRev(key)
			if _, ok := keep[rev]; !ok {
				tx.UnsafeDelete(schema.Key, key)
				s.keyCompactions++
			}
		}

		if len(keys) < batchNum {
			UnsafeSetFinishedCompact(tx, compactMainRev)
			tx.Unlock()
			return true
		}

		// update last
		revToBytes(revision{main: rev.main, sub: rev.sub + 1}, last)
		tx.Unlock()
		// Immediately commit the compaction deletes instead of letting them accumulate in the write buffer
		s.b.ForceCommit()
		dbCompactionPauseMs.Observe(float64(time.Since(start) / time.Millisecond))

		select {
		case <-time.After(batchInterval):
		case <-s.stopc:
			return false
		}
	}
}
