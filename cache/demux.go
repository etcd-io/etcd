package cache

import (
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type watcher struct {
	eventChan     chan *clientv3.Event
	predicate     func([]byte) bool
	lastRev       int64
	missCount     int
	dropThreshold int
}

type demux struct {
	mu       sync.RWMutex
	watchers map[*watcher]struct{}
}

func newDemux() *demux {
	return &demux{watchers: make(map[*watcher]struct{})}
}

func (d *demux) add(watcher *watcher) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.watchers[watcher] = struct{}{}
}

func (d *demux) remove(watcher *watcher) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.watchers[watcher]; ok {
		delete(d.watchers, watcher)
		drain(watcher.eventChan)
		close(watcher.eventChan)
	}
}

func (d *demux) broadcast(event *clientv3.Event) {
	// take a snapshot of watchers to avoid holding write lock for long.
	d.mu.RLock()
	defer d.mu.RUnlock()
	watchers := make([]*watcher, 0, len(d.watchers))
	for w := range d.watchers {
		watchers = append(watchers, w)
	}

	var dropList []*watcher
	for _, w := range watchers {
		if w.predicate != nil && !w.predicate(event.Kv.Key) {
			continue
		}
		if event.Kv.ModRevision <= w.lastRev {
			continue
		}
		select {
		case w.eventChan <- event:
			w.lastRev = event.Kv.ModRevision
			w.missCount = 0
		default:
			w.missCount++
			if w.missCount >= w.dropThreshold {
				dropList = append(dropList, w)
			}
		}
	}
	// Remove slow watchers outside of main loop to avoid concurrent map write.
	for _, w := range dropList {
		d.remove(w)
	}
}

// purgeAll closes all watcher channels and clears them.
// Called when etcd compaction invalidates our cached history, so clients should resubscribe.
func (d *demux) purgeAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for w := range d.watchers {
		drain(w.eventChan)
		close(w.eventChan)
	}
	d.watchers = make(map[*watcher]struct{})
}

// drain empties ch so the very next receive gets ok == false.
func drain(ch chan *clientv3.Event) {
	for len(ch) > 0 {
		<-ch
	}
}
