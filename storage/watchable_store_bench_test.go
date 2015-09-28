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
	"fmt"
	"testing"
)

func BenchmarkWatchableStoreUnsyncedCancel(b *testing.B) {
	s := newWatchableStore(tmpPath)
	defer cleanup(s, tmpPath)

	b.ReportAllocs()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// Put 1042 keys
		keys := [][]byte{}
		for j := 0; j < 1042; j++ {
			k := []byte(fmt.Sprint("foo", i))
			s.Put(k, []byte("bar"))
			keys = append(keys, k)
		}
		cancels := []func(){}
		// First populate unsynced watchers
		for _, k := range keys {
			_, cancel := s.Watcher(k, true, -1, 0)
			cancels = append(cancels, cancel)
		}
		// then cancel one by one, asynchronously
		done := make(chan struct{})
		for _, cancel := range cancels {
			go func(cancel func()) {
				cancel()
				done <- struct{}{}
			}(cancel)
		}
		cn := 0
		for range done {
			cn++
			if cn == len(cancels) {
				close(done)
			}
		}
	}
}

func BenchmarkWatchableStoreUnsyncedCancelSequential(b *testing.B) {
	s := newWatchableStore(tmpPath)
	defer cleanup(s, tmpPath)

	b.ReportAllocs()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// Put 1042 keys
		keys := [][]byte{}
		for j := 0; j < 1042; j++ {
			k := []byte(fmt.Sprint("foo", i))
			s.Put(k, []byte("bar"))
			keys = append(keys, k)
		}
		cancels := []func(){}
		// First populate unsynced watchers
		for _, k := range keys {
			_, cancel := s.Watcher(k, true, -1, 0)
			cancels = append(cancels, cancel)
		}
		// then cancel one by one, sequentially
		for _, cancel := range cancels {
			cancel()
		}
	}
}
