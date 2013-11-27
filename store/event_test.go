package store

import (
	"testing"
)

// TestEventQueue tests a queue with capacity = 100
// Add 200 events into that queue, and test if the
// previous 100 events have been swapped out.
func TestEventQueue(t *testing.T) {

	eh := newEventHistory(100)

	// Add
	for i := 0; i < 200; i++ {
		e := newEvent(Create, "/foo", uint64(i))
		eh.addEvent(e)
	}

	// Test
	j := 100
	i := eh.Queue.Front
	n := eh.Queue.Size
	for ; n > 0; n-- {
		e := eh.Queue.Events[i]
		if e.Index() != uint64(j) {
			t.Fatalf("queue error!")
		}
		j++
		i = (i + 1) % eh.Queue.Capacity
	}
}

func TestScanHistory(t *testing.T) {
	eh := newEventHistory(100)

	// Add
	eh.addEvent(newEvent(Create, "/foo", 1))
	eh.addEvent(newEvent(Create, "/foo/bar", 2))
	eh.addEvent(newEvent(Create, "/foo/foo", 3))
	eh.addEvent(newEvent(Create, "/foo/bar/bar", 4))
	eh.addEvent(newEvent(Create, "/foo/foo/foo", 5))

	events, err := eh.scan("/foo", 1)
	e := events[0]
	if err != nil || e.Index() != 1 {
		t.Fatalf("scan error [/foo] [1] %v", e.Index)
	}
	if len(events) != 5 {
		t.Fatalf("invalid history")
	}

	events, err = eh.scan("/foo/bar", 1)
	e = events[0]
	if err != nil || e.Index() != 2 {
		t.Fatalf("scan error [/foo/bar] [2] %v", e.Index)
	}
	if len(events) != 2 {
		t.Fatalf("invalid history")
	}

	events, err = eh.scan("/foo/bar", 3)
	e = events[0]
	if err != nil || e.Index() != 4 {
		t.Fatalf("scan error [/foo/bar/bar] [4] %v", e.Index)
	}
	if len(events) != 1 {
		t.Fatalf("invalid history")
	}

	events, err = eh.scan("/foo/bar", 6)
	if events != nil {
		t.Fatalf("bad index shoud reuturn nil")
	}
}
