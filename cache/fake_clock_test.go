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

// Minimal fake clock for testing, based on
// https://github.com/kubernetes/utils/blob/master/clock/testing/fake_clock.go

import (
	"sync"
	"time"
)

type fakeClock struct {
	lock    sync.Mutex
	time    time.Time
	waiters []*fakeClockWaiter
}

type fakeClockWaiter struct {
	targetTime time.Time
	destChan   chan time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{time: time.Now()}
}

func (f *fakeClock) NewTimer(d time.Duration) Timer {
	f.lock.Lock()
	defer f.lock.Unlock()
	ch := make(chan time.Time, 1)
	timer := &fakeTimer{
		fakeClock: f,
		waiter: fakeClockWaiter{
			targetTime: f.time.Add(d),
			destChan:   ch,
		},
	}
	f.waiters = append(f.waiters, &timer.waiter)
	return timer
}

func (f *fakeClock) Advance(d time.Duration) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.time = f.time.Add(d)
	newWaiters := make([]*fakeClockWaiter, 0, len(f.waiters))
	for _, w := range f.waiters {
		if !w.targetTime.After(f.time) {
			w.destChan <- f.time
		} else {
			newWaiters = append(newWaiters, w)
		}
	}
	f.waiters = newWaiters
}

type fakeTimer struct {
	fakeClock *fakeClock
	waiter    fakeClockWaiter
}

func (t *fakeTimer) Chan() <-chan time.Time {
	return t.waiter.destChan
}

func (t *fakeTimer) Stop() bool {
	t.fakeClock.lock.Lock()
	defer t.fakeClock.lock.Unlock()
	active := false
	newWaiters := make([]*fakeClockWaiter, 0, len(t.fakeClock.waiters))
	for _, w := range t.fakeClock.waiters {
		if w != &t.waiter {
			newWaiters = append(newWaiters, w)
		} else {
			active = true
		}
	}
	t.fakeClock.waiters = newWaiters
	return active
}

func (t *fakeTimer) Reset(d time.Duration) bool {
	t.fakeClock.lock.Lock()
	defer t.fakeClock.lock.Unlock()
	active := false
	t.waiter.targetTime = t.fakeClock.time.Add(d)
	for _, w := range t.fakeClock.waiters {
		if w == &t.waiter {
			// If timer is found, it has not been fired yet.
			active = true
			break
		}
	}
	if !active {
		t.fakeClock.waiters = append(t.fakeClock.waiters, &t.waiter)
	}
	return active
}
