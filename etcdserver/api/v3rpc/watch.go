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

package v3rpc

import (
	"io"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/storage"
	"github.com/coreos/etcd/storage/storagepb"
)

type watchServer struct {
	watchable storage.Watchable
}

func NewWatchServer(w storage.Watchable) pb.WatchServer {
	return &watchServer{w}
}

func (ws *watchServer) Watch(stream pb.Watch_WatchServer) error {
	closec := make(chan struct{})
	defer close(closec)

	watcher := ws.watchable.NewWatcher()
	defer watcher.Close()

	go sendLoop(stream, watcher, closec)

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch {
		case req.CreateRequest != nil:
			creq := req.CreateRequest
			var prefix bool
			toWatch := creq.Key
			if len(creq.Key) == 0 {
				toWatch = creq.Prefix
				prefix = true
			}
			watcher.Watch(toWatch, prefix, creq.StartRevision)
		default:
			// TODO: support cancellation
			panic("not implemented")
		}
	}
}

func sendLoop(stream pb.Watch_WatchServer, watcher storage.Watcher, closec chan struct{}) {
	for {
		select {
		case evs, ok := <-watcher.Chan():
			if !ok {
				return
			}

			// TODO: evs is []storagepb.Event type
			// either return []*storagepb.Event from storage package
			// or define protocol buffer with []storagepb.Event.
			events := make([]*storagepb.Event, len(evs))
			for i := range evs {
				events[i] = &evs[i]
			}

			err := stream.Send(&pb.WatchResponse{Events: events})
			storage.ReportEventReceived()
			if err != nil {
				return
			}

		case <-closec:
			// drain the chan to clean up pending events
			for {
				_, ok := <-watcher.Chan()
				if !ok {
					return
				}
				storage.ReportEventReceived()
			}
		}
	}
}
