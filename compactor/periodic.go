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

func (t *Periodic) Run() {
	fetchInterval := t.getFetchInterval()
	retryInterval := t.getRetryInterval()
	retentions := int(t.period/fetchInterval) + 1 // number of revs to keep for t.period
	notify := make(chan struct{}, 1)

	// periodically updates t.revs and notify to the other goroutine
	go func() {
		for {
			rev := t.rg.Rev()
			t.mu.Lock()
			t.revs = append(t.revs, rev)
			if len(t.revs) > retentions {
				t.revs = t.revs[1:] // t.revs[0] is always the rev at t.period ago
			}
			t.mu.Unlock()

			select {
			case notify <- struct{}{}:
			default:
				// compaction can take time more than interval
			}

			select {
			case <-t.ctx.Done():
				return
			case <-t.clock.After(fetchInterval):
			}
		}
	}()

	// run compaction triggered by the other goroutine thorough the notify channel
	// or internal periodic retry
	go func() {
		var lastCompactedRev int64
		for {
			select {
			case <-t.ctx.Done():
				return
			case <-notify:
				// from the other goroutine
			case <-t.clock.After(retryInterval):
				// for retry
				// when t.rev is not updated, this event will be ignored later,
				// so we don't need to think about race with <-notify.
			}

			t.mu.Lock()
			p := t.paused
			rev := t.revs[0]
			len := len(t.revs)
			t.mu.Unlock()
			if p {
				continue
			}

			// it's too early to start working
			if len != retentions {
				continue
			}

			// if t.revs is not updated, we can ignore the event.
			// it's not the first time to try comapction in this interval.
			if rev == lastCompactedRev {
				continue
			}

			plog.Noticef("Starting auto-compaction at revision %d (retention: %v)", rev, t.period)
			_, err := t.c.Compact(t.ctx, &pb.CompactionRequest{Revision: rev})
			if err == nil || err == mvcc.ErrCompacted {
				plog.Noticef("Finished auto-compaction at revision %d", rev)
				lastCompactedRev = rev
			} else {
				plog.Noticef("Failed auto-compaction at revision %d (%v)", rev, err)
				plog.Noticef("Retry after %s", retryInterval)
			}
		}
	}()
}

// if given compaction period x is <1-hour, compact every x duration.
// (e.g. --auto-compaction-mode 'periodic' --auto-compaction-retention='10m', then compact every 10-minute)
// if given compaction period x is >1-hour, compact every hour.
// (e.g. --auto-compaction-mode 'periodic' --auto-compaction-retention='2h', then compact every 1-hour)
func (t *Periodic) getFetchInterval() time.Duration {
	itv := t.period
	if itv > time.Hour {
		itv = time.Hour
	}
	return itv
}

const retryDivisor = 10

func (t *Periodic) getRetryInterval() time.Duration {
	itv := t.period / retryDivisor
	// we don't want to too aggressive retries
	// and also jump between 6-minute through 60-minute
	if itv < (6 * time.Minute) { // t.period is less than hour
		// if t.period is less than 6-minute,
		// retry interval is t.period.
		// if we divide byretryDivisor, it's too aggressive
		if t.period < 6*time.Minute {
			itv = t.period
		} else {
			itv = 6 * time.Minute
		}
	}
	return itv
}

func (t *Periodic) Stop() {
	t.cancel()
}

func (t *Periodic) Pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = true
}

func (t *Periodic) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = false
}
