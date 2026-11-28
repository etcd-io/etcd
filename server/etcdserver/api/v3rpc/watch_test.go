// Copyright 2018 The etcd Authors
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
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/apply"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func TestSendFragment(t *testing.T) {
	tt := []struct {
		wr              *pb.WatchResponse
		maxRequestBytes uint
		fragments       int
		werr            error
	}{
		{ // large limit should not fragment
			wr:              createResponse(100, 1),
			maxRequestBytes: math.MaxInt32,
			fragments:       1,
		},
		{ // large limit for two messages, expect no fragment
			wr:              createResponse(10, 2),
			maxRequestBytes: 50,
			fragments:       1,
		},
		{ // limit is small but only one message, expect no fragment
			wr:              createResponse(1024, 1),
			maxRequestBytes: 1,
			fragments:       1,
		},
		{ // exceed limit only when combined, expect fragments
			wr:              createResponse(11, 5),
			maxRequestBytes: 20,
			fragments:       5,
		},
		{ // 5 events with each event exceeding limits, expect fragments
			wr:              createResponse(15, 5),
			maxRequestBytes: 10,
			fragments:       5,
		},
		{ // 4 events with some combined events exceeding limits
			wr:              createResponse(10, 4),
			maxRequestBytes: 35,
			fragments:       2,
		},
	}

	for i := range tt {
		fragmentedResp := make([]*pb.WatchResponse, 0)
		testSend := func(wr *pb.WatchResponse) error {
			fragmentedResp = append(fragmentedResp, wr)
			return nil
		}
		err := sendFragments(tt[i].wr, tt[i].maxRequestBytes, testSend)
		if !errors.Is(err, tt[i].werr) {
			t.Errorf("#%d: expected error %v, got %v", i, tt[i].werr, err)
		}
		got := len(fragmentedResp)
		if got != tt[i].fragments {
			t.Errorf("#%d: expected response number %d, got %d", i, tt[i].fragments, got)
		}
		if got > 0 && fragmentedResp[got-1].Fragment {
			t.Errorf("#%d: expected fragment=false in last response, got %+v", i, fragmentedResp[got-1])
		}
	}
}

func TestWatchResponseProtoFieldCount(t *testing.T) {
	const expectedWatchResponseProtoFields = 8

	fields := 0
	typ := reflect.TypeOf(pb.WatchResponse{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Tag.Get("protobuf") != "" {
			fields++
		}
	}

	// NOTE:
	//
	// We do manually value-copy in sendFragments. If there is new
	// protobuf field added to WatchResponse, we need to update sendFragments.
	if fields != expectedWatchResponseProtoFields {
		t.Fatalf("unexpected pb.WatchResponse protobuf field count, got=%d expected=%d", fields, expectedWatchResponseProtoFields)
	}
}

func createResponse(dataSize, events int) (resp *pb.WatchResponse) {
	resp = &pb.WatchResponse{Events: make([]*mvccpb.Event, events)}
	for i := range resp.Events {
		resp.Events[i] = &mvccpb.Event{
			Kv: &mvccpb.KeyValue{
				Key: bytes.Repeat([]byte("a"), dataSize),
			},
		}
	}
	return resp
}

type pendingWatchRPCStream struct {
	pb.Watch_WatchServer
	responses chan *pb.WatchResponse
}

func (s *pendingWatchRPCStream) Send(response *pb.WatchResponse) error {
	s.responses <- response
	return nil
}

type pendingMVCCWatchStream struct {
	mvcc.WatchStream
	responses chan mvcc.WatchResponse
}

func (s *pendingMVCCWatchStream) Chan() <-chan mvcc.WatchResponse { return s.responses }

type pendingWatchableKV struct {
	mvcc.WatchableKV
	previous *mvccpb.KeyValue
}

func (w *pendingWatchableKV) Range(_ context.Context, _, _ []byte, _ mvcc.RangeOptions) (*mvcc.RangeResult, error) {
	return &mvcc.RangeResult{KVs: []*mvccpb.KeyValue{w.previous}}, nil
}

type pendingWatchStatus struct {
	apply.RaftStatusGetter
	termStarted  chan struct{}
	continueTerm chan struct{}
}

func (s *pendingWatchStatus) Term() uint64 {
	close(s.termStarted)
	<-s.continueTerm
	return 1
}

func (s *pendingWatchStatus) Leader() types.ID { return 1 }

func TestWatchPendingResponsePreservesOptions(t *testing.T) {
	const id = mvcc.WatchID(1)
	t.Run("prev_kv", func(t *testing.T) {
		previous := &mvccpb.KeyValue{Key: []byte("key"), Value: []byte("old"), ModRevision: 1}
		event := &mvccpb.Event{
			Type: mvccpb.Event_PUT,
			Kv:   &mvccpb.KeyValue{Key: []byte("key"), Value: []byte("new"), CreateRevision: 1, ModRevision: 2},
		}
		sws, status, rpcStream := newPendingWatchStream(t, id, 1024, &pendingWatchableKV{previous: previous}, []*mvccpb.Event{event})

		sws.mu.Lock()
		sws.prevKV[id] = true
		sws.mu.Unlock()
		close(status.continueTerm)
		sws.ctrlStream <- &pb.WatchResponse{WatchId: int64(id), Created: true}

		_ = receivePendingWatchResponse(t, rpcStream)
		response := receivePendingWatchResponse(t, rpcStream)
		if len(response.Events) != 1 || !reflect.DeepEqual(response.Events[0].PrevKv, previous) {
			t.Fatalf("watch replay did not include requested PrevKv: %+v", response)
		}
	})

	t.Run("fragment", func(t *testing.T) {
		events := []*mvccpb.Event{
			{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("key1"), Value: []byte("new1"), CreateRevision: 1, ModRevision: 2}},
			{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("key2"), Value: []byte("new2"), CreateRevision: 1, ModRevision: 2}},
		}
		sws, status, rpcStream := newPendingWatchStream(t, id, 1, &pendingWatchableKV{}, events)

		sws.mu.Lock()
		sws.fragment[id] = true
		sws.mu.Unlock()
		close(status.continueTerm)
		sws.ctrlStream <- &pb.WatchResponse{WatchId: int64(id), Created: true}

		_ = receivePendingWatchResponse(t, rpcStream)
		first := receivePendingWatchResponse(t, rpcStream)
		second := receivePendingWatchResponse(t, rpcStream)
		if !first.Fragment || first.Fragment == second.Fragment || len(first.Events) != 1 || len(second.Events) != 1 {
			t.Fatalf("watch replay did not preserve fragmentation: first=%+v second=%+v", first, second)
		}
	})
}

func newPendingWatchStream(t *testing.T, id mvcc.WatchID, maxRequestBytes uint, watchable *pendingWatchableKV, events []*mvccpb.Event) (*serverWatchStream, *pendingWatchStatus, *pendingWatchRPCStream) {
	t.Helper()

	watchStream := &pendingMVCCWatchStream{responses: make(chan mvcc.WatchResponse, 1)}
	status := &pendingWatchStatus{termStarted: make(chan struct{}), continueTerm: make(chan struct{})}
	rpcStream := &pendingWatchRPCStream{responses: make(chan *pb.WatchResponse, 8)}
	sws := &serverWatchStream{
		maxRequestBytes: maxRequestBytes,
		sg:              status,
		watchable:       watchable,
		gRPCStream:      rpcStream,
		watchStream:     watchStream,
		ctrlStream:      make(chan *pb.WatchResponse, 1),
		progress:        make(map[mvcc.WatchID]bool),
		prevKV:          make(map[mvcc.WatchID]bool),
		fragment:        make(map[mvcc.WatchID]bool),
		closec:          make(chan struct{}),
	}
	done := make(chan struct{})
	go func() {
		sws.sendLoop()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-status.continueTerm:
		default:
			close(status.continueTerm)
		}
		close(sws.closec)
		close(watchStream.responses)
		<-done
	})
	watchStream.responses <- mvcc.WatchResponse{WatchID: id, Revision: 2, Events: events}
	select {
	case <-status.termStarted:
	case <-time.After(time.Second):
		t.Fatal("watch event was not processed")
	}
	return sws, status, rpcStream
}

func receivePendingWatchResponse(t *testing.T, stream *pendingWatchRPCStream) *pb.WatchResponse {
	t.Helper()
	select {
	case response := <-stream.responses:
		return response
	case <-time.After(time.Second):
		t.Fatal("watch response was not sent")
		return nil
	}
}
