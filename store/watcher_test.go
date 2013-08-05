package store

import (
	"testing"
	"time"
)

func TestWatch(t *testing.T) {

	s := CreateStore(100)

	watchers := make([]*Watcher, 10)

	for i, _ := range watchers {

		// create a new watcher
		watchers[i] = NewWatcher()
		// add to the watchers list
		s.AddWatcher("foo", watchers[i], 0)

	}

	s.Set("/foo/foo", "bar", time.Unix(0, 0), 1)

	for _, watcher := range watchers {

		// wait for the notification for any changing
		res := <-watcher.C

		if res == nil {
			t.Fatal("watcher is cleared")
		}
	}

	for i, _ := range watchers {

		// create a new watcher
		watchers[i] = NewWatcher()
		// add to the watchers list
		s.AddWatcher("foo/foo/foo", watchers[i], 0)

	}

	s.watcher.stopWatchers()

	for _, watcher := range watchers {

		// wait for the notification for any changing
		res := <-watcher.C

		if res != nil {
			t.Fatal("watcher is cleared")
		}
	}
}

// BenchmarkWatch creates 10K watchers watch at /foo/[paht] each time.
// Path is randomly chosen with max depth 10.
// It should take less than 15ms to wake up 10K watchers.
func BenchmarkWatch(b *testing.B) {
	s := CreateStore(100)

	keys := GenKeys(10000, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		watchers := make([]*Watcher, 10000)
		for i := 0; i < 10000; i++ {
			// create a new watcher
			watchers[i] = NewWatcher()
			// add to the watchers list
			s.AddWatcher(keys[i], watchers[i], 0)
		}

		s.watcher.stopWatchers()

		for _, watcher := range watchers {
			// wait for the notification for any changing
			<-watcher.C
		}

		s.watcher = newWatcherHub()
	}
}
