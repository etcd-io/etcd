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
	"sync/atomic"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// watcher holds one client’s buffered stream of events.
type watcher struct {
	eventQueue chan []*clientv3.Event
	keyPred    KeyPredicate
	stopped    int32
	done       chan struct{} // closed together with Stop()
}

func newWatcher(bufSize int, pred KeyPredicate) *watcher {
	return &watcher{
		eventQueue: make(chan []*clientv3.Event, bufSize),
		keyPred:    pred,
		done:       make(chan struct{}),
	}
}

// true  -> events delivered (or filtered/duplicate)
// false -> buffer full (caller should mark watcher “lagging”)
func (w *watcher) enqueueEvent(eventBatch []*clientv3.Event) bool {
	if w.keyPred != nil {
		filtered := make([]*clientv3.Event, 0, len(eventBatch))
		for _, event := range eventBatch {
			if w.keyPred(event.Kv.Key) {
				filtered = append(filtered, event)
			}
		}
		if len(filtered) == 0 {
			return true
		}
		eventBatch = filtered
	}
	select {
	case w.eventQueue <- eventBatch:
		return true
	default:
		return false
	}
}

// Stop closes the event channel atomically.
func (w *watcher) Stop() {
	if atomic.CompareAndSwapInt32(&w.stopped, 0, 1) {
		close(w.eventQueue)
		close(w.done)
	}
}

func (w *watcher) Done() <-chan struct{} { return w.done }
