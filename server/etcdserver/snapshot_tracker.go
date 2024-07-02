// Copyright 2024 The etcd Authors
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

package etcdserver

import (
	"cmp"
	"container/heap"
	"errors"
	"sync"
)

// SnapshotTracker keeps track of all ongoing snapshot creation. To safeguard ongoing snapshot creation,
// only compact the raft log up to the minimum snapshot index in the track.
type SnapshotTracker struct {
	h  minHeap[uint64]
	mu *sync.Mutex
}

func newSnapshotTracker() *SnapshotTracker {
	return &SnapshotTracker{
		h:  minHeap[uint64]{},
		mu: new(sync.Mutex),
	}
}

// MinSnapi returns the minimum snapshot index in the track or an error if the tracker is empty.
func (st *SnapshotTracker) MinSnapi() (uint64, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.h.Len() == 0 {
		return 0, errors.New("SnapshotTracker is empty")
	}
	return st.h[0], nil
}

// Track adds a snapi to the tracker. Make sure to call UnTrack once the snapshot has been created.
func (st *SnapshotTracker) Track(snapi uint64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	heap.Push(&st.h, snapi)
}

// UnTrack removes 'snapi' from the tracker. No action taken if 'snapi' is not found.
func (st *SnapshotTracker) UnTrack(snapi uint64) {
	st.mu.Lock()
	defer st.mu.Unlock()

	for i := 0; i < len((*st).h); i++ {
		if (*st).h[i] == snapi {
			heap.Remove(&st.h, i)
			return
		}
	}
}

// minHeap implements the heap.Interface for E.
type minHeap[E interface {
	cmp.Ordered
}] []E

func (h minHeap[_]) Len() int {
	return len(h)
}

func (h minHeap[_]) Less(i, j int) bool {
	return h[i] < h[j]
}

func (h minHeap[_]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *minHeap[E]) Push(x any) {
	*h = append(*h, x.(E))
}

func (h *minHeap[E]) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
