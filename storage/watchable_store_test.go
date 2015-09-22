package storage

import (
	"fmt"
	"testing"
)

func TestingWatchableStore(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer cleanup(s, tmpPath)

	for i := 0; i < 3; i++ {
		s.Put([]byte("foo"), []byte("bar"))
		if r := s.Rev(); r != int64(i+1) {
			t.Errorf("#%d: rev = %d, want %d", i, r, i+1)
		}
		s.Watcher([]byte(fmt.Sprint("foo", i)), false, 0, 0)
	}
}
