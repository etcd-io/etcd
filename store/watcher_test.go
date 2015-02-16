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

func TestWatcher(t *testing.T) {
	s := newStore()
	wh := s.WatcherHub
	w, err := wh.watch("/foo", true, false, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	c := w.EventChan()

	select {
	case <-c:
		t.Fatal("should not receive from channel before send the event")
	default:
		// do nothing
	}

	e := newEvent(Create, "/foo/bar", 1, 1)
	wh.notify(e)

	re := <-c
	if e != re {
		t.Fatal("recv != send")
	}

	w, err = wh.watch("/foo", false, false, 3, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	c = w.EventChan()

	e = newEvent(Create, "/foo/bar", 2, 2)
	wh.notify(e)

	select {
	case re = <-c:
		t.Fatal("should not receive from channel if not recursive ", re)
	default:
		// do nothing
	}

	e = newEvent(Create, "/foo", 3, 3)
	wh.notify(e)

	re = <-c
	if e != re {
		t.Fatal("recv != send")
	}

	// should not receive another event if not streaming
	e = newEvent(Create, "/foo", 4, 4)
	wh.notify(e)

	select {
	case _, ok := <-c:
		if ok {
			t.Fatal("should not receive more than once from non-streaming watcher ", re)
		}
	default:
	}

	// test streaming watcher
	w, err = wh.watch("/bar", true, true, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	c = w.EventChan()

	for i := uint64(1); int(i) < cap(c); i++ {
		e = newEvent(Create, "/bar", i, i)
		wh.notify(e)

		re, ok := <-c
		if !ok {
			t.Fatal("streaming channel should receive")
		}
		if re != e {
			t.Fatal("recv != send")
		}
	}

	// ensure we are doing exact matching rather than prefix matching
	w, err = wh.watch("/fo", true, false, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	c = w.EventChan()

	select {
	case re = <-c:
		t.Fatal("should not receive from channel:", re)
	default:
		// do nothing
	}

	e = newEvent(Create, "/fo/bar", 3, 3)
	wh.notify(e)

	re = <-c
	if e != re {
		t.Fatal("recv != send")
	}
}

func TestWatcherRemoval(t *testing.T) {
	s := newStore()
	wh := s.WatcherHub

	// ensure the watcher receives no further events after removing itself
	w, err := wh.watch("/foo", false, false, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	c := w.EventChan()

	w.Remove()

	e := newEvent(Create, "/foo/bar", 1, 1)
	wh.notify(e)

	select {
	case re, ok := <-c:
		if ok {
			t.Fatal("should not receive from channel after removing our watch ", re)
		}
	default:
		t.Fatal("event channel blocking, should be closed")
	}

	// removal on missed notifcation
	w, err = wh.watch("/spam", true, true, 1, 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	c = w.EventChan()

	i := uint64(1)
	for int(i) <= cap(c)+1 {
		e = newEvent(Create, "/spam", i, i)
		wh.notify(e)
		i++
	}
	for range c {
		// drain must exit
	}
}
