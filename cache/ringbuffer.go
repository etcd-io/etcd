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

type ringBuffer[T any] struct {
	buffer []entry[T]
	// head is the index immediately after the last non-empty entry in the buffer (i.e., the next write position).
	head, tail, size int
	revisionOf       RevisionOf[T]
}

type entry[T any] struct {
	revision int64
	item     T
}

type (
	KeyPredicate      = func([]byte) bool
	RevisionOf[T any] func(T) int64
	IterFunc[T any]   func(rev int64, item T) bool
)

func newRingBuffer[T any](capacity int, revisionOf RevisionOf[T]) *ringBuffer[T] {
	// assume capacity > 0 – validated by Cache
	return &ringBuffer[T]{
		buffer:     make([]entry[T], capacity),
		revisionOf: revisionOf,
	}
}

func (r *ringBuffer[T]) Append(item T) {
	entry := entry[T]{revision: r.revisionOf(item), item: item}
	if r.size == len(r.buffer) {
		r.tail = (r.tail + 1) % len(r.buffer)
	} else {
		r.size++
	}
	r.buffer[r.head] = entry
	r.head = (r.head + 1) % len(r.buffer)
}

// AscendGreaterOrEqual iterates through entries in ascending order starting from the first entry with revision >= pivot.
// TODO: use binary search on the ring buffer to locate the first entry >= nextRev instead of a full scan
func (r *ringBuffer[T]) AscendGreaterOrEqual(pivot int64, iter IterFunc[T]) {
	if r.size == 0 {
		return
	}

	for n, i := 0, r.tail; n < r.size; n, i = n+1, (i+1)%len(r.buffer) {
		entry := r.buffer[i]

		if entry.revision < pivot {
			continue
		}

		if !iter(entry.revision, entry.item) {
			return
		}
	}
}

// AscendLessThan iterates in ascending order over entries with revision < pivot.
func (r *ringBuffer[T]) AscendLessThan(pivot int64, iter IterFunc[T]) {
	if r.size == 0 {
		return
	}

	for n, i := 0, r.tail; n < r.size; n, i = n+1, (i+1)%len(r.buffer) {
		entry := r.buffer[i]

		if entry.revision >= pivot {
			return
		}

		if !iter(entry.revision, entry.item) {
			return
		}
	}
}

// DescendGreaterThan iterates in descending order over entries with revision > pivot.
func (r *ringBuffer[T]) DescendGreaterThan(pivot int64, iter IterFunc[T]) {
	if r.size == 0 {
		return
	}

	for n, i := 0, r.moduloIndex(r.head-1); n < r.size; n, i = n+1, r.moduloIndex(i-1) {
		entry := r.buffer[i]

		if entry.revision <= pivot {
			return
		}

		if !iter(entry.revision, entry.item) {
			return
		}
	}
}

// DescendLessOrEqual iterates in descending order over entries with revision <= pivot.
func (r *ringBuffer[T]) DescendLessOrEqual(pivot int64, iter IterFunc[T]) {
	if r.size == 0 {
		return
	}

	for n, i := 0, r.moduloIndex(r.head-1); n < r.size; n, i = n+1, r.moduloIndex(i-1) {
		entry := r.buffer[i]

		if entry.revision > pivot {
			continue
		}

		if !iter(entry.revision, entry.item) {
			return
		}
	}
}

// PeekLatest returns the most recently-appended revision (or 0 if empty).
func (r *ringBuffer[T]) PeekLatest() int64 {
	if r.size == 0 {
		return 0
	}
	idx := (r.head - 1 + len(r.buffer)) % len(r.buffer)
	return r.buffer[idx].revision
}

// PeekOldest returns the oldest revision currently stored (or 0 if empty).
func (r *ringBuffer[T]) PeekOldest() int64 {
	if r.size == 0 {
		return 0
	}
	return r.buffer[r.tail].revision
}

func (r *ringBuffer[T]) RebaseHistory() {
	r.head, r.tail, r.size = 0, 0, 0
	for i := range r.buffer {
		r.buffer[i] = entry[T]{}
	}
}

func (r *ringBuffer[T]) moduloIndex(index int) int {
	return (index + len(r.buffer)) % len(r.buffer)
}
