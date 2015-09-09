package storage

import (
	"fmt"
	"testing"
)

func BenchmarkKVWatcherMemoryUsage(b *testing.B) {
	s := newWatchableStore(tmpPath)
	defer cleanup(s, tmpPath)

	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		s.Watcher([]byte(fmt.Sprint("foo", i)), false, 0, 0)
	}
}
