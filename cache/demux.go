package cache

import (
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type watcher struct {
	ch         chan *clientv3.Event
	pred       func([]byte) bool
	lastSent   int64
	missedSend int
	closed     bool
}

type demux struct {
	mu       sync.RWMutex
	watchers map[*watcher]struct{}
}

func newDemux() *demux { return &demux{watchers: make(map[*watcher]struct{})} }

func (dx *demux) add(w *watcher) {
	dx.mu.Lock()
	dx.watchers[w] = struct{}{}
	dx.mu.Unlock()
}

func (dx *demux) remove(w *watcher) {
	dx.mu.Lock()
	delete(dx.watchers, w)
	close(w.ch)
	dx.mu.Unlock()
}

func (dx *demux) broadcast(ev *clientv3.Event, dropThreshold int) {
	dx.mu.RLock()
	for w := range dx.watchers {
		if w.closed {
			continue
		}
		if w.pred != nil && !w.pred(ev.Kv.Key) {
			continue
		}
		if ev.Kv.ModRevision <= w.lastSent {
			continue
		}
		select {
		case w.ch <- ev:
			w.lastSent = ev.Kv.ModRevision
			w.missedSend = 0
		default:
			w.missedSend++
			if w.missedSend >= dropThreshold {
				w.closed = true
				go dx.remove(w)
			}
		}
	}
	dx.mu.RUnlock()
}
