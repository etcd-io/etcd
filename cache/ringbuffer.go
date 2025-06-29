package cache

import (
	"fmt"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type ringBuffer struct {
	mu     sync.RWMutex
	buffer []*clientv3.Event
	// head is the index immediately after the last non-empty entry in the buffer (i.e., the next write position).
	head, tail, size int
}

// EntryPredicate lets callers decide which entries to keep (e.g. “after revision X”)
type (
	EntryPredicate func(*clientv3.Event) bool
	KeyPredicate   = func([]byte) bool
)

func newRingBuffer(capacity int) *ringBuffer {
	// assume capacity > 0 – validated by Cache
	return &ringBuffer{
		buffer: make([]*clientv3.Event, capacity),
	}
}

func (r *ringBuffer) Append(event *clientv3.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size == len(r.buffer) { // full → overwrite oldest
		r.tail = (r.tail + 1) % len(r.buffer)
	} else {
		r.size++
	}

	r.buffer[r.head] = event
	r.head = (r.head + 1) % len(r.buffer)
}

// Filter returns the events that satisfy every predicate
func (r *ringBuffer) Filter(entryPred EntryPredicate) ([]*clientv3.Event, int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*clientv3.Event, 0, len(r.buffer))
	var latestRev int64

	for i := r.tail; i != r.head; i = (i + 1) % len(r.buffer) {
		entry := r.buffer[i]
		if entry == nil {
			panic(fmt.Sprintf("ringBuffer.Filter: unexpected nil entry at index %d", i))
		}
		latestRev = max(latestRev, entry.Kv.ModRevision)
		if entryPred == nil || entryPred(entry) {
			out = append(out, entry)
		}
	}
	return out, latestRev
}

// PeekLatest returns the most recently-appended event (or nil if empty).
func (r *ringBuffer) PeekLatest() *clientv3.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.head == r.tail {
		return nil
	}
	idx := (r.head - 1 + len(r.buffer)) % len(r.buffer)
	return r.buffer[idx]
}

// PeekOldest returns the oldest event currently stored (or nil if empty).
func (r *ringBuffer) PeekOldest() *clientv3.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.size == 0 {
		return nil
	}
	return r.buffer[r.tail]
}

func (r *ringBuffer) RebaseHistory() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.head, r.tail = 0, 0
	for i := range r.buffer {
		r.buffer[i] = nil
	}
}
