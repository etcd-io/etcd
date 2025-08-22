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

import (
	"context"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type demux struct {
	mu sync.RWMutex
	// activeWatchers & laggingWatchers hold the first revision the watcher still needs (nextRev).
	activeWatchers  map[*watcher]int64
	laggingWatchers map[*watcher]int64
	history         ringBuffer[[]*clientv3.Event]
	resyncInterval  time.Duration
}

func NewDemux(ctx context.Context, wg *sync.WaitGroup, historyWindowSize int, resyncInterval time.Duration) *demux {
	d := newDemux(historyWindowSize, resyncInterval)
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.resyncLoop(ctx)
	}()
	return d
}

func newDemux(historyWindowSize int, resyncInterval time.Duration) *demux {
	return &demux{
		activeWatchers:  make(map[*watcher]int64),
		laggingWatchers: make(map[*watcher]int64),
		history:         *newRingBuffer(historyWindowSize, func(batch []*clientv3.Event) int64 { return batch[0].Kv.ModRevision }),
		resyncInterval:  resyncInterval,
	}
}

// resyncLoop periodically tries to catch lagging watchers up by replaying events from History.
func (d *demux) resyncLoop(ctx context.Context) {
	timer := time.NewTimer(d.resyncInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			d.resyncLaggingWatchers()
			timer.Reset(d.resyncInterval)
		}
	}
}

func (d *demux) Register(w *watcher, startingRev int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	latestRev := d.history.PeekLatest()
	if latestRev == 0 {
		if startingRev == 0 {
			d.activeWatchers[w] = 0
		} else {
			d.laggingWatchers[w] = startingRev
		}
		return
	}

	// Special case: 0 means “newest”.
	if startingRev == 0 {
		startingRev = latestRev + 1
	}

	if startingRev <= latestRev {
		d.laggingWatchers[w] = startingRev
	} else {
		d.activeWatchers[w] = startingRev
	}
}

func (d *demux) Unregister(w *watcher) {
	func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		delete(d.activeWatchers, w)
		delete(d.laggingWatchers, w)
	}()
	w.Stop()
}

func (d *demux) Broadcast(events []*clientv3.Event) {
	if len(events) == 0 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	batchStart := 0
	for end := 1; end < len(events); end++ {
		if events[end].Kv.ModRevision != events[batchStart].Kv.ModRevision {
			if end > batchStart {
				d.history.Append(events[batchStart:end])
			}
			batchStart = end
		}
	}
	if batchStart < len(events) {
		d.history.Append(events[batchStart:])
	}

	firstRev := events[0].Kv.ModRevision
	lastRev := events[len(events)-1].Kv.ModRevision
	for w, nextRev := range d.activeWatchers {
		if nextRev != 0 && firstRev > nextRev {
			d.laggingWatchers[w] = nextRev
			delete(d.activeWatchers, w)
			continue
		}
		sendStart := len(events)
		for i, ev := range events {
			if ev.Kv.ModRevision >= nextRev {
				sendStart = i
				break
			}
		}
		if sendStart == len(events) {
			continue
		}

		if !w.enqueueEvent(events[sendStart:]) { // overflow → lagging
			d.laggingWatchers[w] = nextRev
			delete(d.activeWatchers, w)
		} else {
			d.activeWatchers[w] = lastRev + 1
		}
	}
}

// Purge stops all watchers and rebase history on watch errors
func (d *demux) Purge() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.history.RebaseHistory()
	for w := range d.activeWatchers {
		w.Stop()
	}
	for w := range d.laggingWatchers {
		w.Stop()
	}
	d.activeWatchers = make(map[*watcher]int64)
	d.laggingWatchers = make(map[*watcher]int64)
}

// Compact is called when etcd reports a compaction at compactRev to rebase history;
// it keeps provably-too-old watchers for later cancellation, stops others, and clients should resubscribe.
func (d *demux) Compact(compactRev int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.history.RebaseHistory()

	for w, next := range d.activeWatchers {
		if next != 0 && next <= compactRev {
			delete(d.activeWatchers, w)
			d.laggingWatchers[w] = next
		}
	}
}

func (d *demux) resyncLaggingWatchers() {
	d.mu.Lock()
	defer d.mu.Unlock()

	oldestRev := d.history.PeekOldest()
	if oldestRev == 0 {
		return
	}

	for w, nextRev := range d.laggingWatchers {
		if nextRev < oldestRev {
			w.Compact(nextRev)
			delete(d.laggingWatchers, w)
			continue
		}
		// TODO: re-enable key‐predicate in Filter when non‐zero startRev or performance tuning is needed
		enqueueFailed := false
		d.history.AscendGreaterOrEqual(nextRev, func(rev int64, eventBatch []*clientv3.Event) bool {
			if !w.enqueueEvent(eventBatch) { // buffer overflow: watcher still lagging
				enqueueFailed = true
				return false
			}
			nextRev = rev + 1
			return true
		})

		if !enqueueFailed {
			delete(d.laggingWatchers, w)
			d.activeWatchers[w] = nextRev
		} else {
			d.laggingWatchers[w] = nextRev
		}
	}
}
