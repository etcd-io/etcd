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

import (
	"bytes"
	"testing"
)

// TestWatcherWatchID tests that each watcher provides unique watch ID,
// and the watched event attaches the correct watch ID.
func TestWatcherWatchID(t *testing.T) {
	s := WatchableKV(newWatchableStore(tmpPath))
	defer cleanup(s, tmpPath)

	w := s.NewWatcher()
	defer w.Close()

	idm := make(map[int64]struct{})

	// synced watchings
	for i := 0; i < testN/2; i++ {
		id, cancel := w.Watch(testKey, false, 0)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		s.Put(testKey, testValue)

		evs := <-w.Chan()
		for j, ev := range evs {
			if !bytes.Equal(ev.Kv.Key, testKey) {
				t.Errorf("#%d: Key got = %s, want = %s", j, ev.Kv.Key, testKey)
			}
			if !bytes.Equal(ev.Kv.Value, testValue) {
				t.Errorf("#%d: Value got = %s, want = %s", j, ev.Kv.Value, testValue)
			}
			if ev.WatchID != id {
				t.Errorf("#%d: watch id in event = %d, want %d", j, ev.WatchID, id)
			}
		}

		cancel()
	}

	s.Put(testKeys[0], testValues[0])

	// unsynced watchings
	for i := testN / 2; i < testN; i++ {
		id, cancel := w.Watch(testKeys[0], false, 1)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		evs := <-w.Chan()
		for j, ev := range evs {
			if !bytes.Equal(ev.Kv.Key, testKeys[0]) {
				t.Errorf("#%d: Key got = %s, want = %s", j, ev.Kv.Key, testKeys[0])
			}
			if !bytes.Equal(ev.Kv.Value, testValues[0]) {
				t.Errorf("#%d: Value got = %s, want = %s", j, ev.Kv.Value, testValues[0])
			}
			if ev.WatchID != id {
				t.Errorf("#%d: watch id in event = %d, want %d", j, ev.WatchID, id)
			}
		}

		cancel()
	}
}
