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

		var prefix bool
		toWatch := req.Key
		if len(req.Key) == 0 {
			toWatch = req.Prefix
			prefix = true
		}
		// TODO: support cancellation
		watcher.Watch(toWatch, prefix, req.StartRevision)
	}
}

func sendLoop(stream pb.Watch_WatchServer, watcher storage.Watcher, closec chan struct{}) {
	for {
		select {
		case e, ok := <-watcher.Chan():
			if !ok {
				return
			}
			err := stream.Send(&pb.WatchResponse{Event: &e})
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
