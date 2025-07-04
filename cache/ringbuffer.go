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
	if r.size == len(r.buffer) { // full → overwrite oldest
		r.tail = (r.tail + 1) % len(r.buffer)
	} else {
		r.size++
	}

	r.buffer[r.head] = event
	r.head = (r.head + 1) % len(r.buffer)
}

// Filter returns the events that satisfy every predicate
// TODO: use binary search on the ring buffer to locate the first entry >= nextRev instead of a full scan
func (r *ringBuffer) Filter(entryPred EntryPredicate) (events []*clientv3.Event) {
	events = make([]*clientv3.Event, 0, r.size)

	for n, i := 0, r.tail; n < r.size; n, i = n+1, (i+1)%len(r.buffer) {
		entry := r.buffer[i]
		if entry == nil {
			panic(fmt.Sprintf("ringBuffer.Filter: unexpected nil entry at index %d", i))
		}
		if entryPred == nil || entryPred(entry) {
			events = append(events, entry)
		}
	}
	return events
}

// PeekLatest returns the most recently-appended event (or nil if empty).
func (r *ringBuffer) PeekLatest() int64 {
	if r.size == 0 {
		return 0
	}
	idx := (r.head - 1 + len(r.buffer)) % len(r.buffer)
	return r.buffer[idx].Kv.ModRevision
}

// PeekOldest returns the oldest event currently stored (or nil if empty).
func (r *ringBuffer) PeekOldest() int64 {
	if r.size == 0 {
		return 0
	}
	return r.buffer[r.tail].Kv.ModRevision
}

func (r *ringBuffer) RebaseHistory() {
	r.head, r.tail, r.size = 0, 0, 0
	for i := range r.buffer {
		r.buffer[i] = nil
	}
}
