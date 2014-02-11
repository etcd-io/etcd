package raft

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Ensure that we can listen and dispatch events.
func TestDispatchEvent(t *testing.T) {
	var count int
	dispatcher := newEventDispatcher(nil)
	dispatcher.AddEventListener("foo", func(e Event) {
		count += 1
	})
	dispatcher.AddEventListener("foo", func(e Event) {
		count += 10
	})
	dispatcher.AddEventListener("bar", func(e Event) {
		count += 100
	})
	dispatcher.DispatchEvent(&event{typ: "foo", value: nil, prevValue: nil})
	assert.Equal(t, 11, count)
}

// Ensure that we can add and remove a listener.
func TestRemoveEventListener(t *testing.T) {
	var count int
	f0 := func(e Event) {
		count += 1
	}
	f1 := func(e Event) {
		count += 10
	}

	dispatcher := newEventDispatcher(nil)
	dispatcher.AddEventListener("foo", f0)
	dispatcher.AddEventListener("foo", f1)
	dispatcher.DispatchEvent(&event{typ: "foo"})
	dispatcher.RemoveEventListener("foo", f0)
	dispatcher.DispatchEvent(&event{typ: "foo"})
	assert.Equal(t, 21, count)
}

// Ensure that event is properly passed to listener.
func TestEventListener(t *testing.T) {
	dispatcher := newEventDispatcher("X")
	dispatcher.AddEventListener("foo", func(e Event) {
		assert.Equal(t, "foo", e.Type())
		assert.Equal(t, "X", e.Source())
		assert.Equal(t, 10, e.Value())
		assert.Equal(t, 20, e.PrevValue())
	})
	dispatcher.DispatchEvent(&event{typ: "foo", value: 10, prevValue: 20})
}

// Benchmark the performance of event dispatch.
func BenchmarkEventDispatch(b *testing.B) {
	dispatcher := newEventDispatcher(nil)
	dispatcher.AddEventListener("xxx", func(e Event) {})
	for i := 0; i < b.N; i++ {
		dispatcher.DispatchEvent(&event{typ: "foo", value: 10, prevValue: 20})
	}
}
