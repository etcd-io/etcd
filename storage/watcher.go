// Copyright 2015 CoreOS, Inc.
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

package storage

import (
	"errors"
	"sync"

	"github.com/coreos/etcd/storage/storagepb"
)

var (
	ErrWatcherNotExist = errors.New("storage: watcher does not exist")
)

type WatchStream interface {
	// Watch creates a watcher. The watcher watches the events happening or
	// happened on the given key or key prefix from the given startRev.
	//
	// The whole event history can be watched unless compacted.
	// If `prefix` is true, watch observes all events whose key prefix could be the given `key`.
	// If `startRev` <=0, watch observes events after currentRev.
	//
	// The returned `id` is the ID of this watcher. It appears as WatchID
	// in events that are sent to the created watcher through stream channel.
	//
	// TODO: remove the returned CancelFunc. Always use Cancel.
	Watch(key []byte, prefix bool, startRev int64) (id int64, cancel CancelFunc)

	// Chan returns a chan. All watched events will be sent to the returned chan.
	Chan() <-chan []storagepb.Event

	// Cancel cancels a watcher by giving its ID. If watcher does not exist, an error will be
	// returned.
	Cancel(id int64) error

	// Close closes the WatchChan and release all related resources.
	Close()
}

// watchStream contains a collection of watchers that share
// one streaming chan to send out watched events and other control events.
type watchStream struct {
	watchable watchable
	ch        chan []storagepb.Event

	mu sync.Mutex // guards fields below it
	// nextID is the ID pre-allocated for next new watcher in this stream
	nextID  int64
	closed  bool
	cancels map[int64]CancelFunc
}

// TODO: return error if ws is closed?
func (ws *watchStream) Watch(key []byte, prefix bool, startRev int64) (id int64, cancel CancelFunc) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.closed {
		return -1, nil
	}

	id = ws.nextID
	ws.nextID++

	_, c := ws.watchable.watch(key, prefix, startRev, id, ws.ch)

	ws.cancels[id] = c
	return id, c
}

func (ws *watchStream) Chan() <-chan []storagepb.Event {
	return ws.ch
}

func (ws *watchStream) Cancel(id int64) error {
	cancel, ok := ws.cancels[id]
	if !ok {
		return ErrWatcherNotExist
	}
	cancel()
	delete(ws.cancels, id)
	return nil
}

func (ws *watchStream) Close() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	for _, cancel := range ws.cancels {
		cancel()
	}
	ws.closed = true
	close(ws.ch)
	watchStreamGauge.Dec()
}
