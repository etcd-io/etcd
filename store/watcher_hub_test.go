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

package store

import (
	"testing"
)

// TestIsHidden tests isHidden functions.
func TestIsHidden(t *testing.T) {
	// watch at "/"
	// key is "/_foo", hidden to "/"
	// expected: hidden = true
	watch := "/"
	key := "/_foo"
	hidden := isHidden(watch, key)
	if !hidden {
		t.Fatalf("%v should be hidden to %v\n", key, watch)
	}

	// watch at "/_foo"
	// key is "/_foo", not hidden to "/_foo"
	// expected: hidden = false
	watch = "/_foo"
	hidden = isHidden(watch, key)
	if hidden {
		t.Fatalf("%v should not be hidden to %v\n", key, watch)
	}

	// watch at "/_foo/"
	// key is "/_foo/foo", not hidden to "/_foo"
	key = "/_foo/foo"
	hidden = isHidden(watch, key)
	if hidden {
		t.Fatalf("%v should not be hidden to %v\n", key, watch)
	}

	// watch at "/_foo/"
	// key is "/_foo/_foo", hidden to "/_foo"
	key = "/_foo/_foo"
	hidden = isHidden(watch, key)
	if !hidden {
		t.Fatalf("%v should be hidden to %v\n", key, watch)
	}

	// watch at "/_foo/foo"
	// key is "/_foo"
	watch = "_foo/foo"
	key = "/_foo/"
	hidden = isHidden(watch, key)
	if hidden {
		t.Fatalf("%v should not be hidden to %v\n", key, watch)
	}
}

func TestWatchersCount(t *testing.T) {
	wh := newWatchHub(10)

	w, err := wh.watch("/foo", true, false, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if wh.count != 1 {
		t.Fatalf("watchers count was %d, expected %d", wh.count, 1)
	}

	wr, err := wh.watch("/foo", true, true, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if wh.count != 2 {
		t.Fatalf("watchers count was %d, expected %d", wh.count, 2)
	}
	wr2, err := wh.watch("/foo", true, true, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if wh.count != 3 {
		t.Fatalf("watchers count was %d, expected %d", wh.count, 3)
	}

	e := newEvent(Set, "/foo", 1, 1)
	wh.notify(e)

	// watcher hub's internal remove must reduce the count for the
	// non-streaming watcher
	if wh.count != 2 {
		t.Fatalf("watchers count was %d, expected %d", wh.count, 2)
	}
	// removing the non-streaming watcher manually must not reduce the
	// count again
	w.Remove()
	if wh.count != 2 {
		t.Fatalf("watchers count was %d, expected %d", wh.count, 2)
	}
	// removing first streaming watcher externally
	wr.Remove()
	if wh.count != 1 {
		t.Fatalf("watchers count was %d, expected %d", wh.count, 1)
	}
	// removing second streaming watcher by missing event
	c := wr2.EventChan()
	i := uint64(1)
	for int(i) <= cap(c)+1 {
		e = newEvent(Create, "/foo", i, i)
		wh.notify(e)
		i++
	}
	if wh.count != 0 {
		t.Fatalf("watchers count was %d, expected %d", wh.count, 0)
	}
}
