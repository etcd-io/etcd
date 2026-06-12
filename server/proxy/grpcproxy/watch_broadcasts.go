// Copyright 2016 The etcd Authors
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

package grpcproxy

import (
	"sync"
)

type watchBroadcasts struct {
	wp *watchProxy

	// mu protects bcasts and watchers from the coalesce loop.
	mu       sync.Mutex
	bcasts   map[*watchBroadcast]struct{}
	watchers map[*watcher]*watchBroadcast

	updatec chan *watchBroadcast
	donec   chan struct{}
}

// maxCoalesceRecievers prevents a popular watchBroadcast from being coalesced.
const maxCoalesceReceivers = 5

func newWatchBroadcasts(wp *watchProxy) *watchBroadcasts {
	wbs := &watchBroadcasts{
		wp:       wp,
		bcasts:   make(map[*watchBroadcast]struct{}),
		watchers: make(map[*watcher]*watchBroadcast),
		updatec:  make(chan *watchBroadcast, 1),
		donec:    make(chan struct{}),
	}
	go func() {
		defer close(wbs.donec)
		for wb := range wbs.updatec {
			wbs.coalesce(wb)
		}
	}()
	return wbs
}

func (wbs *watchBroadcasts) coalesce(wb *watchBroadcast) {
	if wb.size() >= maxCoalesceReceivers {
		return
	}
	wbs.mu.Lock()
	for wbswb := range wbs.bcasts {
		if wbswb == wb {
			continue
		}
		// Do not coalesce into a dead broadcast whose goroutine has exited.
		select {
		case <-wbswb.donec:
			continue
		default:
		}
		wb.mu.Lock()
		wbswb.mu.Lock()
		// 1. check if wbswb is behind wb so it won't skip any events in wb
		// 2. ensure wbswb started; nextrev == 0 may mean wbswb is waiting
		// for a current watcher and expects a create event from the server.
		if wb.nextrev >= wbswb.nextrev && wbswb.responses > 0 {
			for w := range wb.receivers {
				wbswb.receivers[w] = struct{}{}
				wbs.watchers[w] = wbswb
			}
			wb.receivers = nil
		}
		wbswb.mu.Unlock()
		wb.mu.Unlock()
		if wb.empty() {
			delete(wbs.bcasts, wb)
			wb.stop()
			break
		}
	}
	wbs.mu.Unlock()
}

func (wbs *watchBroadcasts) add(w *watcher) {
	wbs.mu.Lock()
	defer wbs.mu.Unlock()
	// Reassign any watchers orphaned by dead broadcasts before adding the
	// new one.  This ensures that watchers do not silently stop receiving
	// events after a backend watch connection is broken.
	wbs.pruneDeadBroadcasts()
	// find fitting bcast
	for wb := range wbs.bcasts {
		if wb.add(w) {
			wbs.watchers[w] = wb
			return
		}
	}
	// no fit; create a bcast
	wb := newWatchBroadcast(wbs.wp.lg, wbs.wp, w, wbs.update)
	wbs.watchers[w] = wb
	wbs.bcasts[wb] = struct{}{}
}

// pruneDeadBroadcasts removes broadcasts whose goroutines have exited and
// reassigns their watchers to live broadcasts (or new ones).  It must be
// called with wbs.mu held.
func (wbs *watchBroadcasts) pruneDeadBroadcasts() {
	if wbs.bcasts == nil {
		return
	}

	// First pass: collect dead broadcasts and their orphaned watchers.
	type deadEntry struct {
		wb      *watchBroadcast
		orphans []*watcher
	}
	var dead []deadEntry

	for wb := range wbs.bcasts {
		select {
		case <-wb.donec:
		default:
			continue
		}
		wb.mu.Lock()
		orphans := make([]*watcher, 0, len(wb.receivers))
		for w := range wb.receivers {
			orphans = append(orphans, w)
			delete(wbs.watchers, w)
		}
		wb.receivers = nil
		wb.mu.Unlock()
		dead = append(dead, deadEntry{wb: wb, orphans: orphans})
	}

	if len(dead) == 0 {
		return
	}

	// Second pass: remove dead broadcasts.
	for _, d := range dead {
		delete(wbs.bcasts, d.wb)
	}

	// Third pass: re-add each orphaned watcher to a live broadcast or a
	// freshly created one.
	for _, d := range dead {
		for _, w := range d.orphans {
			added := false
			for candidate := range wbs.bcasts {
				select {
				case <-candidate.donec:
					continue
				default:
				}
				if candidate.add(w) {
					wbs.watchers[w] = candidate
					added = true
					break
				}
			}
			if !added {
				newWb := newWatchBroadcast(wbs.wp.lg, wbs.wp, w, wbs.update)
				wbs.watchers[w] = newWb
				wbs.bcasts[newWb] = struct{}{}
			}
		}
	}
}

// delete removes a watcher and returns the number of remaining watchers.
func (wbs *watchBroadcasts) delete(w *watcher) int {
	wbs.mu.Lock()
	defer wbs.mu.Unlock()

	wb, ok := wbs.watchers[w]
	if !ok {
		panic("deleting missing watcher from broadcasts")
	}
	delete(wbs.watchers, w)
	wb.delete(w)
	if wb.empty() {
		delete(wbs.bcasts, wb)
		wb.stop()
	}
	return len(wbs.bcasts)
}

func (wbs *watchBroadcasts) stop() {
	wbs.mu.Lock()
	for wb := range wbs.bcasts {
		wb.stop()
	}
	wbs.bcasts = nil
	close(wbs.updatec)
	wbs.mu.Unlock()
	<-wbs.donec
}

func (wbs *watchBroadcasts) update(wb *watchBroadcast) {
	select {
	case wbs.updatec <- wb:
	default:
	}
}
