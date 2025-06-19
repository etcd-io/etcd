package cache

import (
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type entry *clientv3.Event

type ringBuffer struct {
	mu                sync.RWMutex
	buffer            []entry
	head, tail        int
	oldestRev, latest int64
}

func newRingBuffer(capacity int) *ringBuffer {
	// assume capacity > 0 – validated by Cache
	return &ringBuffer{buffer: make([]entry, capacity)}
}

func (r *ringBuffer) Append(ev *clientv3.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rev := ev.Kv.ModRevision
	r.buffer[r.head] = ev
	r.head = (r.head + 1) % len(r.buffer)

	if r.head == r.tail { // buffer full -> overwrite oldest
		r.tail = (r.tail + 1) % len(r.buffer)
	}
	r.oldestRev = r.buffer[r.tail].Kv.ModRevision
	r.latest = rev
}

func (r *ringBuffer) GetSince(rev int64, pred KeyPredicate) ([]*clientv3.Event, int64) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.tail == r.head || rev > r.latest {
		return nil, r.latest
	}
	out := make([]*clientv3.Event, 0, len(r.buffer))
	for i := r.tail; i != r.head; i = (i + 1) % len(r.buffer) {
		e := r.buffer[i]
		if e != nil && e.Kv.ModRevision >= rev && (pred == nil || pred(e.Kv.Key)) {
			out = append(out, e)
		}
	}
	return out, r.latest
}

func (r *ringBuffer) OldestRevision() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.oldestRev
}

func (r *ringBuffer) LatestRevision() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.latest
}

func (r *ringBuffer) RebaseHistory(compacted int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.head, r.tail = 0, 0
	r.oldestRev, r.latest = compacted, compacted
	for i := range r.buffer {
		r.buffer[i] = nil
	}
}

func (r *ringBuffer) FeedFrom(src History) {
	events, _ := src.GetSince(src.OldestRevision(), nil)
	for _, ev := range events {
		r.Append(ev)
	}
}

// Ensure ringBuffer implements History.
var _ History = (*ringBuffer)(nil)
