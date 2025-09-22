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
	"sync"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// watcher holds one client’s buffered stream of events.
type watcher struct {
	respCh     chan clientv3.WatchResponse
	cancelResp *clientv3.WatchResponse
	keyPred    KeyPredicate
	stopOnce   sync.Once
}

func newWatcher(bufSize int, pred KeyPredicate) *watcher {
	return &watcher{
		respCh:  make(chan clientv3.WatchResponse, bufSize),
		keyPred: pred,
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
	case w.respCh <- clientv3.WatchResponse{Events: eventBatch}:
		return true
	default:
		return false
	}
}

func (w *watcher) Compact(compactRev int64) {
	resp := &clientv3.WatchResponse{
		Canceled:        true,
		CompactRevision: compactRev,
		CancelReason:    rpctypes.ErrCompacted.Error(),
	}
	w.stopOnce.Do(func() {
		w.cancelResp = resp
		close(w.respCh)
	})
}

// Stop closes the event channel atomically.
func (w *watcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.respCh)
	})
}
