package fileSystem

import (
	"testing"
)

func TestWatch(t *testing.T) {
	wh := newWatchHub(100)
	err, c := wh.watch("/foo", 0)

	if err != nil {
		t.Fatal("%v", err)
	}

	select {
	case <-c:
		t.Fatal("should not receive from channel before send the event")
	default:
		// do nothing
	}

	e := newEvent(Set, "/foo/bar", 1, 0)

	wh.notify(e)

	re := <-c

	if e != re {
		t.Fatal("recv != send")
	}
}
