package cache

import (
	"context"
	"sync/atomic"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// watcher holds one client’s buffered stream of events.
type watcher struct {
	eventChan chan *clientv3.Event
	predicate func([]byte) bool
	lastRev   int64
	stopped   int32 // 0 = live, 1 = closed
}

func newWatcher(bufSize int, pred KeyPredicate, startRev int64) *watcher {
	return &watcher{
		eventChan: make(chan *clientv3.Event, bufSize),
		predicate: pred,
		lastRev:   startRev,
	}
}

// deliver applies the predicate and tries to enqueue event.
// true  -> event delivered or filtered/duplicate
// false -> buffer full (caller should mark watcher “lagging”)
func (w *watcher) enqueueEvent(event *clientv3.Event) bool {
	if w.predicate != nil && !w.predicate(event.Kv.Key) {
		return true
	}
	if event.Kv.ModRevision <= w.lastRev {
		return true
	}
	select {
	case w.eventChan <- event:
		w.lastRev = event.Kv.ModRevision
		return true
	default:
		return false
	}
}

func (w *watcher) stop() {
	if atomic.CompareAndSwapInt32(&w.stopped, 0, 1) {
		close(w.eventChan)
	}
}

func (w *watcher) autoRemoveOnCancel(ctx context.Context, d *demux) {
	go func() {
		<-ctx.Done()
		d.remove(w)
	}()
}
