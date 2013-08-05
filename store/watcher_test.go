package store

import (
	"fmt"
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
