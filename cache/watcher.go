package cache

import (
	"sync/atomic"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// watcher holds one client’s buffered stream of events.
type watcher struct {
	eventQueue chan *clientv3.Event
	keyPred    KeyPredicate
	stopped    int32
}

func newWatcher(bufSize int, pred KeyPredicate) *watcher {
	return &watcher{
		eventQueue: make(chan *clientv3.Event, bufSize),
		keyPred:    pred,
	}
}

// true  -> event delivered (or filtered/duplicate)
// false -> buffer full (caller should mark watcher “lagging”)
func (w *watcher) enqueueEvent(event *clientv3.Event) bool {
	if w.keyPred != nil && !w.keyPred(event.Kv.Key) {
		return true
	}
	select {
	case w.eventQueue <- event:
		return true
	default:
		return false
	}
}

// Stop closes the event channel atomically.
func (w *watcher) Stop() {
	if atomic.CompareAndSwapInt32(&w.stopped, 0, 1) {
		close(w.eventQueue)
	}
}
