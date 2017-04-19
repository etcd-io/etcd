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

func BenchmarkKVWatcherMemoryUsageWithUnsynced(b *testing.B) {
	s := newWatchableStore(tmpPath)
	defer cleanup(s, tmpPath)

	wa, cancel := s.Watcher([]byte("foo"), true, -1, 0)
	defer cancel()

	for i := 0; i < 3; i++ {
		s.Put([]byte("foo"), []byte("bar"))
	}
	select {
	case <-wa.Event():
	}
	s.Put([]byte("foo1"), []byte("bar1"))

	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		s.Watcher([]byte(fmt.Sprint("foo", i)), false, -1, 0)
	}
}
