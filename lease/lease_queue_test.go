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
	"testing"
	"time"
)

func TestLeaseQueue(t *testing.T) {
	expiredRetryInterval := 100 * time.Millisecond
	le := &lessor{
		leaseExpiredNotifier:      newLeaseExpiredNotifier(),
		leaseMap:                  make(map[LeaseID]*Lease),
		expiredLeaseRetryInterval: expiredRetryInterval,
	}
	le.leaseExpiredNotifier.Init()

	// insert in reverse order of expiration time
	for i := 50; i >= 1; i-- {
		now := time.Now()
		exp := now.Add(time.Hour)
		if i == 1 {
			exp = now
		}
		le.leaseMap[LeaseID(i)] = &Lease{ID: LeaseID(i)}
		le.leaseExpiredNotifier.RegisterOrUpdate(&LeaseWithTime{id: LeaseID(i), time: exp})
	}

	// first element is expired.
	if le.leaseExpiredNotifier.Poll().id != LeaseID(1) {
		t.Fatalf("first item expected lease ID %d, got %d", LeaseID(1), le.leaseExpiredNotifier.Poll().id)
	}

	existExpiredEvent := func() {
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

		if le.leaseExpiredNotifier.Len() != 50 {
			t.Fatalf("expected the expired lease to be pushed back to the heap, heap size got %d", le.leaseExpiredNotifier.Len())
		}

		if le.leaseExpiredNotifier.Poll().id != LeaseID(1) {
			t.Fatalf("first item expected lease ID %d, got %d", LeaseID(1), le.leaseExpiredNotifier.Poll().id)
		}
	}

	noExpiredEvent := func() {
		// re-acquire the expired item, nothing exists
		_, ok, more := le.expireExists()
		if ok {
			t.Fatal("expect no expiry lease exists")
		}
		if more {
			t.Fatal("expect no more expiry lease")
		}
	}

	existExpiredEvent() // first acquire
	noExpiredEvent()    // second acquire
	time.Sleep(expiredRetryInterval)
	existExpiredEvent() // acquire after retry interval
}
