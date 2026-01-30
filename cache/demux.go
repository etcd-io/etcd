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
	"errors"
	"sync"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type demux struct {
	mu sync.RWMutex
	// activeWatchers & laggingWatchers hold the first revision the watcher still needs (nextRev).
	activeWatchers  map[*watcher]int64
	laggingWatchers map[*watcher]int64
	resyncInterval  time.Duration
	// Range of revisions maintained for demux operations, inclusive. Broader than history as event revision is not contious.
	// maxRev tracks highest seen revision; minRev sets watcher compaction threshold (updated to evictedRev+1 on history overflow)
	minRev, maxRev int64
	// History stores events within [minRev, maxRev].
	history ringBuffer[[]*clientv3.Event]
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

	if d.maxRev == 0 {
		if startingRev == 0 {
			d.activeWatchers[w] = 0
		} else {
			d.laggingWatchers[w] = startingRev
		}
		return
	}

	// Special case: 0 means “newest”.
	if startingRev == 0 {
		startingRev = d.maxRev + 1
	}

	if startingRev <= d.maxRev {
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

func (d *demux) Init(minRev int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if minRev == 0 {
		return
	}
	if d.minRev == 0 {
		// Watch started for empty demux
		d.minRev = minRev
		return
	}
	if d.maxRev == 0 {
		// Watch started on initialized demux that never got any event.
		d.purge()
		d.minRev = minRev
		return
	}
	if minRev == d.maxRev+1 {
		// Watch continuing from last revision it observed.
		return
	}
	// Watch opened on revision mismatching dmux last observed revision.
	d.purge()
	d.minRev = minRev
}

func (d *demux) Broadcast(resp clientv3.WatchResponse) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.minRev == 0 {
		return errors.New("demux: not initialized")
	}
	err := validateRevisions(resp, d.maxRev)
	if err != nil {
		return err
	}
	d.updateStoreLocked(resp)
	d.broadcastLocked(resp)
	return nil
}

func (d *demux) LatestRev() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.maxRev
}

func (d *demux) updateStoreLocked(resp clientv3.WatchResponse) {
	if resp.IsProgressNotify() {
		d.maxRev = resp.Header.Revision
		return
	}
	if len(resp.Events) == 0 {
		return
	}
	events := resp.Events
	batchStart := 0
	for end := 1; end < len(events); end++ {
		if events[end].Kv.ModRevision != events[batchStart].Kv.ModRevision {
			if end > batchStart {
				if end+1 == len(events) && d.history.full() {
					d.minRev = d.history.PeekOldest() + 1
				}
				d.history.Append(events[batchStart:end])
			}
			batchStart = end
		}
	}
	if batchStart < len(events) {
		if d.history.full() {
			d.minRev = d.history.PeekOldest() + 1
		}
		d.history.Append(events[batchStart:])
	}
	d.maxRev = events[len(events)-1].Kv.ModRevision
}

func (d *demux) broadcastLocked(resp clientv3.WatchResponse) {
	switch {
	case resp.IsProgressNotify():
		d.broadcastProgressLocked(resp.Header.Revision)
	case len(resp.Events) != 0:
		d.broadcastEventsLocked(resp.Events)
	default:
	}
}

func (d *demux) broadcastProgressLocked(progressRev int64) {
	for w, nextRev := range d.activeWatchers {
		if nextRev >= progressRev {
			continue
		}
		resp := clientv3.WatchResponse{
			Header: etcdserverpb.ResponseHeader{
				Revision: progressRev,
			},
		}
		if w.enqueueResponse(resp) {
			d.activeWatchers[w] = progressRev + 1
		}
	}
}

func (d *demux) broadcastEventsLocked(events []*clientv3.Event) {
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

		if !w.enqueueResponse(clientv3.WatchResponse{
			Events: events[sendStart:],
		}) { // overflow → lagging
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
	d.purge()
}

func (d *demux) purge() {
	d.maxRev = 0
	d.minRev = 0
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
	d.purge()
}

func (d *demux) resyncLaggingWatchers() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.minRev == 0 {
		return
	}

	for w, nextRev := range d.laggingWatchers {
		if nextRev < d.minRev {
			w.Compact(nextRev)
			delete(d.laggingWatchers, w)
			continue
		}
		// TODO: re-enable key‐predicate in Filter when non‐zero startRev or performance tuning is needed
		resyncSuccess := true
		d.history.AscendGreaterOrEqual(nextRev, func(rev int64, eventBatch []*clientv3.Event) bool {
			resp := clientv3.WatchResponse{
				Events: eventBatch,
			}
			if !w.enqueueResponse(resp) { // buffer overflow: watcher still lagging
				resyncSuccess = false
				return false
			}
			nextRev = rev + 1
			return true
		})
		// Send progress to just resync.
		if resyncSuccess {
			resp := clientv3.WatchResponse{
				Header: etcdserverpb.ResponseHeader{Revision: d.maxRev},
			}
			if d.maxRev > nextRev && w.enqueueResponse(resp) {
				nextRev = d.maxRev + 1
			}
			delete(d.laggingWatchers, w)
			d.activeWatchers[w] = nextRev
		} else {
			d.laggingWatchers[w] = nextRev
		}
	}
}
