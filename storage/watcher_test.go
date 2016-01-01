// Copyright 2015 CoreOS, Inc.
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

package storage

import "testing"

// TestWatcherWatchID tests that each watcher provides unique watch ID,
// and the watched event attaches the correct watch ID.
func TestWatcherWatchID(t *testing.T) {
	s := WatchableKV(newWatchableStore(tmpPath))
	defer cleanup(s, tmpPath)

	w := s.NewWatcher()
	defer w.Close()

	idm := make(map[int64]struct{})

	for i := 0; i < 10; i++ {
		id, cancel := w.Watch([]byte("foo"), false, 0)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		s.Put([]byte("foo"), []byte("bar"))

		evs := <-w.Chan()
		for j, ev := range evs {
			if ev.WatchID != id {
				t.Errorf("#%d.%d: watch id in event = %d, want %d", i, j, ev.WatchID, id)
			}
		}
		cancel()
	}

	s.Put([]byte("foo2"), []byte("bar"))

	// unsynced watchings
	for i := 10; i < 20; i++ {
		id, cancel := w.Watch([]byte("foo2"), false, 1)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		evs := <-w.Chan()
		for j, ev := range evs {
			if ev.WatchID != id {
				t.Errorf("#%d.%d: watch id in event = %d, want %d", i, j, ev.WatchID, id)
			}
		}

		cancel()
	}
}
