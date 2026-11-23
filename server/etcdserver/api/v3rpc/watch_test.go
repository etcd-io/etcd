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
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

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

type blockingRaftStatusGetter struct {
	apply.RaftStatusGetter
	once         sync.Once
	enteredTerm  chan struct{}
	continueTerm chan struct{}
}

func (g *blockingRaftStatusGetter) Term() uint64 {
	g.once.Do(func() {
		close(g.enteredTerm)
		<-g.continueTerm
	})
	return 1
}

func (g *blockingRaftStatusGetter) Leader() types.ID {
	return 1
}

type stubWatchStream struct {
	mvcc.WatchStream
	ch        chan mvcc.WatchResponse
	closeOnce sync.Once
}

func (s *stubWatchStream) Chan() <-chan mvcc.WatchResponse { return s.ch }
func (s *stubWatchStream) Close()                          { s.closeOnce.Do(func() { close(s.ch) }) }

type stubWatchableKV struct {
	mvcc.WatchableKV
	previous *mvccpb.KeyValue
}

func (kv *stubWatchableKV) Range(_ context.Context, _, _ []byte, _ mvcc.RangeOptions) (*mvcc.RangeResult, error) {
	if kv.previous == nil {
		return &mvcc.RangeResult{}, nil
	}
	return &mvcc.RangeResult{KVs: []*mvccpb.KeyValue{kv.previous}}, nil
}

type stubWatchGRPCServer struct {
	pb.Watch_WatchServer
	ctx  context.Context
	sent chan *pb.WatchResponse
}

func (s *stubWatchGRPCServer) Context() context.Context { return s.ctx }
func (s *stubWatchGRPCServer) Send(resp *pb.WatchResponse) error {
	s.sent <- resp
	return nil
}

func newPendingWatchStream(
	t *testing.T,
	id mvcc.WatchID,
	maxRequestBytes uint,
	kv mvcc.WatchableKV,
	events []*mvccpb.Event,
) (*serverWatchStream, *blockingRaftStatusGetter, *stubWatchGRPCServer) {
	t.Helper()

	status := &blockingRaftStatusGetter{
		enteredTerm:  make(chan struct{}),
		continueTerm: make(chan struct{}),
	}
	ws := &stubWatchStream{ch: make(chan mvcc.WatchResponse, 1)}
	rpcStream := &stubWatchGRPCServer{
		ctx:  t.Context(),
		sent: make(chan *pb.WatchResponse, 16),
	}
	sws := &serverWatchStream{
		lg:              zap.NewNop(),
		maxRequestBytes: maxRequestBytes,
		sg:              status,
		watchable:       kv,
		gRPCStream:      rpcStream,
		watchStream:     ws,
		ctrlStream:      make(chan *pb.WatchResponse, ctrlStreamBufLen),
		progress:        make(map[mvcc.WatchID]bool),
		prevKV:          make(map[mvcc.WatchID]bool),
		fragment:        make(map[mvcc.WatchID]bool),
		closec:          make(chan struct{}),
	}

	sws.wg.Add(1)
	go func() {
		defer sws.wg.Done()
		sws.sendLoop()
	}()
	t.Cleanup(func() {
		sws.close()
	})

	ws.ch <- mvcc.WatchResponse{
		WatchID:  id,
		Events:   events,
		Revision: 2,
	}

	select {
	case <-status.enteredTerm:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for sendLoop to receive replay event")
	}

	return sws, status, rpcStream
}

func receivePendingWatchResponse(t *testing.T, rpcStream *stubWatchGRPCServer) *pb.WatchResponse {
	t.Helper()
	select {
	case resp := <-rpcStream.sent:
		return resp
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watch response")
		return nil
	}
}

func TestWatchPendingResponsePreservesOptions(t *testing.T) {
	const id mvcc.WatchID = 7

	t.Run("prev_kv", func(t *testing.T) {
		previous := &mvccpb.KeyValue{Key: []byte("key"), Value: []byte("old"), ModRevision: 1}
		updateEvent := &mvccpb.Event{
			Type: mvccpb.Event_PUT,
			Kv:   &mvccpb.KeyValue{Key: []byte("key"), Value: []byte("new"), CreateRevision: 1, ModRevision: 2},
		}
		createEvent := &mvccpb.Event{
			Type: mvccpb.Event_PUT,
			Kv:   &mvccpb.KeyValue{Key: []byte("fresh"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2},
		}
		sws, status, rpcStream := newPendingWatchStream(
			t,
			id,
			1024,
			&stubWatchableKV{previous: previous},
			[]*mvccpb.Event{updateEvent, createEvent},
		)

		sws.mu.Lock()
		sws.prevKV[id] = true
		sws.mu.Unlock()
		close(status.continueTerm)
		sws.ctrlStream <- &pb.WatchResponse{WatchId: int64(id), Created: true}

		created := receivePendingWatchResponse(t, rpcStream)
		if !created.Created || created.WatchId != int64(id) {
			t.Fatalf("expected created response for watch %d, got %+v", id, created)
		}

		resp := receivePendingWatchResponse(t, rpcStream)
		if len(resp.Events) != 2 {
			t.Fatalf("expected 2 replayed events, got %d", len(resp.Events))
		}
		if !reflect.DeepEqual(resp.Events[0].PrevKv, previous) {
			t.Fatalf("expected PrevKv %+v on update event, got %+v", previous, resp.Events[0].PrevKv)
		}
		if resp.Events[1].PrevKv != nil {
			t.Fatalf("expected nil PrevKv on create event, got %+v", resp.Events[1].PrevKv)
		}
	})

	t.Run("fragment", func(t *testing.T) {
		events := createResponse(20, 4).Events
		sws, status, rpcStream := newPendingWatchStream(t, id, 45, &stubWatchableKV{}, events)

		sws.mu.Lock()
		sws.fragment[id] = true
		sws.mu.Unlock()
		close(status.continueTerm)
		sws.ctrlStream <- &pb.WatchResponse{WatchId: int64(id), Created: true}

		created := receivePendingWatchResponse(t, rpcStream)
		if !created.Created || created.WatchId != int64(id) {
			t.Fatalf("expected created response for watch %d, got %+v", id, created)
		}

		var fragments []*pb.WatchResponse
		var totalEvents int
		for {
			resp := receivePendingWatchResponse(t, rpcStream)
			fragments = append(fragments, resp)
			totalEvents += len(resp.Events)
			if !resp.Fragment {
				break
			}
		}
		if len(fragments) < 2 {
			t.Fatalf("expected pending response to be fragmented into multiple responses, got %d", len(fragments))
		}
		if totalEvents != len(events) {
			t.Fatalf("expected %d total events across fragments, got %d", len(events), totalEvents)
		}
	})

	t.Run("progress", func(t *testing.T) {
		events := createResponse(8, 1).Events
		sws, status, rpcStream := newPendingWatchStream(t, id, 1024, &stubWatchableKV{}, events)

		sws.mu.Lock()
		sws.progress[id] = true
		sws.mu.Unlock()
		close(status.continueTerm)
		sws.ctrlStream <- &pb.WatchResponse{WatchId: int64(id), Created: true}
		// Send a second control message as a happens-before barrier ensuring the
		// pending flush (and its progress update) has completed before inspecting.
		sws.ctrlStream <- &pb.WatchResponse{WatchId: int64(id + 1), Created: true}

		_ = receivePendingWatchResponse(t, rpcStream)
		_ = receivePendingWatchResponse(t, rpcStream)
		_ = receivePendingWatchResponse(t, rpcStream)

		sws.mu.RLock()
		stillProgress := sws.progress[id]
		sws.mu.RUnlock()
		if stillProgress {
			t.Fatal("expected pending key event flush to elide next progress notification")
		}
	})
}
