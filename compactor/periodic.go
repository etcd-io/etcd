// Copyright 2017 The etcd Authors
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

package compactor

import (
	"context"
	"sync"
	"time"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc"

	"github.com/jonboulle/clockwork"
)

// Periodic compacts the log by purging revisions older than
// the configured retention time.
type Periodic struct {
	clock  clockwork.Clock
	period time.Duration

	rg RevGetter
	c  Compactable

	revs   []int64
	ctx    context.Context
	cancel context.CancelFunc

	// mu protects paused
	mu     sync.RWMutex
	paused bool
}

// NewPeriodic creates a new instance of Periodic compactor that purges
// the log older than h Duration.
func NewPeriodic(h time.Duration, rg RevGetter, c Compactable) *Periodic {
	return newPeriodic(clockwork.NewRealClock(), h, rg, c)
}

func newPeriodic(clock clockwork.Clock, h time.Duration, rg RevGetter, c Compactable) *Periodic {
	t := &Periodic{
		clock:  clock,
		period: h,
		rg:     rg,
		c:      c,
		revs:   make([]int64, 0),
	}
	t.ctx, t.cancel = context.WithCancel(context.Background())
	return t
}

// periodDivisor divides Periodic.period in into checkCompactInterval duration
const periodDivisor = 10

// Run runs periodic compactor.
func (t *Periodic) Run() {
	interval := t.period / time.Duration(periodDivisor)
	go func() {
		initialWait := t.clock.Now()
		for {
			t.revs = append(t.revs, t.rg.Rev())
			select {
			case <-t.ctx.Done():
				return
			case <-t.clock.After(interval):
				t.mu.Lock()
				p := t.paused
				t.mu.Unlock()
				if p {
					continue
				}
			}

			// wait up to initial given period
			if t.clock.Now().Sub(initialWait) < t.period {
				continue
			}

			rev, remaining := t.getRev()
			if rev < 0 {
				continue
			}

			plog.Noticef("Starting auto-compaction at revision %d (retention: %v)", rev, t.period)
			_, err := t.c.Compact(t.ctx, &pb.CompactionRequest{Revision: rev})
			if err == nil || err == mvcc.ErrCompacted {
				// move to next sliding window
				t.revs = remaining
				plog.Noticef("Finished auto-compaction at revision %d", rev)
			} else {
				plog.Noticef("Failed auto-compaction at revision %d (%v)", rev, err)
				plog.Noticef("Retry after %v", interval)
			}
		}
	}()
}

// Stop stops periodic compactor.
func (t *Periodic) Stop() {
	t.cancel()
}

// Pause pauses periodic compactor.
func (t *Periodic) Pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = true
}

// Resume resumes periodic compactor.
func (t *Periodic) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = false
}

func (t *Periodic) getRev() (int64, []int64) {
	i := len(t.revs) - periodDivisor
	if i < 0 {
		return -1, t.revs
	}
	return t.revs[i], t.revs[i+1:]
}
