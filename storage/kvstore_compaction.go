package storage

import (
	"encoding/binary"
	"time"
)

func (s *store) scheduleCompaction(compactMainRev int64, keep map[reversion]struct{}) {
	end := make([]byte, 8)
	binary.BigEndian.PutUint64(end, uint64(compactMainRev+1))

	batchsize := int64(10000)
	last := make([]byte, 8+1+8)
	for {
		var rev reversion

		tx := s.b.BatchTx()
		tx.Lock()

		keys, _ := tx.UnsafeRange(keyBucketName, last, end, batchsize)
		for _, key := range keys {
			rev = bytesToRev(key)
			if _, ok := keep[rev]; !ok {
				tx.UnsafeDelete(keyBucketName, key)
			}
		}

		if len(keys) == 0 {
			rbytes := make([]byte, 8+1+8)
			revToBytes(reversion{main: compactMainRev}, rbytes)
			tx.UnsafePut(metaBucketName, finishedCompactKeyName, rbytes)
			tx.Unlock()
			return
		}

		// update last
		revToBytes(reversion{main: rev.main, sub: rev.sub + 1}, last)
		tx.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}
