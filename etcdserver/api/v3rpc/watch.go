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
	"reflect"

	"github.com/coreos/etcd/etcdserver"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/storage"
	"github.com/coreos/etcd/storage/storagepb"
)

type watchServer struct {
	clusterID int64
	memberID  int64
	raftTimer etcdserver.RaftTimer
	watchable storage.Watchable
}

func NewWatchServer(s *etcdserver.EtcdServer) pb.WatchServer {
	return &watchServer{
		clusterID: int64(s.Cluster().ID()),
		memberID:  int64(s.ID()),
		raftTimer: s,
		watchable: s.Watchable(),
	}
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
	clusterID int64
	memberID  int64
	raftTimer etcdserver.RaftTimer

	gRPCStream  pb.Watch_WatchServer
	watchStream storage.WatchStream
	ctrlStream  chan *pb.WatchResponse

	// closec indicates the stream is closed.
	closec chan struct{}
}

func (ws *watchServer) Watch(stream pb.Watch_WatchServer) error {
	sws := serverWatchStream{
		clusterID:   ws.clusterID,
		memberID:    ws.memberID,
		raftTimer:   ws.raftTimer,
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

		switch uv := req.RequestUnion.(type) {
		case *pb.WatchRequest_CreateRequest:
			if uv.CreateRequest == nil {
				break
			}

			creq := uv.CreateRequest
			toWatch := creq.Key
			isPrefix := len(creq.RangeEnd) != 0
			badPrefix := isPrefix && !reflect.DeepEqual(getPrefix(toWatch), creq.RangeEnd)

			rev := creq.StartRevision
			wsrev := sws.watchStream.Rev()
			futureRev := rev > wsrev
			if rev == 0 {
				// rev 0 watches past the current revision
				rev = wsrev + 1
			}
			// do not allow future watch revision
			// do not allow range that is not a prefix
			id := storage.WatchID(-1)
			if !futureRev && !badPrefix {
				id = sws.watchStream.Watch(toWatch, isPrefix, rev)
			}
			sws.ctrlStream <- &pb.WatchResponse{
				Header:   sws.newResponseHeader(wsrev),
				WatchId:  int64(id),
				Created:  true,
				Canceled: futureRev || badPrefix,
			}
		case *pb.WatchRequest_CancelRequest:
			if uv.CancelRequest != nil {
				id := uv.CancelRequest.WatchId
				err := sws.watchStream.Cancel(storage.WatchID(id))
				if err == nil {
					sws.ctrlStream <- &pb.WatchResponse{
						Header:   sws.newResponseHeader(sws.watchStream.Rev()),
						WatchId:  id,
						Canceled: true,
					}
				}
			}
			// TODO: do we need to return error back to client?
		default:
			panic("not implemented")
		}
	}
}

func (sws *serverWatchStream) sendLoop() {
	// watch ids that are currently active
	ids := make(map[storage.WatchID]struct{})
	// watch responses pending on a watch id creation message
	pending := make(map[storage.WatchID][]*pb.WatchResponse)

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

			wr := &pb.WatchResponse{
				Header:          sws.newResponseHeader(wresp.Revision),
				WatchId:         int64(wresp.WatchID),
				Events:          events,
				CompactRevision: wresp.CompactRevision,
			}

			if _, hasId := ids[wresp.WatchID]; !hasId {
				// buffer if id not yet announced
				wrs := append(pending[wresp.WatchID], wr)
				pending[wresp.WatchID] = wrs
				continue
			}

			storage.ReportEventReceived()
			if err := sws.gRPCStream.Send(wr); err != nil {
				return
			}

		case c, ok := <-sws.ctrlStream:
			if !ok {
				return
			}

			if err := sws.gRPCStream.Send(c); err != nil {
				return
			}

			// track id creation
			wid := storage.WatchID(c.WatchId)
			if c.Canceled {
				delete(ids, wid)
				continue
			}
			if c.Created {
				// flush buffered events
				ids[wid] = struct{}{}
				for _, v := range pending[wid] {
					storage.ReportEventReceived()
					if err := sws.gRPCStream.Send(v); err != nil {
						return
					}
				}
				delete(pending, wid)
			}
		case <-sws.closec:
			// drain the chan to clean up pending events
			for range sws.watchStream.Chan() {
				storage.ReportEventReceived()
			}
			for _, wrs := range pending {
				for range wrs {
					storage.ReportEventReceived()
				}
			}
		}
	}
}

func (sws *serverWatchStream) close() {
	sws.watchStream.Close()
	close(sws.closec)
	close(sws.ctrlStream)
}

func (sws *serverWatchStream) newResponseHeader(rev int64) *pb.ResponseHeader {
	return &pb.ResponseHeader{
		ClusterId: uint64(sws.clusterID),
		MemberId:  uint64(sws.memberID),
		Revision:  rev,
		RaftTerm:  sws.raftTimer.Term(),
	}
}

// TODO: remove getPrefix when storage supports full range watchers

func getPrefix(key []byte) []byte {
	end := make([]byte, len(key))
	copy(end, key)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xff {
			end[i] = end[i] + 1
			end = end[:i+1]
			return end
		}
	}
	// next prefix does not exist (e.g., 0xffff);
	// default to WithFromKey policy
	end = []byte{0}
	return end
}
