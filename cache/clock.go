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

import "time"

// Clock allows for injecting fake or real clocks into code
// that needs to do arbitrary things based on time.
type Clock interface {
	NewTimer(d time.Duration) Timer
}

// Timer allows for injecting fake or real timers into code
// that needs to do arbitrary things based on time.
type Timer interface {
	Chan() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// realClock implements Clock using the standard time package.
type realClock struct{}

func (realClock) NewTimer(d time.Duration) Timer {
	return &realTimer{timer: time.NewTimer(d)}
}

type realTimer struct {
	timer *time.Timer
}

func (r *realTimer) Chan() <-chan time.Time {
	return r.timer.C
}

func (r *realTimer) Stop() bool {
	return r.timer.Stop()
}

func (r *realTimer) Reset(d time.Duration) bool {
	return r.timer.Reset(d)
}
