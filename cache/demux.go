package cache

import (
	"sync"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type demux struct {
	mu              sync.RWMutex
	activeWatchers  map[*watcher]struct{}
	laggingWatchers map[*watcher]int64
	history         History
	resyncInterval  time.Duration
}

func newDemux(history History, resyncInterval time.Duration) *demux {
	d := &demux{
		activeWatchers:  make(map[*watcher]struct{}),
		laggingWatchers: make(map[*watcher]int64),
		history:         history,
		resyncInterval:  resyncInterval,
	}

	go d.resyncLoop() // background catch-up
	return d
}

func (d *demux) add(w *watcher) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeWatchers[w] = struct{}{}
}

func (d *demux) remove(w *watcher) {
	d.mu.Lock()
	delete(d.activeWatchers, w)
	delete(d.laggingWatchers, w)

	if atomic.CompareAndSwapInt32(&w.stopped, 0, 1) {
		close(w.eventChan)
	}
	d.mu.Unlock()
}

func (d *demux) broadcast(event *clientv3.Event) {
	d.mu.RLock()
	activeWatchersList := make([]*watcher, 0, len(d.activeWatchers))
	for w := range d.activeWatchers {
		activeWatchersList = append(activeWatchersList, w)
	}
	d.mu.RUnlock()

	for _, w := range activeWatchersList {
		if atomic.LoadInt32(&w.stopped) == 1 {
			continue // already gone
		}
		if ok := w.enqueueEvent(event); !ok {
			d.moveToUnsync(w)
		}
	}
}

// moveToUnsync transfers a watcher from activeWatchers to laggingWatchers.
func (d *demux) moveToUnsync(w *watcher) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.activeWatchers[w]; ok {
		delete(d.activeWatchers, w)
		d.laggingWatchers[w] = w.lastRev + 1 // first missed revision
	}
}

// purgeAll closes all watcher channels and clears them.
// Called when etcd compaction invalidates our cached history, so clients should resubscribe.
func (d *demux) purgeAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for w := range d.activeWatchers {
		if atomic.CompareAndSwapInt32(&w.stopped, 0, 1) {
			close(w.eventChan)
		}
	}
	for w := range d.laggingWatchers {
		if atomic.CompareAndSwapInt32(&w.stopped, 0, 1) {
			close(w.eventChan)
		}
	}
	d.activeWatchers, d.laggingWatchers = make(map[*watcher]struct{}), make(map[*watcher]int64)
}

func (d *demux) resyncLoop() {
	ticker := time.NewTicker(d.resyncInterval)
	defer ticker.Stop()

	for range ticker.C {
		// copy unsync map so we can work without locks.
		d.mu.RLock()
		laggingWatchersList := make(map[*watcher]int64, len(d.laggingWatchers))
		for w, next := range d.laggingWatchers {
			laggingWatchersList[w] = next
		}
		d.mu.RUnlock()

		// Try to replay history for each lagging watcher.
		for w, next := range laggingWatchersList {
			missed, _ := d.history.GetSince(next, w.predicate)
			if len(missed) == 0 {
				continue
			}

			i := 0
			for i < len(missed) {
				if w.enqueueEvent(missed[i]) {
					i++
				} else {
					break // still full: keep watcher unsynced
				}
			}

			if i == len(missed) {
				// fully delivered -> move back to activeWatchers
				d.mu.Lock()
				delete(d.laggingWatchers, w)
				d.activeWatchers[w] = struct{}{}
				d.mu.Unlock()
			} else {
				// partial delivery –> remember where to resume
				d.mu.Lock()
				d.laggingWatchers[w] = w.lastRev + 1
				d.mu.Unlock()
			}
		}
	}
}
