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
