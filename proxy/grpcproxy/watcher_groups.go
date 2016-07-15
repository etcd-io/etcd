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

	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

type watchergroups struct {
	c *clientv3.Client

	mu        sync.Mutex
	groups    map[watchRange]*watcherGroup
	idToGroup map[receiverID]*watcherGroup
}

func (wgs *watchergroups) addWatcher(rid receiverID, w watcher) {
	wgs.mu.Lock()
	defer wgs.mu.Unlock()

	groups := wgs.groups

	if wg, ok := groups[w.wr]; ok {
		wg.add(rid, w)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	wch := wgs.c.Watch(ctx, w.wr.key, clientv3.WithRange(w.wr.end), clientv3.WithProgressNotify())
	watchg := newWatchergroup(wch, cancel)
	watchg.add(rid, w)
	go watchg.run()
	groups[w.wr] = watchg
}

func (wgs *watchergroups) removeWatcher(rid receiverID) bool {
	wgs.mu.Lock()
	defer wgs.mu.Unlock()

	if g, ok := wgs.idToGroup[rid]; ok {
		g.delete(rid)
		if g.isEmpty() {
			g.stop()
		}
		return true
	}
	return false
}

func (wgs *watchergroups) maybeJoinWatcherSingle(rid receiverID, ws watcherSingle) bool {
	wgs.mu.Lock()
	defer wgs.mu.Unlock()

	gropu, ok := wgs.groups[ws.w.wr]
	if ok {
		if ws.rev >= gropu.rev {
			gropu.add(receiverID{streamID: ws.sws.id, watcherID: ws.w.id}, ws.w)
			return true
		}
		return false
	}

	if ws.canPromote() {
		wg := newWatchergroup(ws.ch, ws.cancel)
		wgs.groups[ws.w.wr] = wg
		wg.add(receiverID{streamID: ws.sws.id, watcherID: ws.w.id}, ws.w)
		go wg.run()
		return true
	}

	return false
}
