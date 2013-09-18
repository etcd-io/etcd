package fileSystem

import (
	"testing"
)

func TestWatcher(t *testing.T) {
	wh := newWatchHub(100)
	c, err := wh.watch("/foo", true, 0)

	if err != nil {
		t.Fatal("%v", err)
	}

	select {
	case <-c:
		t.Fatal("should not receive from channel before send the event")
	default:
		// do nothing
	}

	e := newEvent(Create, "/foo/bar", 1, 0)

	wh.notify(e)

	re := <-c

	if e != re {
		t.Fatal("recv != send")
	}

	c, _ = wh.watch("/foo", false, 0)

	e = newEvent(Create, "/foo/bar", 1, 0)

	wh.notify(e)

	select {
	case <-c:
		t.Fatal("should not receive from channel if not recursive")
	default:
		// do nothing
	}

	e = newEvent(Create, "/foo", 1, 0)

	wh.notify(e)

	re = <-c

	if e != re {
		t.Fatal("recv != send")
	}

}
