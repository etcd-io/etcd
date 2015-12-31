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
	"fmt"
	"os"
	"testing"
)

var (
	testN      = 100
	testKey    = []byte("Foo")
	testValue  = []byte("Bar")
	testKeys   = make([][]byte, testN)
	testValues = make([][]byte, testN)
)

func init() {
	for i := 0; i < testN; i++ {
		testKeys[i] = []byte(fmt.Sprintf("%s_%d", testKey, i))
		testValues[i] = []byte(fmt.Sprintf("%s_%d", testValue, i))
	}
}

func TestWatchableStoreWatch(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	var (
		prefix          = false
		startRev        = int64(0) // to populate sync
		watcherToKey    = make(map[Watcher][]byte)
		watcherToValue  = make(map[Watcher][]byte)
		watcherToCancel = make(map[Watcher]CancelFunc)
	)
	for i := 0; i < testN; i++ {
		// wa, c := s.watch(testKeys[i], prefix, startRev, id, ch)
		w := s.NewWatcher()
		_, c := w.Watch(testKeys[i], prefix, startRev)
		watcherToKey[w] = testKeys[i]
		watcherToValue[w] = testValues[i]
		watcherToCancel[w] = c
	}
	if len(s.synced) != testN {
		t.Errorf("len(s.synced) got = %d, want = %d", len(s.synced), testN)
	}
	if len(s.unsynced) != 0 {
		t.Errorf("len(s.unsynced) got = %d, want = 0", len(s.unsynced))
	}

	// now start sending events
	for i := 0; i < testN; i++ {
		s.Put(testKeys[i], testValues[i])
	}

	// start receiving from eventsCh
	for w, cancel := range watcherToCancel {
		select {
		case evs := <-w.Chan():
			if !bytes.Equal(evs[0].Kv.Key, watcherToKey[w]) {
				t.Errorf("evs[0].Kv.Key got = %s, want = %s", evs[0].Kv.Key, watcherToKey[w])
			}
			if !bytes.Equal(evs[0].Kv.Value, watcherToValue[w]) {
				t.Errorf("evs[0].Kv.Value got = %s, want = %s", evs[0].Kv.Value, watcherToValue[w])
			}
		default:
			t.Errorf("failed to receive from w.Chan() at %s", watcherToKey[w])
		}

		cancel()

		if _, ok := s.synced[string(watcherToKey[w])]; ok {
			t.Errorf("s.synced[k] for %s: ok = true, want = false", string(watcherToKey[w]))
		}
	}

	if len(s.synced) != 0 {
		t.Errorf("len(s.synced) got = %d, want = %d", len(s.synced), 0)
	}
}

func TestWatchableStoreWatchPrefix(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	var (
		prefix       = true
		startRev     = int64(0) // to populate sync
		watchedKey   = testKey
		watchedValue = testValue
	)
	// this will watch all events that has prefix watchedKey
	w := s.NewWatcher()
	_, c := w.Watch(watchedKey, prefix, startRev)

	if len(s.synced) != 1 {
		t.Errorf("len(s.synced) got = %d, want = %d", len(s.synced), 1)
	}
	if len(s.unsynced) != 0 {
		t.Errorf("len(s.unsynced) got = %d, want = 0", len(s.unsynced))
	}

	s.Put(watchedKey, watchedValue)
	for i := 0; i < testN; i++ {
		s.Put(testKeys[i], testValues[i])
	}

	for i := -1; i < testN; i++ {
		select {
		case evs := <-w.Chan():
			if i == -1 {
				if !bytes.Equal(evs[0].Kv.Key, watchedKey) {
					t.Errorf("evs[0].Kv.Key got = %s, want = %s", evs[0].Kv.Key, watchedKey)
				}
				if !bytes.Equal(evs[0].Kv.Value, watchedValue) {
					t.Errorf("evs[0].Kv.Value got = %s, want = %s", evs[0].Kv.Value, watchedValue)
				}
			} else {
				if !bytes.Equal(evs[0].Kv.Key, testKeys[i]) {
					t.Errorf("evs[0].Kv.Key got = %s, want = %s", evs[0].Kv.Key, testKeys[i])
				}
				if !bytes.Equal(evs[0].Kv.Value, testValues[i]) {
					t.Errorf("evs[0].Kv.Value got = %s, want = %s", evs[0].Kv.Value, testValues[i])
				}
			}
		default:
			t.Errorf("#%d: failed to receive from w.Chan()", i)
		}
	}

	c()

	if _, ok := s.synced[string(testKey)]; ok {
		t.Errorf("s.synced[k] for %s: ok = true, want = false", testKey)
	}
}

func TestWatchableStoreWatchTxn(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	var (
		prefix          = false
		startRev        = int64(0) // to populate sync
		watcherToKey    = make(map[Watcher][]byte)
		watcherToValue  = make(map[Watcher][]byte)
		watcherToCancel = make(map[Watcher]CancelFunc)
	)
	for i := 0; i < testN; i++ {
		w := s.NewWatcher()
		_, c := w.Watch(testKeys[i], prefix, startRev)
		watcherToKey[w] = testKeys[i]
		watcherToValue[w] = testValues[i]
		watcherToCancel[w] = c
	}
	if len(s.synced) != testN {
		t.Errorf("len(s.synced) got = %d, want = %d", len(s.synced), testN)
	}
	if len(s.unsynced) != 0 {
		t.Errorf("len(s.unsynced) got = %d, want = 0", len(s.unsynced))
	}

	tid := s.TxnBegin()
	for i := 0; i < testN; i++ {
		if _, err := s.TxnPut(tid, testKeys[i], testValue); err != nil {
			t.Error(err)
		}
		if _, err := s.TxnPut(tid, testKeys[i], testValues[i]); err != nil {
			t.Error(err)
		}
	}
	if err := s.TxnEnd(tid); err != nil {
		t.Error(err)
	}

	// start receiving from eventsCh
	for w, cancel := range watcherToCancel {
		select {
		case evs := <-w.Chan():
			if len(evs) == 0 {
				continue
			}
			if !bytes.Equal(evs[0].Kv.Key, watcherToKey[w]) {
				t.Errorf("evs[0].Kv.Key got = %s, want = %s", evs[0].Kv.Key, watcherToKey[w])
			}
			if !bytes.Equal(evs[0].Kv.Value, watcherToValue[w]) {
				t.Errorf("evs[0].Kv.Value got = %s, want = %s", evs[0].Kv.Value, watcherToValue[w])
			}
		default:
			t.Errorf("failed to receive from w.Chan() at %s", watcherToKey[w])
		}

		cancel()

		if _, ok := s.synced[string(watcherToKey[w])]; ok {
			t.Errorf("s.synced[k] for %s: ok = true, want = false", string(watcherToKey[w]))
		}
	}

	if len(s.synced) != 0 {
		t.Errorf("len(s.synced) got = %d, want = %d", len(s.synced), 0)
	}
}

// TestCancelUnsynced tests if running CancelFunc removes watchings from unsynced.
func TestCancelUnsynced(t *testing.T) {
	// manually create watchableStore instead of newWatchableStore
	// because newWatchableStore automatically calls syncWatchers
	// method to sync watchers in unsynced map. We want to keep watchers
	// in unsynced to test if syncWatchers works as expected.
	s := &watchableStore{
		store:    newStore(tmpPath),
		unsynced: make(map[*watching]struct{}),

		// to make the test not crash from assigning to nil map.
		// 'synced' doesn't get populated in this test.
		synced: make(map[string]map[*watching]struct{}),
	}

	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	// Put a key so that we can spawn watchers on that key.
	// (testKey in this test). This increases the rev to 1,
	// and later we can we set the watcher's startRev to 1,
	// and force watchers to be in unsynced.
	s.Put(testKey, testValue)

	w := s.NewWatcher()

	// arbitrary number for watchers
	watcherN := 100

	// create watcherN of CancelFunc of
	// synced and unsynced
	cancels := make([]CancelFunc, watcherN)
	for i := 0; i < watcherN; i++ {
		// use 1 to keep watchers in unsynced
		_, cancel := w.Watch(testKey, true, 1)
		cancels[i] = cancel
	}

	for idx := range cancels {
		cancels[idx]()
	}

	// After running CancelFunc
	//
	// unsynced should be empty
	// because cancel removes watching from unsynced
	if len(s.unsynced) != 0 {
		t.Errorf("unsynced size = %d, want 0", len(s.unsynced))
	}
}

// TestSyncWatchings populates unsynced watching map and
// tests syncWatchings method to see if it correctly sends
// events to channel of unsynced watchings and moves these
// watchings to synced.
func TestSyncWatchings(t *testing.T) {
	s := &watchableStore{
		store:    newStore(tmpPath),
		unsynced: make(map[*watching]struct{}),
		synced:   make(map[string]map[*watching]struct{}),
	}

	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	s.Put(testKey, testValue)

	w := s.NewWatcher()

	// arbitrary number for watchers
	watcherN := 100

	for i := 0; i < watcherN; i++ {
		// use 1 to keep watchers in unsynced
		w.Watch(testKey, true, 1)
	}

	// Before running s.syncWatchings()
	//
	// synced should be empty
	// because we manually populate unsynced only
	if len(s.synced[string(testKey)]) != 0 {
		t.Fatalf("synced[string(testKey)] size = %d, want 0", len(s.synced[string(testKey)]))
	}
	// unsynced should not be empty
	// because we manually populated unsynced only
	if len(s.unsynced) == 0 {
		t.Errorf("unsynced size = %d, want %d", len(s.unsynced), watcherN)
	}

	// this should move all unsynced watchings
	// to synced ones
	s.syncWatchings()

	// After running s.syncWatchings()
	//
	// synced should not be empty
	// because syncWatchings populates synced
	// in this test case
	if len(s.synced[string(testKey)]) == 0 {
		t.Errorf("synced[string(testKey)] size = 0, want %d", len(s.synced[string(testKey)]))
	}
	// unsynced should be empty
	// because syncWatchings is expected to move
	// all watchings from unsynced to synced
	// in this test case
	if len(s.unsynced) != 0 {
		t.Errorf("unsynced size = %d, want 0", len(s.unsynced))
	}

	// All of the watchings actually share one channel
	// so we only need to check one shared channel
	// (See watcher.go for more detail).
	// evs := <-w.(*watcher).ch
	// if evs[0].Type != storagepb.PUT {
	// 	t.Errorf("got = %v, want = %v", evs[0].Type, storagepb.PUT)
	// }
	// if !bytes.Equal(evs[0].Kv.Key, testKey) {
	// 	t.Errorf("got = %s, want = %s", evs[0].Kv.Key, testKey)
	// }
	// if !bytes.Equal(evs[0].Kv.Value, testValue) {
	// 	t.Errorf("got = %s, want = %s", evs[0].Kv.Value, testValue)
	// }
}

func TestUnsafeAddWatching(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	s.Put(testKey, testValue)

	size := 10
	ws := make([]*watching, size)
	for i := 0; i < size; i++ {
		ws[i] = &watching{
			key:    testKey,
			prefix: true,
			cur:    0,
		}
	}
	// to test if unsafeAddWatching is correctly updating
	// synced map when adding new watching.
	for i, wa := range ws {
		if err := unsafeAddWatching(&s.synced, string(testKey), wa); err != nil {
			t.Errorf("#%d: error = %v, want nil", i, err)
		}
		if v, ok := s.synced[string(testKey)]; !ok {
			t.Errorf("#%d: ok = %v, want ok true", i, ok)
		} else {
			if len(v) != i+1 {
				t.Errorf("#%d: len(v) = %d, want %d", i, len(v), i+1)
			}
			if _, ok := v[wa]; !ok {
				t.Errorf("#%d: ok = %v, want ok true", i, ok)
			}
		}
	}
}
