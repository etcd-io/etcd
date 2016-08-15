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
	"io"
	"sync"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type watchProxy struct {
	cw  clientv3.Watcher
	wgs watchergroups

	mu           sync.Mutex
	nextStreamID int64
}

func NewWatchProxy(c *clientv3.Client) pb.WatchServer {
	return &watchProxy{
		cw: c.Watcher,
		wgs: watchergroups{
			cw:     c.Watcher,
			groups: make(map[watchRange]*watcherGroup),
		},
	}
}

func (wp *watchProxy) Watch(stream pb.Watch_WatchServer) (err error) {
	wp.mu.Lock()
	wp.nextStreamID++
	wp.mu.Unlock()

	sws := serverWatchStream{
		cw:      wp.cw,
		groups:  &wp.wgs,
		singles: make(map[int64]*watcherSingle),

		id:         wp.nextStreamID,
		gRPCStream: stream,

		ctrlCh:  make(chan *pb.WatchResponse, 10),
		watchCh: make(chan *pb.WatchResponse, 10),
	}

	go sws.recvLoop()

	sws.sendLoop()

	return nil
}

type serverWatchStream struct {
	id int64
	cw clientv3.Watcher

	mu      sync.Mutex // make sure any access of groups and singles is atomic
	groups  *watchergroups
	singles map[int64]*watcherSingle

	gRPCStream pb.Watch_WatchServer

	ctrlCh  chan *pb.WatchResponse
	watchCh chan *pb.WatchResponse

	nextWatcherID int64
}

func (sws *serverWatchStream) close() {
	close(sws.watchCh)
	close(sws.ctrlCh)
	for _, ws := range sws.singles {
		ws.stop()
	}
	sws.groups.stop()
}

func (sws *serverWatchStream) recvLoop() error {
	defer sws.close()

	for {
		req, err := sws.gRPCStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch uv := req.RequestUnion.(type) {
		case *pb.WatchRequest_CreateRequest:
			cr := uv.CreateRequest

			watcher := watcher{
				wr: watchRange{
					key: string(cr.Key),
					end: string(cr.RangeEnd),
				},
				id: sws.nextWatcherID,
				ch: sws.watchCh,

				progress: cr.ProgressNotify,
				filters:  v3rpc.FiltersFromRequest(cr),
			}
			if cr.StartRevision != 0 {
				sws.addDedicatedWatcher(watcher, cr.StartRevision)
			} else {
				sws.addCoalescedWatcher(watcher)
			}
			sws.nextWatcherID++

		case *pb.WatchRequest_CancelRequest:
			sws.removeWatcher(uv.CancelRequest.WatchId)
		default:
			panic("not implemented")
		}
	}
}

func (sws *serverWatchStream) sendLoop() {
	for {
		select {
		case wresp, ok := <-sws.watchCh:
			if !ok {
				return
			}
			if err := sws.gRPCStream.Send(wresp); err != nil {
				return
			}

		case c, ok := <-sws.ctrlCh:
			if !ok {
				return
			}
			if err := sws.gRPCStream.Send(c); err != nil {
				return
			}
		}
	}
}

func (sws *serverWatchStream) addCoalescedWatcher(w watcher) {
	sws.mu.Lock()
	defer sws.mu.Unlock()

	rid := receiverID{streamID: sws.id, watcherID: w.id}
	sws.groups.addWatcher(rid, w)
}

func (sws *serverWatchStream) addDedicatedWatcher(w watcher, rev int64) {
	sws.mu.Lock()
	defer sws.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	wch := sws.cw.Watch(ctx,
		w.wr.key, clientv3.WithRange(w.wr.end),
		clientv3.WithRev(rev),
		clientv3.WithProgressNotify(),
		clientv3.WithCreatedNotify(),
	)

	ws := newWatcherSingle(wch, cancel, w, sws)
	sws.singles[w.id] = ws
	go ws.run()
}

func (sws *serverWatchStream) maybeCoalesceWatcher(ws watcherSingle) bool {
	sws.mu.Lock()
	defer sws.mu.Unlock()

	rid := receiverID{streamID: sws.id, watcherID: ws.w.id}
	if sws.groups.maybeJoinWatcherSingle(rid, ws) {
		delete(sws.singles, ws.w.id)
		return true
	}
	return false
}

func (sws *serverWatchStream) removeWatcher(id int64) {
	sws.mu.Lock()
	defer sws.mu.Unlock()

	if sws.groups.removeWatcher(receiverID{streamID: sws.id, watcherID: id}) {
		return
	}

	if ws, ok := sws.singles[id]; ok {
		delete(sws.singles, id)
		ws.stop()
	}
}
