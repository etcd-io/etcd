package store

import (
	"testing"
)

func TestWatcher(t *testing.T) {
	s := New()
	wh := s.WatcherHub
	c, err := wh.watch("/foo", true, 1)
	if err != nil {
		t.Fatal("%v", err)
	}

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

	c, _ = wh.watch("/foo", false, 2)

	e = newEvent(Create, "/foo/bar", 2, 1)

	wh.notify(e)

	select {
	case re = <-c:
		t.Fatal("should not receive from channel if not recursive ", re)
	default:
		// do nothing
	}

	e = newEvent(Create, "/foo", 3, 1)

	wh.notify(e)

	re = <-c

	if e != re {
		t.Fatal("recv != send")
	}

}
