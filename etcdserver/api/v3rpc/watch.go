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

const (
	// We send ctrl response inside the read loop. We do not want
	// send to block read, but we still want ctrl response we sent to
	// be serialized. Thus we use a buffered chan to solve the problem.
	// A small buffer should be OK for most cases, since we expect the
	// ctrl requests are infrequent.
	ctrlStreamBufLen = 16
)

// serverWatchStream is an etcd server side stream. It receives requests
// from client side gRPC stream. It receives watch events from storage.WatchStream,
// and creates responses that forwarded to gRPC stream.
// It also forwards control message like watch created and canceled.
type serverWatchStream struct {
	gRPCStream  pb.Watch_WatchServer
	watchStream storage.WatchStream
	ctrlStream  chan *pb.WatchResponse

	// closec indicates the stream is closed.
	closec chan struct{}
}

func (ws *watchServer) Watch(stream pb.Watch_WatchServer) error {
	sws := serverWatchStream{
		gRPCStream:  stream,
		watchStream: ws.watchable.NewWatchStream(),
		// chan for sending control response like watcher created and canceled.
		ctrlStream: make(chan *pb.WatchResponse, ctrlStreamBufLen),
		closec:     make(chan struct{}),
	}
	defer sws.close()

	go sws.sendLoop()
	return sws.recvLoop()
}

func (sws *serverWatchStream) recvLoop() error {
	for {
		req, err := sws.gRPCStream.Recv()
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
			id := sws.watchStream.Watch(toWatch, prefix, creq.StartRevision)
			sws.ctrlStream <- &pb.WatchResponse{
				// TODO: fill in response header.
				WatchId: id,
				Created: true,
			}
		case req.CancelRequest != nil:
			id := req.CancelRequest.WatchId
			err := sws.watchStream.Cancel(id)
			if err == nil {
				sws.ctrlStream <- &pb.WatchResponse{
					// TODO: fill in response header.
					WatchId:  id,
					Canceled: true,
				}
			}
			// TODO: do we need to return error back to client?
		default:
			panic("not implemented")
		}
	}
}

func (sws *serverWatchStream) sendLoop() {
	for {
		select {
		case wresp, ok := <-sws.watchStream.Chan():
			if !ok {
				return
			}

			// TODO: evs is []storagepb.Event type
			// either return []*storagepb.Event from storage package
			// or define protocol buffer with []storagepb.Event.
			evs := wresp.Events
			events := make([]*storagepb.Event, len(evs))
			for i := range evs {
				events[i] = &evs[i]
			}

			err := sws.gRPCStream.Send(&pb.WatchResponse{WatchId: wresp.WatchID, Events: events})
			storage.ReportEventReceived()
			if err != nil {
				return
			}

		case c, ok := <-sws.ctrlStream:
			if !ok {
				return
			}

			if err := sws.gRPCStream.Send(c); err != nil {
				return
			}

		case <-sws.closec:
			// drain the chan to clean up pending events
			for {
				_, ok := <-sws.watchStream.Chan()
				if !ok {
					return
				}
				storage.ReportEventReceived()
			}
		}
	}
}

func (sws *serverWatchStream) close() {
	sws.watchStream.Close()
	close(sws.closec)
	close(sws.ctrlStream)
}
