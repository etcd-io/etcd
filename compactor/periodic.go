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

// Run runs periodic compactor.
func (t *Periodic) Run() {
	go t.run()
}

// periodically fetches revisions and ensures that
// first element is always up-to-date for retention window
func (t *Periodic) run() {
	initialWait := t.clock.Now()
	fetchInterval := t.getInterval()
	retryInterval, retries := t.getRetryInterval()

	// e.g. period 9h with compaction period 1h, then retain up-to 9 revs
	// e.g. period 12h with compaction period 1h, then retain up-to 12 revs
	// e.g. period 20m with compaction period 20m, then retain up-to 1 rev
	retentions := int(t.period / fetchInterval)
	for {
		t.revs = append(t.revs, t.rg.Rev())
		if len(t.revs) > retentions {
			t.revs = t.revs[1:]
		}

		select {
		case <-t.ctx.Done():
			return
		case <-t.clock.After(fetchInterval):
			t.mu.RLock()
			p := t.paused
			t.mu.RUnlock()
			if p {
				continue
			}
		}

		// no compaction until initial wait period
		if t.clock.Now().Sub(initialWait) < t.period {
			continue
		}
		rev := t.revs[0]

		for i := 0; i < retries; i++ {
			plog.Noticef("Starting auto-compaction at revision %d (retention: %v)", rev, t.period)
			_, err := t.c.Compact(t.ctx, &pb.CompactionRequest{Revision: rev})
			if err == nil || err == mvcc.ErrCompacted {
				// compactor succeeds at revs[0], move sliding window
				t.revs = t.revs[1:]
				plog.Noticef("Finished auto-compaction at revision %d", rev)
				break
			}

			// compactor fails at revs[0]:
			//  1. retry revs[0], so long as revs[0] is up-to-date
			//  2. retry revs[1], when revs[0] becomes stale
			plog.Noticef("Failed auto-compaction at revision %d (%v)", rev, err)
			plog.Noticef("Retry after %v", retryInterval)
			paused := false
			select {
			case <-t.ctx.Done():
				return
			case <-t.clock.After(retryInterval):
				t.mu.RLock()
				paused = t.paused
				t.mu.RUnlock()
			}
			if paused {
				break
			}
		}
	}
}

// If given compaction period x is <1-hour, compact every x duration, with x retention window
// (e.g. --auto-compaction-mode 'periodic' --auto-compaction-retention='10m', then compact every 10-minute).
// If given compaction period x is >1-hour, compact every 1-hour, with x retention window
// (e.g. --auto-compaction-mode 'periodic' --auto-compaction-retention='72h', then compact every 1-hour).
func (t *Periodic) getInterval() time.Duration {
	itv := t.period
	if itv > time.Hour {
		itv = time.Hour
	}
	return itv
}

const retryDivisor = 10

// divide by 10 to retry faster
// e.g. given period 2-hour, retry in 12-min rather than 1-hour (compaction period)
func (t *Periodic) getRetryInterval() (itv time.Duration, retries int) {
	// if period 10-min, retry once rather than 10 times (100-min)
	itv, retries = t.period, 1
	if itv > time.Hour {
		itv /= retryDivisor
		retries = retryDivisor
	}
	return itv, retries
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
