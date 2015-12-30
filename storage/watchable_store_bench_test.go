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
	"math/rand"
	"os"
	"testing"
)

// Benchmarks on cancel function performance for unsynced watchers
// in a WatchableStore. It creates k*N watchers to populate unsynced
// with a reasonably large number of watchers. And measures the time it
// takes to cancel N watchers out of k*N watchers. The performance is
// expected to differ depending on the unsynced member implementation.
// TODO: k is an arbitrary constant. We need to figure out what factor
// we should put to simulate the real-world use cases.
func BenchmarkWatchableStoreUnsyncedCancel(b *testing.B) {
	// manually create watchableStore instead of newWatchableStore
	// because newWatchableStore periodically calls syncWatchersLoop
	// method to sync watchers in unsynced map. We want to keep watchers
	// in unsynced for this benchmark.
	s := &watchableStore{
		store:    newDefaultStore(tmpPath),
		unsynced: make(map[*watching]struct{}),

		// to make the test not crash from assigning to nil map.
		// 'synced' doesn't get populated in this test.
		synced: make(map[string]map[*watching]struct{}),
	}

	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	// Put a key so that we can spawn watchers on that key
	// (testKey in this test). This increases the rev to 1,
	// and later we can we set the watcher's startRev to 1,
	// and force watchers to be in unsynced.
	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue)

	w := s.NewWatcher()

	const k int = 2
	benchSampleN := b.N
	watcherN := k * benchSampleN

	cancels := make([]CancelFunc, watcherN)
	for i := 0; i < watcherN; i++ {
		// non-0 value to keep watchers in unsynced
		_, cancel := w.Watch(testKey, true, 1)
		cancels[i] = cancel
	}

	// random-cancel N watchers to make it not biased towards
	// data structures with an order, such as slice.
	ix := rand.Perm(watcherN)

	b.ResetTimer()
	b.ReportAllocs()

	// cancel N watchers
	for _, idx := range ix[:benchSampleN] {
		cancels[idx]()
	}
}

func BenchmarkWatchableStoreSyncedCancel(b *testing.B) {
	s := newWatchableStore(tmpPath)
	defer func() {
		s.store.Close()
		os.Remove(tmpPath)
	}()

	// Put a key so that we can spawn watchers on that key
	testKey := []byte("foo")
	testValue := []byte("bar")
	s.Put(testKey, testValue)

	w := s.NewWatcher()

	// put 1 million watchers on the same key
	const watcherN = 1000000

	cancels := make([]CancelFunc, watcherN)
	for i := 0; i < watcherN; i++ {
		// 0 for startRev to keep watchers in synced
		_, cancel := w.Watch(testKey, true, 0)
		cancels[i] = cancel
	}

	// randomly cancel watchers to make it not biased towards
	// data structures with an order, such as slice.
	ix := rand.Perm(watcherN)

	b.ResetTimer()
	b.ReportAllocs()

	for _, idx := range ix {
		cancels[idx]()
	}
}
