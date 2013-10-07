/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

// BenchmarkWatch creates 10K watchers watch at /foo/[path] each time.
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
