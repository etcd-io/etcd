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
	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

type watcherSingle struct {
	// ch delievers events received from the etcd server
	ch clientv3.WatchChan
	// cancel is used to cancel the underlying etcd server watcher
	// It should also close the ch.
	cancel context.CancelFunc

	// sws is the stream this watcherSingle attached to
	sws *serverWatchStream

	w watcher

	rev         int64 // current revision
	lastSeenRev int64

	donec chan struct{}
}

func newWatcherSingle(wch clientv3.WatchChan, c context.CancelFunc, w watcher, sws *serverWatchStream) *watcherSingle {
	return &watcherSingle{
		sws: sws,

		ch:     wch,
		cancel: c,

		w:     w,
		donec: make(chan struct{}),
	}
}

func (ws watcherSingle) run() {
	defer close(ws.donec)

	for wr := range ws.ch {
		ws.rev = wr.Header.Revision
		ws.w.send(wr)
		ws.lastSeenRev = wr.Events[len(wr.Events)-1].Kv.ModRevision

		if ws.sws.maybeCoalesceWatcher(ws) {
			return
		}
	}
}

// canPromote returns true if a watcherSingle can promote itself to a watchergroup
// when it already caught up with the current revision.
func (ws watcherSingle) canPromote() bool {
	return ws.rev == ws.lastSeenRev
}

func (ws watcherSingle) stop() {
	ws.cancel()
	<-ws.donec
}
