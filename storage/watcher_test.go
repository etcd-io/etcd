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

// TestWatcherWatchID tests that each watcher provides unique watchID,
// and the watched event attaches the correct watchID.
func TestWatcherWatchID(t *testing.T) {
	s := WatchableKV(newWatchableStore(tmpPath))
	defer cleanup(s, tmpPath)

	w := s.NewWatchStream()
	defer w.Close()

	idm := make(map[int64]struct{})

	for i := 0; i < 10; i++ {
		id, cancel := w.Watch([]byte("foo"), false, 0)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		s.Put([]byte("foo"), []byte("bar"))

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		cancel()
	}

	s.Put([]byte("foo2"), []byte("bar"))

	// unsynced watchers
	for i := 10; i < 20; i++ {
		id, cancel := w.Watch([]byte("foo2"), false, 1)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		cancel()
	}
}

// TestWatchStreamCancel ensures cancel calls the cancel func of the watcher
// with given id inside watchStream.
func TestWatchStreamCancelWatcherByID(t *testing.T) {
	s := WatchableKV(newWatchableStore(tmpPath))
	defer cleanup(s, tmpPath)

	w := s.NewWatchStream()
	defer w.Close()

	id, _ := w.Watch([]byte("foo"), false, 0)

	tests := []struct {
		cancelID int64
		werr     error
	}{
		// no error should be returned when cancel the created watcher.
		{id, nil},
		// not exist error should be returned when cancel again.
		{id, ErrWatcherNotExist},
		// not exist error should be returned when cancel a bad id.
		{id + 1, ErrWatcherNotExist},
	}

	for i, tt := range tests {
		gerr := w.Cancel(tt.cancelID)

		if gerr != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, gerr, tt.werr)
		}
	}

	if l := len(w.(*watchStream).cancels); l != 0 {
		t.Errorf("cancels = %d, want 0", l)
	}
}
