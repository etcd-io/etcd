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

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
)

type ctxCancelMap map[context.Context]context.CancelFunc

// leaderNotifier creates a watcher and cancels all attached contexts on
// leader loss.
type leaderNotifier struct {
	cw clientv3.Watcher

	ctx    context.Context
	cancel context.CancelFunc
	donec  chan struct{}

	// readyc signals to the run loop when a new context is added.
	readyc chan struct{}

	// mu protects updates to cancels.
	mu sync.RWMutex

	// cancels tracks all contexts that cancel if the leader is lost.
	cancels ctxCancelMap
}

func newLeaderNotifier(c *clientv3.Client) *leaderNotifier {
	ctx, cancel := context.WithCancel(c.Ctx())
	lctx := clientv3.WithRequireLeader(ctx)
	ln := &leaderNotifier{
		cw:      c.Watcher,
		ctx:     lctx,
		cancel:  cancel,
		donec:   make(chan struct{}),
		readyc:  make(chan struct{}, 1),
		cancels: make(ctxCancelMap),
	}
	go ln.run()
	return ln
}

func (ln *leaderNotifier) add(ctx context.Context, cancel context.CancelFunc) {
	ln.mu.Lock()
	ln.cancels[ctx] = cancel
	ln.mu.Unlock()
	select {
	case ln.readyc <- struct{}{}:
	default:
	}
}

func (ln *leaderNotifier) delete(ctx context.Context) {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	delete(ln.cancels, ctx)
}

func (ln *leaderNotifier) close() {
	ln.cancel()
	<-ln.donec
}

func (ln *leaderNotifier) run() {
	defer close(ln.donec)
	for ln.ctx.Err() == nil {
		// only listen for lost leader if cancels are available
		for {
			ln.mu.RLock()
			n := len(ln.cancels)
			ln.mu.RUnlock()
			if n > 0 {
				break
			}
			select {
			case <-ln.ctx.Done():
				return
			case <-ln.readyc:
			}
		}
		// watch forever on a nonsense key at the end of time
		rev := int64((uint64(1) << 63) - 1)
		wch := ln.cw.Watch(ln.ctx, "__leaderNotifier", clientv3.WithRev(rev))
		for range wch {
		}
		// lost leader; cancel everything
		ln.mu.Lock()
		cancels := ln.cancels
		ln.cancels = make(ctxCancelMap)
		ln.mu.Unlock()
		for _, cancel := range cancels {
			cancel()
		}
	}
}
