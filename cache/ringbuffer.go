// Copyright 2025 The etcd Authors
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

package cache

import (
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type ringBuffer struct {
	buffer []batch
	// head is the index immediately after the last non-empty entry in the buffer (i.e., the next write position).
	head, tail, size int
}

// batch groups all events that share one ModRevision.
type batch struct {
	rev    int64
	events []*clientv3.Event
}

type KeyPredicate = func([]byte) bool

func newRingBuffer(capacity int) *ringBuffer {
	// assume capacity > 0 – validated by Cache
	return &ringBuffer{
		buffer: make([]batch, capacity),
	}
}

func (r *ringBuffer) Append(events []*clientv3.Event) {
	start := 0
	for end := 1; end < len(events); end++ {
		if events[end].Kv.ModRevision != events[start].Kv.ModRevision {
			r.append(batch{
				rev:    events[start].Kv.ModRevision,
				events: events[start:end],
			})
			start = end
		}
	}
	if start < len(events) {
		r.append(batch{
			rev:    events[start].Kv.ModRevision,
			events: events[start:],
		})
	}
}

func (r *ringBuffer) append(b batch) {
	if len(b.events) == 0 {
		return
	}
	if r.size == len(r.buffer) {
		r.tail = (r.tail + 1) % len(r.buffer)
	} else {
		r.size++
	}
	r.buffer[r.head] = b
	r.head = (r.head + 1) % len(r.buffer)
}

// Filter returns all events in the buffer whose ModRevision is >= minRev.
// TODO: use binary search on the ring buffer to locate the first entry >= nextRev instead of a full scan
func (r *ringBuffer) Filter(minRev int64) (eventBatches [][]*clientv3.Event) {
	eventBatches = make([][]*clientv3.Event, 0, r.size)

	for n, i := 0, r.tail; n < r.size; n, i = n+1, (i+1)%len(r.buffer) {
		eventBatch := r.buffer[i]
		if eventBatch.events == nil {
			panic(fmt.Sprintf("ringBuffer.Filter: unexpected nil eventBatch at index %d", i))
		}
		if eventBatch.rev >= minRev {
			eventBatches = append(eventBatches, eventBatch.events)
		}
	}
	return eventBatches
}

// PeekLatest returns the most recently-appended event (or nil if empty).
func (r *ringBuffer) PeekLatest() int64 {
	if r.size == 0 {
		return 0
	}
	idx := (r.head - 1 + len(r.buffer)) % len(r.buffer)
	return r.buffer[idx].rev
}

// PeekOldest returns the oldest event currently stored (or nil if empty).
func (r *ringBuffer) PeekOldest() int64 {
	if r.size == 0 {
		return 0
	}
	return r.buffer[r.tail].rev
}

func (r *ringBuffer) RebaseHistory() {
	r.head, r.tail, r.size = 0, 0, 0
	for i := range r.buffer {
		r.buffer[i] = batch{}
	}
}
