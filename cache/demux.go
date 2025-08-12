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
	history         *ringBuffer
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
		history:         newRingBuffer(historyWindowSize),
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

	// Special case: 0 means “newest”.
	if startingRev == 0 {
		if latestRev == 0 {
			d.activeWatchers[w] = 0
			return
		}
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

	d.history.Append(events)

	lastRev := events[len(events)-1].Kv.ModRevision
	for w, nextRev := range d.activeWatchers {
		start := len(events)
		for i, ev := range events {
			if ev.Kv.ModRevision >= nextRev {
				start = i
				break
			}
		}

		if start == len(events) {
			continue
		}

		if !w.enqueueEvent(events[start:]) { // overflow → lagging
			d.laggingWatchers[w] = nextRev
			delete(d.activeWatchers, w)
		} else {
			d.activeWatchers[w] = lastRev + 1
		}
	}
}

// Purge is called when etcd compaction invalidates our cached history, so clients should resubscribe.
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

func (d *demux) resyncLaggingWatchers() {
	d.mu.Lock()
	defer d.mu.Unlock()

	oldestRev := d.history.PeekOldest()
	for w, nextRev := range d.laggingWatchers {
		if oldestRev != 0 && nextRev < oldestRev {
			w.Stop()
			delete(d.laggingWatchers, w)
			continue
		}
		// TODO: re-enable key‐predicate in Filter when non‐zero startRev or performance tuning is needed
		missed := d.history.Filter(nextRev)

		enqueueFailed := false
		for _, eventBatch := range missed {
			if !w.enqueueEvent(eventBatch) { // buffer overflow: watcher still lagging
				enqueueFailed = true
				break
			}
			nextRev = eventBatch[0].Kv.ModRevision + 1
		}

		if !enqueueFailed {
			delete(d.laggingWatchers, w)
			d.activeWatchers[w] = nextRev
		} else {
			d.laggingWatchers[w] = nextRev
		}
	}
}

func (d *demux) PeekOldest() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.history.PeekOldest()
}
