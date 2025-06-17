package cache

import clientv3 "go.etcd.io/etcd/client/v3"

type entry struct {
	rev   int64
	event *clientv3.Event
}

type ringBuffer struct {
	buf               []entry
	head, size        int
	oldestRev, latest int64
}

func newRingBuffer(capacity int) *ringBuffer {
	if capacity <= 0 {
		capacity = 1024
	}
	return &ringBuffer{buf: make([]entry, capacity)}
}

// cloneEvent deep‑copies event so once it leaves the ring it's immutable.
func cloneEvent(event *clientv3.Event) *clientv3.Event {
	dup := *event
	if event.Kv != nil {
		kv := *event.Kv
		kv.Key = append([]byte(nil), event.Kv.Key...)
		kv.Value = append([]byte(nil), event.Kv.Value...)
		dup.Kv = &kv
	}
	return &dup
}

// Append appends event, overwriting oldest if full.
func (r *ringBuffer) Append(event *clientv3.Event) {
	rev := event.Kv.ModRevision
	r.buf[r.head] = entry{rev: rev, event: cloneEvent(event)}
	r.head = (r.head + 1) % len(r.buf)

	if r.size < len(r.buf) {
		r.size++
	} else {
		// we overwrote the oldest, advance oldestRev
		r.oldestRev = r.buf[r.head].rev
	}
	if r.oldestRev == 0 {
		r.oldestRev = rev
	}
	r.latest = rev
}

// GetSince returns events with ModRevision >= rev filtered by pred (nil = all).
func (r *ringBuffer) GetSince(rev int64, pred KeyPredicate) []*clientv3.Event {
	if r.size == 0 || rev > r.latest {
		return nil
	}

	out := make([]*clientv3.Event, 0, r.size)
	startIdx := (r.head - r.size + len(r.buf)) % len(r.buf)
	for i := 0; i < r.size; i++ {
		ent := r.buf[(startIdx+i)%len(r.buf)]
		if ent.rev >= rev && (pred == nil || pred(ent.event.Kv.Key)) {
			out = append(out, cloneEvent(ent.event))
		}
	}
	return out
}

func (r *ringBuffer) OldestRevision() int64 { return r.oldestRev }
func (r *ringBuffer) LatestRevision() int64 { return r.latest }

func (r *ringBuffer) RebaseHistory(newCompactedRev int64) {
	r.head, r.size = 0, 0
	r.oldestRev, r.latest = newCompactedRev, newCompactedRev
	for i := range r.buf {
		r.buf[i] = entry{}
	}
}

// FeedFrom copies every entry from src into the ring in order.
func (r *ringBuffer) FeedFrom(src History) {
	for _, ev := range src.GetSince(src.OldestRevision(), nil) {
		r.Append(ev)
	}
}

// Ensure ringBuffer implements History.
var _ History = (*ringBuffer)(nil)
