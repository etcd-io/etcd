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
	"os"
	"testing"
)

func TestWatch(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()
	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue)

	w := s.NewWatcher()
	w.Watch(testKey, true, 0)

	if _, ok := s.synced[string(testKey)]; !ok {
		// the key must have had an entry in synced
		t.Errorf("existence = %v, want true", ok)
	}
}

func TestNewWatcherCancel(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()
	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue)

	w := s.NewWatcher()
	_, cancel := w.Watch(testKey, true, 0)

	cancel()

	if _, ok := s.synced[string(testKey)]; ok {
		// the key shoud have been deleted
		t.Errorf("existence = %v, want false", ok)
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
	testKey := []byte("foo")
	testValue := []byte("bar")
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

	testKey := []byte("foo")
	testValue := []byte("bar")
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
	if len(w.(*watcher).ch) != watcherN {
		t.Errorf("watched event size = %d, want %d", len(w.(*watcher).ch), watcherN)
	}
}

func TestUnsafeAddWatching(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()
	testKey := []byte("foo")
	testValue := []byte("bar")
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
