package storage

import (
	"reflect"
	"testing"
)

func TestScheduleCompaction(t *testing.T) {
	revs := []revision{{1, 0}, {2, 0}, {3, 0}}

	tests := []struct {
		rev   int64
		keep  map[revision]struct{}
		wrevs []revision
	}{
		// compact at 1 and discard all history
		{
			1,
			nil,
			revs[1:],
		},
		// compact at 3 and discard all history
		{
			3,
			nil,
			nil,
		},
		// compact at 1 and keeps history one step earlier
		{
			1,
			map[revision]struct{}{
				revision{main: 1}: {},
			},
			revs,
		},
		// compact at 1 and keeps history two steps earlier
		{
			3,
			map[revision]struct{}{
				revision{main: 2}: {},
				revision{main: 3}: {},
			},
			revs[1:],
		},
	}
	for i, tt := range tests {
		s := newStore(tmpPath)
		tx := s.b.BatchTx()

		tx.Lock()
		ibytes := newRevBytes()
		for _, rev := range revs {
			revToBytes(rev, ibytes)
			tx.UnsafePut(keyBucketName, ibytes, []byte("bar"))
		}
		tx.Unlock()
		// call `s.wg.Add(1)` to match the `s.wg.Done()` call in scheduleCompaction
		// to avoid panic from wait group
		s.wg.Add(1)
		s.scheduleCompaction(tt.rev, tt.keep)

		tx.Lock()
		for _, rev := range tt.wrevs {
			revToBytes(rev, ibytes)
			keys, _ := tx.UnsafeRange(keyBucketName, ibytes, nil, 0)
			if len(keys) != 1 {
				t.Errorf("#%d: range on %v = %d, want 1", i, rev, len(keys))
			}
		}
		_, vals := tx.UnsafeRange(metaBucketName, finishedCompactKeyName, nil, 0)
		revToBytes(revision{main: tt.rev}, ibytes)
		if w := [][]byte{ibytes}; !reflect.DeepEqual(vals, w) {
			t.Errorf("#%d: vals on %v = %+v, want %+v", i, finishedCompactKeyName, vals, w)
		}
		tx.Unlock()

		cleanup(s, tmpPath)
	}
}
