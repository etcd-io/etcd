// Copyright 2018 The etcd Authors
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

package lease

import (
	"container/heap"
	"testing"
	"time"
)

func TestLeaseQueue(t *testing.T) {
	le := &lessor{
		leaseHeap: make(LeaseQueue, 0),
		leaseMap:  make(map[LeaseID]*Lease),
	}
	heap.Init(&le.leaseHeap)

	// insert in reverse order of expiration time
	for i := 50; i >= 1; i-- {
		exp := time.Now().Add(time.Hour).UnixNano()
		if i == 1 {
			exp = time.Now().UnixNano()
		}
		le.leaseMap[LeaseID(i)] = &Lease{ID: LeaseID(i)}
		heap.Push(&le.leaseHeap, &LeaseWithTime{id: LeaseID(i), time: exp})
	}

	// first element must be front
	if le.leaseHeap[0].id != LeaseID(1) {
		t.Fatalf("first item expected lease ID %d, got %d", LeaseID(1), le.leaseHeap[0].id)
	}

	l, ok, more := le.expireExists()
	if l.ID != 1 {
		t.Fatalf("first item expected lease ID %d, got %d", 1, l.ID)
	}
	if !ok {
		t.Fatal("expect expiry lease exists")
	}
	if more {
		t.Fatal("expect no more expiry lease")
	}

	if le.leaseHeap.Len() != 49 {
		t.Fatalf("expected lease heap pop, got %d", le.leaseHeap.Len())
	}
}
