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

func newDemux(ctx context.Context, wg *sync.WaitGroup, history *ringBuffer, resyncInterval time.Duration) *demux {
	d := &demux{
		activeWatchers:  make(map[*watcher]int64),
		laggingWatchers: make(map[*watcher]int64),
		history:         history,
		resyncInterval:  resyncInterval,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		d.resyncLoop(ctx)
	}()
	return d
}

func (d *demux) add(w *watcher, nextRev int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Special case: 0 means “newest”.
	if nextRev == 0 {
		d.activeWatchers[w] = 0
		return
	}

	var latestRev int64
	if latestEvent := d.history.PeekLatest(); latestEvent != nil {
		latestRev = latestEvent.Kv.ModRevision
	}
	if nextRev <= latestRev {
		d.laggingWatchers[w] = nextRev
	} else {
		d.activeWatchers[w] = nextRev
	}
}

func (d *demux) remove(w *watcher) {
	func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		delete(d.activeWatchers, w)
		delete(d.laggingWatchers, w)
	}()
	w.Stop()
}

func (d *demux) broadcast(event *clientv3.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for w, nextRev := range d.activeWatchers {
		if event.Kv.ModRevision < nextRev {
			continue
		}

		if !w.enqueueEvent(event) { // buffer overflow → fell behind
			delete(d.activeWatchers, w)
			d.laggingWatchers[w] = event.Kv.ModRevision
		} else {
			d.activeWatchers[w] = event.Kv.ModRevision + 1
		}
	}
}

// Called when etcd compaction invalidates our cached history, so clients should resubscribe.
func (d *demux) purgeAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for w := range d.activeWatchers {
		w.Stop()
	}
	for w := range d.laggingWatchers {
		w.Stop()
	}
	d.activeWatchers = make(map[*watcher]int64)
	d.laggingWatchers = make(map[*watcher]int64)
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

func (d *demux) resyncLaggingWatchers() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for w, nextRev := range d.laggingWatchers {
		// TODO: re-enable key‐predicate in Filter when non‐zero startRev or performance tuning is needed
		// for simplicity, only filter by revision; per-watcher key filtering still happens in enqueueEvent
		missedEvents, _ := d.history.Filter(AfterRev(nextRev))

		if len(missedEvents) == 0 {
			delete(d.laggingWatchers, w)
			d.activeWatchers[w] = nextRev
			continue
		}

		for i := 0; i < len(missedEvents); i++ {
			event := missedEvents[i]
			if event.Kv.ModRevision < nextRev {
				continue
			}

			if !w.enqueueEvent(event) {
				break
			}
			nextRev = event.Kv.ModRevision + 1
		}

		// fully caught-up
		if nextRev > missedEvents[len(missedEvents)-1].Kv.ModRevision {
			delete(d.laggingWatchers, w)
			d.activeWatchers[w] = nextRev
		} else {
			d.laggingWatchers[w] = nextRev
		}
	}
}
