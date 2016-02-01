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

	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/storage/backend"
)

// TestWatcherWatchID tests that each watcher provides unique watchID,
// and the watched event attaches the correct watchID.
func TestWatcherWatchID(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := WatchableKV(newWatchableStore(b, &lease.FakeLessor{}))
	defer cleanup(s, b, tmpPath)

	w := s.NewWatchStream()
	defer w.Close()

	idm := make(map[WatchID]struct{})

	for i := 0; i < 10; i++ {
		id := w.Watch([]byte("foo"), false, 0)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		s.Put([]byte("foo"), []byte("bar"), lease.NoLease)

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Error(err)
		}
	}

	s.Put([]byte("foo2"), []byte("bar"), lease.NoLease)

	// unsynced watchers
	for i := 10; i < 20; i++ {
		id := w.Watch([]byte("foo2"), false, 1)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Error(err)
		}
	}
}

// TestWatcherWatchPrefix tests if Watch operation correctly watches
// and returns events with matching prefixes.
func TestWatcherWatchPrefix(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := WatchableKV(newWatchableStore(b, &lease.FakeLessor{}))
	defer cleanup(s, b, tmpPath)

	w := s.NewWatchStream()
	defer w.Close()

	idm := make(map[WatchID]struct{})

	prefixMatch := true
	val := []byte("bar")
	keyWatch, keyPut := []byte("foo"), []byte("foobar")

	for i := 0; i < 10; i++ {
		id := w.Watch(keyWatch, prefixMatch, 0)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: unexpected duplicated id %x", i, id)
		}
		idm[id] = struct{}{}

		s.Put(keyPut, val, lease.NoLease)

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Errorf("#%d: unexpected cancel error %v", i, err)
		}

		if len(resp.Events) != 1 {
			t.Errorf("#%d: len(resp.Events) got = %d, want = 1", i, len(resp.Events))
		}
		if len(resp.Events) == 1 {
			if !bytes.Equal(resp.Events[0].Kv.Key, keyPut) {
				t.Errorf("#%d: resp.Events got = %s, want = %s", i, resp.Events[0].Kv.Key, keyPut)
			}
		}
	}

	keyWatch1, keyPut1 := []byte("foo1"), []byte("foo1bar")
	s.Put(keyPut1, val, lease.NoLease)

	// unsynced watchers
	for i := 10; i < 15; i++ {
		id := w.Watch(keyWatch1, prefixMatch, 1)
		if _, ok := idm[id]; ok {
			t.Errorf("#%d: id %d exists", i, id)
		}
		idm[id] = struct{}{}

		resp := <-w.Chan()
		if resp.WatchID != id {
			t.Errorf("#%d: watch id in event = %d, want %d", i, resp.WatchID, id)
		}

		if err := w.Cancel(id); err != nil {
			t.Error(err)
		}

		if len(resp.Events) != 1 {
			t.Errorf("#%d: len(resp.Events) got = %d, want = 1", i, len(resp.Events))
		}
		if len(resp.Events) == 1 {
			if !bytes.Equal(resp.Events[0].Kv.Key, keyPut1) {
				t.Errorf("#%d: resp.Events got = %s, want = %s", i, resp.Events[0].Kv.Key, keyPut1)
			}
		}
	}
}

// TestWatchStreamCancelWatcherByID ensures cancel calls the cancel func of the watcher
// with given id inside watchStream.
func TestWatchStreamCancelWatcherByID(t *testing.T) {
	b, tmpPath := backend.NewDefaultTmpBackend()
	s := WatchableKV(newWatchableStore(b, &lease.FakeLessor{}))
	defer cleanup(s, b, tmpPath)

	w := s.NewWatchStream()
	defer w.Close()

	id := w.Watch([]byte("foo"), false, 0)

	tests := []struct {
		cancelID WatchID
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
