// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package latch

import (
	"sync"
	"time"

	"github.com/pingcap/tidb/store/tikv/oracle"
)

const lockChanSize = 100

// LatchesScheduler is used to schedule latches for transactions.
type LatchesScheduler struct {
	latches         *Latches
	unlockCh        chan *Lock
	closed          bool
	lastRecycleTime uint64
	sync.RWMutex
}

// NewScheduler create the LatchesScheduler.
func NewScheduler(size uint) *LatchesScheduler {
	latches := NewLatches(size)
	unlockCh := make(chan *Lock, lockChanSize)
	scheduler := &LatchesScheduler{
		latches:  latches,
		unlockCh: unlockCh,
		closed:   false,
	}
	go scheduler.run()
	return scheduler
}

const expireDuration = 2 * time.Minute
const checkInterval = 1 * time.Minute
const checkCounter = 50000
const latchListCount = 5

func (scheduler *LatchesScheduler) run() {
	var counter int
	wakeupList := make([]*Lock, 0)
	for lock := range scheduler.unlockCh {
		wakeupList = scheduler.latches.release(lock, wakeupList)
		if len(wakeupList) > 0 {
			scheduler.wakeup(wakeupList)
		}

		if lock.commitTS > lock.startTS {
			currentTS := lock.commitTS
			elapsed := tsoSub(currentTS, scheduler.lastRecycleTime)
			if elapsed > checkInterval || counter > checkCounter {
				go scheduler.latches.recycle(lock.commitTS)
				scheduler.lastRecycleTime = currentTS
				counter = 0
			}
		}
		counter++
	}
}

func (scheduler *LatchesScheduler) wakeup(wakeupList []*Lock) {
	for _, lock := range wakeupList {
		if scheduler.latches.acquire(lock) != acquireLocked {
			lock.wg.Done()
		}
	}
}

// Close closes LatchesScheduler.
func (scheduler *LatchesScheduler) Close() {
	scheduler.RWMutex.Lock()
	defer scheduler.RWMutex.Unlock()
	if !scheduler.closed {
		close(scheduler.unlockCh)
		scheduler.closed = true
	}
}

// Lock acquire the lock for transaction with startTS and keys. The caller goroutine
// would be blocked if the lock can't be obtained now. When this function returns,
// the lock state would be either success or stale(call lock.IsStale)
func (scheduler *LatchesScheduler) Lock(startTS uint64, keys [][]byte) *Lock {
	lock := scheduler.latches.genLock(startTS, keys)
	lock.wg.Add(1)
	if scheduler.latches.acquire(lock) == acquireLocked {
		lock.wg.Wait()
	}
	if lock.isLocked() {
		panic("should never run here")
	}
	return lock
}

// UnLock unlocks a lock.
func (scheduler *LatchesScheduler) UnLock(lock *Lock) {
	scheduler.RLock()
	defer scheduler.RUnlock()
	if !scheduler.closed {
		scheduler.unlockCh <- lock
	}
}

func tsoSub(ts1, ts2 uint64) time.Duration {
	t1 := oracle.GetTimeFromTS(ts1)
	t2 := oracle.GetTimeFromTS(ts2)
	return t1.Sub(t2)
}
