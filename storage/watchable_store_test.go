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
	cancel := w.Watch(testKey, true, 0)

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

	wa := &watching{
		key:    testKey,
		prefix: true,
		cur:    0,
	}

	if err := unsafeAddWatching(&s.synced, string(testKey), wa); err != nil {
		t.Error(err)
	}

	if v, ok := s.synced[string(testKey)]; !ok {
		// the key must have had entry in synced
		t.Errorf("existence = %v, want true", ok)
	} else {
		if len(v) != 1 {
			// the key must have ONE entry in its watching map
			t.Errorf("len(v) = %d, want 1", len(v))
		}
	}

	if err := unsafeAddWatching(&s.synced, string(testKey), wa); err == nil {
		// unsafeAddWatching should have returned error
		// when putting the same watch twice"
		t.Error(`error = nil, want "put the same watch twice"`)
	}
}
