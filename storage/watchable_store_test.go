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
	"os"
	"reflect"
	"testing"

	"github.com/coreos/etcd/storage/storagepb"
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

	w := s.NewWatchStream()
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

	w := s.NewWatchStream()
	_, cancel := w.Watch(testKey, true, 0)

	cancel()

	if _, ok := s.synced[string(testKey)]; ok {
		// the key shoud have been deleted
		t.Errorf("existence = %v, want false", ok)
	}
}

// TestCancelUnsynced tests if running CancelFunc removes watchers from unsynced.
func TestCancelUnsynced(t *testing.T) {
	// manually create watchableStore instead of newWatchableStore
	// because newWatchableStore automatically calls syncWatchers
	// method to sync watchers in unsynced map. We want to keep watchers
	// in unsynced to test if syncWatchers works as expected.
	s := &watchableStore{
		store:    newDefaultStore(tmpPath),
		unsynced: make(map[*watcher]struct{}),

		// to make the test not crash from assigning to nil map.
		// 'synced' doesn't get populated in this test.
		synced: make(map[string]map[*watcher]struct{}),
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

	w := s.NewWatchStream()

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
	// because cancel removes watcher from unsynced
	if len(s.unsynced) != 0 {
		t.Errorf("unsynced size = %d, want 0", len(s.unsynced))
	}
}

// TestSyncWatchers populates unsynced watcher map and tests syncWatchers
// method to see if it correctly sends events to channel of unsynced watchers
// and moves these watchers to synced.
func TestSyncWatchers(t *testing.T) {
	s := &watchableStore{
		store:    newDefaultStore(tmpPath),
		unsynced: make(map[*watcher]struct{}),
		synced:   make(map[string]map[*watcher]struct{}),
	}

	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue)

	w := s.NewWatchStream()

	// arbitrary number for watchers
	watcherN := 100

	for i := 0; i < watcherN; i++ {
		// use 1 to keep watchers in unsynced
		w.Watch(testKey, true, 1)
	}

	// Before running s.syncWatchers()
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

	// this should move all unsynced watchers
	// to synced ones
	s.syncWatchers()

	// After running s.syncWatchers()
	//
	// synced should not be empty
	// because syncwatchers populates synced
	// in this test case
	if len(s.synced[string(testKey)]) == 0 {
		t.Errorf("synced[string(testKey)] size = 0, want %d", len(s.synced[string(testKey)]))
	}
	// unsynced should be empty
	// because syncwatchers is expected to move
	// all watchers from unsynced to synced
	// in this test case
	if len(s.unsynced) != 0 {
		t.Errorf("unsynced size = %d, want 0", len(s.unsynced))
	}

	// All of the watchers actually share one channel
	// so we only need to check one shared channel
	// (See watcher.go for more detail).
	if len(w.(*watchStream).ch) != watcherN {
		t.Errorf("watched event size = %d, want %d", len(w.(*watchStream).ch), watcherN)
	}
	evs := <-w.(*watchStream).ch
	if len(evs) != 1 {
		t.Errorf("len(evs) got = %d, want = 1", len(evs))
	}
	if evs[0].Type != storagepb.PUT {
		t.Errorf("got = %v, want = %v", evs[0].Type, storagepb.PUT)
	}
	if !bytes.Equal(evs[0].Kv.Key, testKey) {
		t.Errorf("got = %s, want = %s", evs[0].Kv.Key, testKey)
	}
	if !bytes.Equal(evs[0].Kv.Value, testValue) {
		t.Errorf("got = %s, want = %s", evs[0].Kv.Value, testValue)
	}
}

func TestUnsafeAddWatcher(t *testing.T) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()
	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue)

	size := 10
	ws := make([]*watcher, size)
	for i := 0; i < size; i++ {
		ws[i] = &watcher{
			key:    testKey,
			prefix: true,
			cur:    0,
		}
	}
	// to test if unsafeAddWatcher is correctly updating
	// synced map when adding new watcher.
	for i, wa := range ws {
		if err := unsafeAddWatcher(&s.synced, string(testKey), wa); err != nil {
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

func TestNewMapwatcherToEventMap(t *testing.T) {
	k0, k1, k2 := []byte("foo0"), []byte("foo1"), []byte("foo2")
	v0, v1, v2 := []byte("bar0"), []byte("bar1"), []byte("bar2")

	ws := []*watcher{{key: k0}, {key: k1}, {key: k2}}

	evs := []storagepb.Event{
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: k0, Value: v0},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: k1, Value: v1},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: k2, Value: v2},
		},
	}

	tests := []struct {
		sync map[string]map[*watcher]struct{}
		evs  []storagepb.Event

		wwe map[*watcher][]storagepb.Event
	}{
		// no watcher in sync, some events should return empty wwe
		{
			map[string]map[*watcher]struct{}{},
			evs,
			map[*watcher][]storagepb.Event{},
		},

		// one watcher in sync, one event that does not match the key of that
		// watcher should return empty wwe
		{
			map[string]map[*watcher]struct{}{
				string(k2): {ws[2]: struct{}{}},
			},
			evs[:1],
			map[*watcher][]storagepb.Event{},
		},

		// one watcher in sync, one event that matches the key of that
		// watcher should return wwe with that matching watcher
		{
			map[string]map[*watcher]struct{}{
				string(k1): {ws[1]: struct{}{}},
			},
			evs[1:2],
			map[*watcher][]storagepb.Event{
				ws[1]: evs[1:2],
			},
		},

		// two watchers in sync that watches two different keys, one event
		// that matches the key of only one of the watcher should return wwe
		// with the matching watcher
		{
			map[string]map[*watcher]struct{}{
				string(k0): {ws[0]: struct{}{}},
				string(k2): {ws[2]: struct{}{}},
			},
			evs[2:],
			map[*watcher][]storagepb.Event{
				ws[2]: evs[2:],
			},
		},

		// two watchers in sync that watches the same key, two events that
		// match the keys should return wwe with those two watchers
		{
			map[string]map[*watcher]struct{}{
				string(k0): {ws[0]: struct{}{}},
				string(k1): {ws[1]: struct{}{}},
			},
			evs[:2],
			map[*watcher][]storagepb.Event{
				ws[0]: evs[:1],
				ws[1]: evs[1:2],
			},
		},
	}

	for i, tt := range tests {
		gwe := newWatcherToEventMap(tt.sync, tt.evs)
		if len(gwe) != len(tt.wwe) {
			t.Errorf("#%d: len(gwe) got = %d, want = %d", i, len(gwe), len(tt.wwe))
		}
		// compare gwe and tt.wwe
		for w, mevs := range gwe {
			if len(mevs) != len(tt.wwe[w]) {
				t.Errorf("#%d: len(mevs) got = %d, want = %d", i, len(mevs), len(tt.wwe[w]))
			}
			if !reflect.DeepEqual(mevs, tt.wwe[w]) {
				t.Errorf("#%d: reflect.DeepEqual events got = %v, want = true", i, false)
			}
		}
	}
}
