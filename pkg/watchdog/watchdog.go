// Copyright 2023 The etcd Authors
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

package watchdog

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Interface defines the watchdog interface.
type Interface interface {
	// Register registers an activity with the given name in watchdog.
	// A callback function is returned, and which is supposed to be
	// invoked to unregister the activity when it is done.
	// The watchdog keeps monitoring the activity until the callback
	// function is called.
	Register(name string) func()

	// Execute executes a function under the watchdog monitoring.
	// It's just a syntactic sugar of `Register`, because it calls
	// the callback function automatically on behalf of the callers.
	Execute(name string, fn func())
}

const (
	tickMs = 100
)

type activity struct {
	id              uint64
	name            string
	inactiveElapsed int
}

type watchdog struct {
	lg             *zap.Logger
	nextActivityId uint64

	ticker              *time.Ticker
	inactiveTimeoutTick int

	mu         sync.Mutex
	activities map[uint64]*activity

	stopC   <-chan struct{}
	cleanup func()
}

func New(lg *zap.Logger, stopC <-chan struct{}, cleanup func(), inactiveTimeoutMs int64) Interface {
	wdInstance := &watchdog{
		lg:      lg,
		stopC:   stopC,
		cleanup: cleanup,

		inactiveTimeoutTick: int(inactiveTimeoutMs / tickMs),
		activities:          make(map[uint64]*activity),
		ticker:              time.NewTicker(tickMs * time.Millisecond),
	}
	go wdInstance.run()

	return wdInstance
}

func (wd *watchdog) run() {
	inactiveTimeout := time.Duration(wd.inactiveTimeoutTick*tickMs) * time.Millisecond
	wd.lg.Info("Watchdog is running", zap.Duration("inactiveTimeout", inactiveTimeout))
	for {
		select {
		case <-wd.ticker.C:
			wd.mu.Lock()
			for _, v := range wd.activities {
				v.inactiveElapsed++
				if v.inactiveElapsed > wd.inactiveTimeoutTick/2 {
					elapsedTime := time.Duration(v.inactiveElapsed*tickMs) * time.Millisecond
					wd.lg.Warn("Slow activity detected", zap.String("activity", v.name), zap.Duration("duration", elapsedTime))
					if v.inactiveElapsed > wd.inactiveTimeoutTick {
						wd.mu.Unlock()
						wd.cleanup()
						wd.lg.Panic("Inactive activity detected", zap.String("activity", v.name), zap.Duration("duration", elapsedTime))
					}
				}
			}
			wd.mu.Unlock()
		case <-wd.stopC:
			wd.lg.Info("Watchdog stopped")
			return
		}
	}
}

func (wd *watchdog) Register(name string) func() {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	id := wd.nextActivityId
	wd.activities[id] = &activity{
		id:              id,
		name:            name,
		inactiveElapsed: 0,
	}
	wd.nextActivityId++

	return func() {
		wd.reset(id)
	}
}

func (wd *watchdog) Execute(name string, fn func()) {
	unregister := wd.Register(name)
	defer unregister()

	fn()
}

func (wd *watchdog) reset(id uint64) {
	wd.mu.Lock()
	defer wd.mu.Unlock()
	delete(wd.activities, id)
}
