// Copyright 2016 CoreOS, Inc.
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
// limitations under the License.package recipe
package integration

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/storage/storagepb"
)

type clusterV3 struct {
	*cluster
	conns []*grpc.ClientConn
}

// newClusterGRPC returns a launched cluster with a grpc client connection
// for each cluster member.
func newClusterGRPC(t *testing.T, cfg *clusterConfig) *clusterV3 {
	cfg.useV3 = true
	cfg.useGRPC = true
	clus := &clusterV3{cluster: NewClusterByConfig(t, cfg)}
	for _, m := range clus.Members {
		conn, err := NewGRPCClient(m)
		if err != nil {
			t.Fatal(err)
		}
		clus.conns = append(clus.conns, conn)
	}
	clus.Launch(t)
	return clus
}

func (c *clusterV3) Terminate(t *testing.T) {
	for _, conn := range c.conns {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	}
	c.cluster.Terminate(t)
}

func (c *clusterV3) RandConn() *grpc.ClientConn {
	return c.conns[rand.Intn(len(c.conns))]
}

// TestV3PutOverwrite puts a key with the v3 api to a random cluster member,
// overwrites it, then checks that the change was applied.
func TestV3PutOverwrite(t *testing.T) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	kvc := pb.NewKVClient(clus.RandConn())
	key := []byte("foo")
	reqput := &pb.PutRequest{Key: key, Value: []byte("bar")}

	respput, err := kvc.Put(context.TODO(), reqput)
	if err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	// overwrite
	reqput.Value = []byte("baz")
	respput2, err := kvc.Put(context.TODO(), reqput)
	if err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}
	if respput2.Header.Revision <= respput.Header.Revision {
		t.Fatalf("expected newer revision on overwrite, got %v <= %v",
			respput2.Header.Revision, respput.Header.Revision)
	}

	reqrange := &pb.RangeRequest{Key: key}
	resprange, err := kvc.Range(context.TODO(), reqrange)
	if err != nil {
		t.Fatalf("couldn't get key (%v)", err)
	}
	if len(resprange.Kvs) != 1 {
		t.Fatalf("expected 1 key, got %v", len(resprange.Kvs))
	}

	kv := resprange.Kvs[0]
	if kv.ModRevision <= kv.CreateRevision {
		t.Errorf("expected modRev > createRev, got %d <= %d",
			kv.ModRevision, kv.CreateRevision)
	}
	if !reflect.DeepEqual(reqput.Value, kv.Value) {
		t.Errorf("expected value %v, got %v", reqput.Value, kv.Value)
	}
}

func TestV3TxnTooManyOps(t *testing.T) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	kvc := pb.NewKVClient(clus.RandConn())

	addCompareOps := func(txn *pb.TxnRequest) {
		txn.Compare = append(txn.Compare,
			&pb.Compare{
				Result: pb.Compare_GREATER,
				Target: pb.Compare_CREATE,
				Key:    []byte("bar"),
			})
	}
	addSuccessOps := func(txn *pb.TxnRequest) {
		txn.Success = append(txn.Success,
			&pb.RequestUnion{
				RequestPut: &pb.PutRequest{
					Key:   []byte("bar"),
					Value: []byte("bar"),
				},
			})
	}
	addFailureOps := func(txn *pb.TxnRequest) {
		txn.Failure = append(txn.Failure,
			&pb.RequestUnion{
				RequestPut: &pb.PutRequest{
					Key:   []byte("bar"),
					Value: []byte("bar"),
				},
			})
	}

	tests := []func(txn *pb.TxnRequest){
		addCompareOps,
		addSuccessOps,
		addFailureOps,
	}

	for i, tt := range tests {
		txn := &pb.TxnRequest{}
		for j := 0; j < v3rpc.MaxOpsPerTxn+1; j++ {
			tt(txn)
		}

		_, err := kvc.Txn(context.Background(), txn)
		if err != v3rpc.ErrTooManyOps {
			t.Errorf("#%d: err = %v, want %v", i, err, v3rpc.ErrTooManyOps)
		}
	}
}

// TestV3PutMissingLease ensures that a Put on a key with a bogus lease fails.
func TestV3PutMissingLease(t *testing.T) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	kvc := pb.NewKVClient(clus.RandConn())
	key := []byte("foo")
	preq := &pb.PutRequest{Key: key, Lease: 123456}
	tests := []func(){
		// put case
		func() {
			if presp, err := kvc.Put(context.TODO(), preq); err == nil {
				t.Errorf("succeeded put key. req: %v. resp: %v", preq, presp)
			}
		},
		// txn success case
		func() {
			txn := &pb.TxnRequest{}
			txn.Success = append(txn.Success, &pb.RequestUnion{RequestPut: preq})
			if tresp, err := kvc.Txn(context.TODO(), txn); err == nil {
				t.Errorf("succeeded txn success. req: %v. resp: %v", txn, tresp)
			}
		},
		// txn failure case
		func() {
			txn := &pb.TxnRequest{}
			txn.Failure = append(txn.Failure, &pb.RequestUnion{RequestPut: preq})
			cmp := &pb.Compare{
				Result: pb.Compare_GREATER,
				Target: pb.Compare_CREATE,
				Key:    []byte("bar"),
			}
			txn.Compare = append(txn.Compare, cmp)
			if tresp, err := kvc.Txn(context.TODO(), txn); err == nil {
				t.Errorf("succeeded txn failure. req: %v. resp: %v", txn, tresp)
			}
		},
		// ignore bad lease in failure on success txn
		func() {
			txn := &pb.TxnRequest{}
			rreq := &pb.RangeRequest{Key: []byte("bar")}
			txn.Success = append(txn.Success, &pb.RequestUnion{RequestRange: rreq})
			txn.Failure = append(txn.Failure, &pb.RequestUnion{RequestPut: preq})
			if tresp, err := kvc.Txn(context.TODO(), txn); err != nil {
				t.Errorf("failed good txn. req: %v. resp: %v", txn, tresp)
			}
		},
	}

	for i, f := range tests {
		f()
		// key shouldn't have been stored
		rreq := &pb.RangeRequest{Key: key}
		rresp, err := kvc.Range(context.TODO(), rreq)
		if err != nil {
			t.Errorf("#%d. could not rangereq (%v)", i, err)
		} else if len(rresp.Kvs) != 0 {
			t.Errorf("#%d. expected no keys, got %v", i, rresp)
		}
	}
}

// TestV3DeleteRange tests various edge cases in the DeleteRange API.
func TestV3DeleteRange(t *testing.T) {
	tests := []struct {
		keySet []string
		begin  string
		end    string

		wantSet [][]byte
	}{
		// delete middle
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/", "fop",
			[][]byte{[]byte("foo"), []byte("fop")},
		},
		// no delete
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/", "foo/",
			[][]byte{[]byte("foo"), []byte("foo/abc"), []byte("fop")},
		},
		// delete first
		{
			[]string{"foo", "foo/abc", "fop"},
			"fo", "fop",
			[][]byte{[]byte("fop")},
		},
		// delete tail
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/", "fos",
			[][]byte{[]byte("foo")},
		},
		// delete exact
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/abc", "",
			[][]byte{[]byte("foo"), []byte("fop")},
		},
		// delete none, [x,x)
		{
			[]string{"foo"},
			"foo", "foo",
			[][]byte{[]byte("foo")},
		},
	}

	for i, tt := range tests {
		clus := newClusterGRPC(t, &clusterConfig{size: 3})
		kvc := pb.NewKVClient(clus.RandConn())

		ks := tt.keySet
		for j := range ks {
			reqput := &pb.PutRequest{Key: []byte(ks[j]), Value: []byte{}}
			_, err := kvc.Put(context.TODO(), reqput)
			if err != nil {
				t.Fatalf("couldn't put key (%v)", err)
			}
		}

		dreq := &pb.DeleteRangeRequest{
			Key:      []byte(tt.begin),
			RangeEnd: []byte(tt.end)}
		dresp, err := kvc.DeleteRange(context.TODO(), dreq)
		if err != nil {
			t.Fatalf("couldn't delete range on test %d (%v)", i, err)
		}

		rreq := &pb.RangeRequest{Key: []byte{0x0}, RangeEnd: []byte{0xff}}
		rresp, err := kvc.Range(context.TODO(), rreq)
		if err != nil {
			t.Errorf("couldn't get range on test %v (%v)", i, err)
		}
		if dresp.Header.Revision != rresp.Header.Revision {
			t.Errorf("expected revision %v, got %v",
				dresp.Header.Revision, rresp.Header.Revision)
		}

		keys := [][]byte{}
		for j := range rresp.Kvs {
			keys = append(keys, rresp.Kvs[j].Key)
		}
		if reflect.DeepEqual(tt.wantSet, keys) == false {
			t.Errorf("expected %v on test %v, got %v", tt.wantSet, i, keys)
		}

		// can't defer because tcp ports will be in use
		clus.Terminate(t)
	}
}

// TestV3TxnInvaildRange tests txn
func TestV3TxnInvaildRange(t *testing.T) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	kvc := pb.NewKVClient(clus.RandConn())
	preq := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}

	for i := 0; i < 3; i++ {
		_, err := kvc.Put(context.Background(), preq)
		if err != nil {
			t.Fatalf("couldn't put key (%v)", err)
		}
	}

	_, err := kvc.Compact(context.Background(), &pb.CompactionRequest{Revision: 2})
	if err != nil {
		t.Fatalf("couldn't compact kv space (%v)", err)
	}

	// future rev
	txn := &pb.TxnRequest{}
	txn.Success = append(txn.Success, &pb.RequestUnion{RequestPut: preq})

	rreq := &pb.RangeRequest{Key: []byte("foo"), Revision: 100}
	txn.Success = append(txn.Success, &pb.RequestUnion{RequestRange: rreq})

	if _, err := kvc.Txn(context.TODO(), txn); err != v3rpc.ErrFutureRev {
		t.Errorf("err = %v, want %v", err, v3rpc.ErrFutureRev)
	}

	// compacted rev
	txn.Success[1].RequestRange.Revision = 1
	if _, err := kvc.Txn(context.TODO(), txn); err != v3rpc.ErrCompacted {
		t.Errorf("err = %v, want %v", err, v3rpc.ErrCompacted)
	}
}

// TestV3WatchFromCurrentRevision tests Watch APIs from current revision.
func TestV3WatchFromCurrentRevision(t *testing.T) {
	tests := []struct {
		putKeys      []string
		watchRequest *pb.WatchRequest

		wresps []*pb.WatchResponse
	}{
		// watch the key, matching
		{
			[]string{"foo"},
			&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Key: []byte("foo")}},

			[]*pb.WatchResponse{
				{
					Header:  &pb.ResponseHeader{Revision: 1},
					Created: true,
				},
				{
					Header:  &pb.ResponseHeader{Revision: 2},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
						},
					},
				},
			},
		},
		// watch the key, non-matching
		{
			[]string{"foo"},
			&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Key: []byte("helloworld")}},

			[]*pb.WatchResponse{
				{
					Header:  &pb.ResponseHeader{Revision: 1},
					Created: true,
				},
			},
		},
		// watch the prefix, matching
		{
			[]string{"fooLong"},
			&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Prefix: []byte("foo")}},

			[]*pb.WatchResponse{
				{
					Header:  &pb.ResponseHeader{Revision: 1},
					Created: true,
				},
				{
					Header:  &pb.ResponseHeader{Revision: 2},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("fooLong"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
						},
					},
				},
			},
		},
		// watch the prefix, non-matching
		{
			[]string{"foo"},
			&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Prefix: []byte("helloworld")}},

			[]*pb.WatchResponse{
				{
					Header:  &pb.ResponseHeader{Revision: 1},
					Created: true,
				},
			},
		},
		// multiple puts, one watcher with matching key
		{
			[]string{"foo", "foo", "foo"},
			&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Key: []byte("foo")}},

			[]*pb.WatchResponse{
				{
					Header:  &pb.ResponseHeader{Revision: 1},
					Created: true,
				},
				{
					Header:  &pb.ResponseHeader{Revision: 2},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
						},
					},
				},
				{
					Header:  &pb.ResponseHeader{Revision: 3},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 3, Version: 2},
						},
					},
				},
				{
					Header:  &pb.ResponseHeader{Revision: 4},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 4, Version: 3},
						},
					},
				},
			},
		},
		// multiple puts, one watcher with matching prefix
		{
			[]string{"foo", "foo", "foo"},
			&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Prefix: []byte("foo")}},

			[]*pb.WatchResponse{
				{
					Header:  &pb.ResponseHeader{Revision: 1},
					Created: true,
				},
				{
					Header:  &pb.ResponseHeader{Revision: 2},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
						},
					},
				},
				{
					Header:  &pb.ResponseHeader{Revision: 3},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 3, Version: 2},
						},
					},
				},
				{
					Header:  &pb.ResponseHeader{Revision: 4},
					Created: false,
					Events: []*storagepb.Event{
						{
							Type: storagepb.PUT,
							Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 4, Version: 3},
						},
					},
				},
			},
		},
	}

	for i, tt := range tests {
		clus := newClusterGRPC(t, &clusterConfig{size: 3})

		wAPI := pb.NewWatchClient(clus.RandConn())
		wStream, err := wAPI.Watch(context.TODO())
		if err != nil {
			t.Fatalf("#%d: wAPI.Watch error: %v", i, err)
		}

		if err := wStream.Send(tt.watchRequest); err != nil {
			t.Fatalf("#%d: wStream.Send error: %v", i, err)
		}

		go func() {
			for _, k := range tt.putKeys {
				kvc := pb.NewKVClient(clus.RandConn())
				req := &pb.PutRequest{Key: []byte(k), Value: []byte("bar")}
				if _, err := kvc.Put(context.TODO(), req); err != nil {
					t.Fatalf("#%d: couldn't put key (%v)", i, err)
				}
			}
		}()

		var createdWatchId int64
		for j, wresp := range tt.wresps {
			resp, err := wStream.Recv()
			if err != nil {
				t.Errorf("#%d.%d: wStream.Recv error: %v", i, j, err)
			}

			if resp.Header == nil {
				t.Fatalf("#%d.%d: unexpected nil resp.Header", i, j)
			}
			if resp.Header.Revision != wresp.Header.Revision {
				t.Errorf("#%d.%d: resp.Header.Revision got = %d, want = %d", i, j, resp.Header.Revision, wresp.Header.Revision)
			}

			if wresp.Created != resp.Created {
				t.Errorf("#%d.%d: resp.Created got = %v, want = %v", i, j, resp.Created, wresp.Created)
			}
			if resp.Created {
				createdWatchId = resp.WatchId
			}
			if resp.WatchId != createdWatchId {
				t.Errorf("#%d.%d: resp.WatchId got = %d, want = %d", i, j, resp.WatchId, createdWatchId)
			}

			if !reflect.DeepEqual(resp.Events, wresp.Events) {
				t.Errorf("#%d.%d: resp.Events got = %+v, want = %+v", i, j, resp.Events, wresp.Events)
			}
		}

		rok, nr := WaitResponse(wStream, 1*time.Second)
		if !rok {
			t.Errorf("unexpected pb.WatchResponse is received %+v", nr)
		}

		// can't defer because tcp ports will be in use
		clus.Terminate(t)
	}
}

// TestV3WatchCancelSynced tests Watch APIs cancellation from synced map.
func TestV3WatchCancelSynced(t *testing.T) {
	testV3WatchCancel(t, 0)
}

// TestV3WatchCancelUnsynced tests Watch APIs cancellation from unsynced map.
func TestV3WatchCancelUnsynced(t *testing.T) {
	testV3WatchCancel(t, 1)
}

func testV3WatchCancel(t *testing.T, startRev int64) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	wAPI := pb.NewWatchClient(clus.RandConn())

	wStream, errW := wAPI.Watch(context.TODO())
	if errW != nil {
		t.Fatalf("wAPI.Watch error: %v", errW)
	}

	if err := wStream.Send(&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: startRev}}); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}

	wresp, errR := wStream.Recv()
	if errR != nil {
		t.Errorf("wStream.Recv error: %v", errR)
	}
	if !wresp.Created {
		t.Errorf("wresp.Created got = %v, want = true", wresp.Created)
	}

	if err := wStream.Send(&pb.WatchRequest{CancelRequest: &pb.WatchCancelRequest{WatchId: wresp.WatchId}}); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}

	cresp, err := wStream.Recv()
	if err != nil {
		t.Errorf("wStream.Recv error: %v", err)
	}
	if !cresp.Canceled {
		t.Errorf("cresp.Canceled got = %v, want = true", cresp.Canceled)
	}

	kvc := pb.NewKVClient(clus.RandConn())
	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}); err != nil {
		t.Errorf("couldn't put key (%v)", err)
	}

	// watch got canceled, so this should block
	rok, nr := WaitResponse(wStream, 1*time.Second)
	if !rok {
		t.Errorf("unexpected pb.WatchResponse is received %+v", nr)
	}

	clus.Terminate(t)
}

func TestV3WatchMultipleWatchersSynced(t *testing.T) {
	testV3WatchMultipleWatchers(t, 0)
}

func TestV3WatchMultipleWatchersUnsynced(t *testing.T) {
	testV3WatchMultipleWatchers(t, 1)
}

// testV3WatchMultipleWatchers tests multiple watchers on the same key
// and one watcher with matching prefix. It first puts the key
// that matches all watchers, and another key that matches only
// one watcher to test if it receives expected events.
func testV3WatchMultipleWatchers(t *testing.T, startRev int64) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	wAPI := pb.NewWatchClient(clus.RandConn())
	kvc := pb.NewKVClient(clus.RandConn())

	wStream, errW := wAPI.Watch(context.TODO())
	if errW != nil {
		t.Fatalf("wAPI.Watch error: %v", errW)
	}

	watchKeyN := 4
	for i := 0; i < watchKeyN+1; i++ {
		var wreq *pb.WatchRequest
		if i < watchKeyN {
			wreq = &pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: startRev}}
		} else {
			wreq = &pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Prefix: []byte("fo"), StartRevision: startRev}}
		}
		if err := wStream.Send(wreq); err != nil {
			t.Fatalf("wStream.Send error: %v", err)
		}
	}

	ids := make(map[int64]struct{})
	for i := 0; i < watchKeyN+1; i++ {
		wresp, err := wStream.Recv()
		if err != nil {
			t.Fatalf("wStream.Recv error: %v", err)
		}
		if !wresp.Created {
			t.Fatalf("wresp.Created got = %v, want = true", wresp.Created)
		}
		ids[wresp.WatchId] = struct{}{}
	}

	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	for i := 0; i < watchKeyN+1; i++ {
		wresp, err := wStream.Recv()
		if err != nil {
			t.Fatalf("wStream.Recv error: %v", err)
		}
		if _, ok := ids[wresp.WatchId]; !ok {
			t.Errorf("watchId %d is not created!", wresp.WatchId)
		} else {
			delete(ids, wresp.WatchId)
		}
		if len(wresp.Events) == 0 {
			t.Errorf("#%d: no events received", i)
		}
		for _, ev := range wresp.Events {
			if string(ev.Kv.Key) != "foo" {
				t.Errorf("ev.Kv.Key got = %s, want = foo", ev.Kv.Key)
			}
			if string(ev.Kv.Value) != "bar" {
				t.Errorf("ev.Kv.Value got = %s, want = bar", ev.Kv.Value)
			}
		}
	}

	// now put one key that has only one matching watcher
	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("fo"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}
	wresp, err := wStream.Recv()
	if err != nil {
		t.Errorf("wStream.Recv error: %v", err)
	}
	if len(wresp.Events) != 1 {
		t.Fatalf("len(wresp.Events) got = %d, want = 1", len(wresp.Events))
	}
	if string(wresp.Events[0].Kv.Key) != "fo" {
		t.Errorf("wresp.Events[0].Kv.Key got = %s, want = fo", wresp.Events[0].Kv.Key)
	}

	// now Recv should block because there is no more events coming
	rok, nr := WaitResponse(wStream, 1*time.Second)
	if !rok {
		t.Errorf("unexpected pb.WatchResponse is received %+v", nr)
	}

	clus.Terminate(t)
}

func TestV3WatchMultipleEventsTxnSynced(t *testing.T) {
	testV3WatchMultipleEventsTxn(t, 0)
}

func TestV3WatchMultipleEventsTxnUnsynced(t *testing.T) {
	testV3WatchMultipleEventsTxn(t, 1)
}

// testV3WatchMultipleEventsTxn tests Watch APIs when it receives multiple events.
func testV3WatchMultipleEventsTxn(t *testing.T, startRev int64) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})

	wAPI := pb.NewWatchClient(clus.RandConn())
	wStream, wErr := wAPI.Watch(context.TODO())
	if wErr != nil {
		t.Fatalf("wAPI.Watch error: %v", wErr)
	}

	if err := wStream.Send(&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Prefix: []byte("foo"), StartRevision: startRev}}); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}

	kvc := pb.NewKVClient(clus.RandConn())
	txn := pb.TxnRequest{}
	for i := 0; i < 3; i++ {
		ru := &pb.RequestUnion{}
		ru.RequestPut = &pb.PutRequest{Key: []byte(fmt.Sprintf("foo%d", i)), Value: []byte("bar")}
		txn.Success = append(txn.Success, ru)
	}

	tresp, err := kvc.Txn(context.Background(), &txn)
	if err != nil {
		t.Fatalf("kvc.Txn error: %v", err)
	}
	if !tresp.Succeeded {
		t.Fatalf("kvc.Txn failed: %+v", tresp)
	}

	events := []*storagepb.Event{}
	for len(events) < 3 {
		resp, err := wStream.Recv()
		if err != nil {
			t.Errorf("wStream.Recv error: %v", err)
		}
		if resp.Created {
			continue
		}
		events = append(events, resp.Events...)
	}
	sort.Sort(eventsSortByKey(events))

	wevents := []*storagepb.Event{
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo0"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo1"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo2"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
		},
	}

	if !reflect.DeepEqual(events, wevents) {
		t.Errorf("events got = %+v, want = %+v", events, wevents)
	}

	rok, nr := WaitResponse(wStream, 1*time.Second)
	if !rok {
		t.Errorf("unexpected pb.WatchResponse is received %+v", nr)
	}

	// can't defer because tcp ports will be in use
	clus.Terminate(t)
}

type eventsSortByKey []*storagepb.Event

func (evs eventsSortByKey) Len() int           { return len(evs) }
func (evs eventsSortByKey) Swap(i, j int)      { evs[i], evs[j] = evs[j], evs[i] }
func (evs eventsSortByKey) Less(i, j int) bool { return bytes.Compare(evs[i].Kv.Key, evs[j].Kv.Key) < 0 }

func TestV3WatchMultipleEventsPutUnsynced(t *testing.T) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	kvc := pb.NewKVClient(clus.RandConn())

	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo0"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}
	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo1"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	wAPI := pb.NewWatchClient(clus.RandConn())
	wStream, wErr := wAPI.Watch(context.TODO())
	if wErr != nil {
		t.Fatalf("wAPI.Watch error: %v", wErr)
	}

	if err := wStream.Send(&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Prefix: []byte("foo"), StartRevision: 1}}); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}

	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo0"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}
	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo1"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	allWevents := []*storagepb.Event{
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo0"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo1"), Value: []byte("bar"), CreateRevision: 3, ModRevision: 3, Version: 1},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo0"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 4, Version: 2},
		},
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo1"), Value: []byte("bar"), CreateRevision: 3, ModRevision: 5, Version: 2},
		},
	}

	events := []*storagepb.Event{}
	for len(events) < 4 {
		resp, err := wStream.Recv()
		if err != nil {
			t.Errorf("wStream.Recv error: %v", err)
		}
		if resp.Created {
			continue
		}
		events = append(events, resp.Events...)
		// if PUT requests are committed by now, first receive would return
		// multiple events, but if not, it returns a single event. In SSD,
		// it should return 4 events at once.
	}

	if !reflect.DeepEqual(events, allWevents) {
		t.Errorf("events got = %+v, want = %+v", events, allWevents)
	}

	rok, nr := WaitResponse(wStream, 1*time.Second)
	if !rok {
		t.Errorf("unexpected pb.WatchResponse is received %+v", nr)
	}
}

func TestV3WatchMultipleStreamsSynced(t *testing.T) {
	testV3WatchMultipleStreams(t, 0)
}

func TestV3WatchMultipleStreamsUnsynced(t *testing.T) {
	testV3WatchMultipleStreams(t, 1)
}

// testV3WatchMultipleStreams tests multiple watchers on the same key on multiple streams.
func testV3WatchMultipleStreams(t *testing.T, startRev int64) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	wAPI := pb.NewWatchClient(clus.RandConn())
	kvc := pb.NewKVClient(clus.RandConn())

	streams := make([]pb.Watch_WatchClient, 5)
	for i := range streams {
		wStream, errW := wAPI.Watch(context.TODO())
		if errW != nil {
			t.Fatalf("wAPI.Watch error: %v", errW)
		}
		if err := wStream.Send(&pb.WatchRequest{CreateRequest: &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: startRev}}); err != nil {
			t.Fatalf("wStream.Send error: %v", err)
		}
		streams[i] = wStream
	}

	for _, wStream := range streams {
		wresp, err := wStream.Recv()
		if err != nil {
			t.Fatalf("wStream.Recv error: %v", err)
		}
		if !wresp.Created {
			t.Fatalf("wresp.Created got = %v, want = true", wresp.Created)
		}
	}

	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	var wg sync.WaitGroup
	wg.Add(len(streams))
	wevents := []*storagepb.Event{
		{
			Type: storagepb.PUT,
			Kv:   &storagepb.KeyValue{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 2, Version: 1},
		},
	}
	for i := range streams {
		go func(i int) {
			defer wg.Done()
			wStream := streams[i]
			wresp, err := wStream.Recv()
			if err != nil {
				t.Fatalf("wStream.Recv error: %v", err)
			}
			if wresp.WatchId != 0 {
				t.Errorf("watchId got = %d, want = 0", wresp.WatchId)
			}
			if !reflect.DeepEqual(wresp.Events, wevents) {
				t.Errorf("wresp.Events got = %+v, want = %+v", wresp.Events, wevents)
			}
			// now Recv should block because there is no more events coming
			rok, nr := WaitResponse(wStream, 1*time.Second)
			if !rok {
				t.Errorf("unexpected pb.WatchResponse is received %+v", nr)
			}
		}(i)
	}
	wg.Wait()

	clus.Terminate(t)
}

// WaitResponse waits on the given stream for given duration.
// If there is no more events, true and a nil response will be
// returned closing the WatchClient stream. Or the response will
// be returned.
func WaitResponse(wc pb.Watch_WatchClient, timeout time.Duration) (bool, *pb.WatchResponse) {
	rCh := make(chan *pb.WatchResponse)
	go func() {
		resp, _ := wc.Recv()
		rCh <- resp
	}()
	select {
	case nr := <-rCh:
		return false, nr
	case <-time.After(timeout):
	}
	wc.CloseSend()
	rv, ok := <-rCh
	if rv != nil || !ok {
		return false, rv
	}
	return true, nil
}

func TestV3RangeRequest(t *testing.T) {
	tests := []struct {
		putKeys []string
		reqs    []pb.RangeRequest

		wresps [][]string
		wmores []bool
	}{
		// single key
		{
			[]string{"foo", "bar"},
			[]pb.RangeRequest{
				// exists
				{Key: []byte("foo")},
				// doesn't exist
				{Key: []byte("baz")},
			},

			[][]string{
				{"foo"},
				{},
			},
			[]bool{false, false},
		},
		// multi-key
		{
			[]string{"a", "b", "c", "d", "e"},
			[]pb.RangeRequest{
				// all in range
				{Key: []byte("a"), RangeEnd: []byte("z")},
				// [b, d)
				{Key: []byte("b"), RangeEnd: []byte("d")},
				// out of range
				{Key: []byte("f"), RangeEnd: []byte("z")},
				// [c,c) = empty
				{Key: []byte("c"), RangeEnd: []byte("c")},
				// [d, b) = empty
				{Key: []byte("d"), RangeEnd: []byte("b")},
			},

			[][]string{
				{"a", "b", "c", "d", "e"},
				{"b", "c"},
				{},
				{},
				{},
			},
			[]bool{false, false, false, false, false},
		},
		// revision
		{
			[]string{"a", "b", "c", "d", "e"},
			[]pb.RangeRequest{
				{Key: []byte("a"), RangeEnd: []byte("z"), Revision: 0},
				{Key: []byte("a"), RangeEnd: []byte("z"), Revision: 1},
				{Key: []byte("a"), RangeEnd: []byte("z"), Revision: 2},
				{Key: []byte("a"), RangeEnd: []byte("z"), Revision: 3},
			},

			[][]string{
				{"a", "b", "c", "d", "e"},
				{},
				{"a"},
				{"a", "b"},
			},
			[]bool{false, false, false, false},
		},
		// limit
		{
			[]string{"foo", "bar"},
			[]pb.RangeRequest{
				// more
				{Key: []byte("a"), RangeEnd: []byte("z"), Limit: 1},
				// no more
				{Key: []byte("a"), RangeEnd: []byte("z"), Limit: 2},
			},

			[][]string{
				{"bar"},
				{"bar", "foo"},
			},
			[]bool{true, false},
		},
		// sort
		{
			[]string{"b", "a", "c", "d", "c"},
			[]pb.RangeRequest{
				{
					Key: []byte("a"), RangeEnd: []byte("z"),
					Limit:      1,
					SortOrder:  pb.RangeRequest_ASCEND,
					SortTarget: pb.RangeRequest_KEY,
				},
				{
					Key: []byte("a"), RangeEnd: []byte("z"),
					Limit:      1,
					SortOrder:  pb.RangeRequest_DESCEND,
					SortTarget: pb.RangeRequest_KEY,
				},
				{
					Key: []byte("a"), RangeEnd: []byte("z"),
					Limit:      1,
					SortOrder:  pb.RangeRequest_ASCEND,
					SortTarget: pb.RangeRequest_CREATE,
				},
				{
					Key: []byte("a"), RangeEnd: []byte("z"),
					Limit:      1,
					SortOrder:  pb.RangeRequest_DESCEND,
					SortTarget: pb.RangeRequest_MOD,
				},
				{
					Key: []byte("z"), RangeEnd: []byte("z"),
					Limit:      1,
					SortOrder:  pb.RangeRequest_DESCEND,
					SortTarget: pb.RangeRequest_CREATE,
				},
			},

			[][]string{
				{"a"},
				{"d"},
				{"b"},
				{"c"},
				{},
			},
			[]bool{true, true, true, true, false},
		},
	}

	for i, tt := range tests {
		clus := newClusterGRPC(t, &clusterConfig{size: 3})
		for _, k := range tt.putKeys {
			kvc := pb.NewKVClient(clus.RandConn())
			req := &pb.PutRequest{Key: []byte(k), Value: []byte("bar")}
			if _, err := kvc.Put(context.TODO(), req); err != nil {
				t.Fatalf("#%d: couldn't put key (%v)", i, err)
			}
		}

		for j, req := range tt.reqs {
			kvc := pb.NewKVClient(clus.RandConn())
			resp, err := kvc.Range(context.TODO(), &req)
			if err != nil {
				t.Errorf("#%d.%d: Range error: %v", i, j, err)
				continue
			}
			if len(resp.Kvs) != len(tt.wresps[j]) {
				t.Errorf("#%d.%d: bad len(resp.Kvs). got = %d, want = %d, ", i, j, len(resp.Kvs), len(tt.wresps[j]))
				continue
			}
			for k, wKey := range tt.wresps[j] {
				respKey := string(resp.Kvs[k].Key)
				if respKey != wKey {
					t.Errorf("#%d.%d: key[%d]. got = %v, want = %v, ", i, j, k, respKey, wKey)
				}
			}
			if resp.More != tt.wmores[j] {
				t.Errorf("#%d.%d: bad more. got = %v, want = %v, ", i, j, resp.More, tt.wmores[j])
			}
			wrev := req.Revision
			if wrev == 0 {
				wrev = int64(len(tt.putKeys) + 1)
			}
			if resp.Header.Revision != wrev {
				t.Errorf("#%d.%d: bad header revision. got = %d. want = %d", i, j, resp.Header.Revision, wrev)
			}
		}
		clus.Terminate(t)
	}
}

// TestV3LeaseRevoke ensures a key is deleted once its lease is revoked.
func TestV3LeaseRevoke(t *testing.T) {
	testLeaseRemoveLeasedKey(t, func(clus *clusterV3, leaseID int64) error {
		lc := pb.NewLeaseClient(clus.RandConn())
		_, err := lc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: leaseID})
		return err
	})
}

// TestV3LeaseCreateById ensures leases may be created by a given id.
func TestV3LeaseCreateByID(t *testing.T) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	// create fixed lease
	lresp, err := pb.NewLeaseClient(clus.RandConn()).LeaseCreate(
		context.TODO(),
		&pb.LeaseCreateRequest{ID: 1, TTL: 1})
	if err != nil {
		t.Errorf("could not create lease 1 (%v)", err)
	}
	if lresp.ID != 1 {
		t.Errorf("got id %v, wanted id %v", lresp.ID)
	}

	// create duplicate fixed lease
	lresp, err = pb.NewLeaseClient(clus.RandConn()).LeaseCreate(
		context.TODO(),
		&pb.LeaseCreateRequest{ID: 1, TTL: 1})
	if err != nil {
		t.Error(err)
	}
	if lresp.ID != 0 || lresp.Error != lease.ErrLeaseExists.Error() {
		t.Errorf("got id %v, wanted id 0 (%v)", lresp.ID, lresp.Error)
	}

	// create fresh fixed lease
	lresp, err = pb.NewLeaseClient(clus.RandConn()).LeaseCreate(
		context.TODO(),
		&pb.LeaseCreateRequest{ID: 2, TTL: 1})
	if err != nil {
		t.Errorf("could not create lease 2 (%v)", err)
	}
	if lresp.ID != 2 {
		t.Errorf("got id %v, wanted id %v", lresp.ID)
	}

}

// TestV3LeaseExpire ensures a key is deleted once a key expires.
func TestV3LeaseExpire(t *testing.T) {
	testLeaseRemoveLeasedKey(t, func(clus *clusterV3, leaseID int64) error {
		// let lease lapse; wait for deleted key

		wAPI := pb.NewWatchClient(clus.RandConn())
		wStream, err := wAPI.Watch(context.TODO())
		if err != nil {
			return err
		}

		creq := &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: 1}
		wreq := &pb.WatchRequest{CreateRequest: creq}
		if err := wStream.Send(wreq); err != nil {
			return err
		}
		if _, err := wStream.Recv(); err != nil {
			// the 'created' message
			return err
		}
		if _, err := wStream.Recv(); err != nil {
			// the 'put' message
			return err
		}

		errc := make(chan error, 1)
		go func() {
			resp, err := wStream.Recv()
			switch {
			case err != nil:
				errc <- err
			case len(resp.Events) != 1:
				fallthrough
			case resp.Events[0].Type != storagepb.DELETE:
				errc <- fmt.Errorf("expected key delete, got %v", resp)
			default:
				errc <- nil
			}
		}()

		select {
		case <-time.After(15 * time.Second):
			return fmt.Errorf("lease expiration too slow")
		case err := <-errc:
			return err
		}
	})
}

// TestV3LeaseKeepAlive ensures keepalive keeps the lease alive.
func TestV3LeaseKeepAlive(t *testing.T) {
	testLeaseRemoveLeasedKey(t, func(clus *clusterV3, leaseID int64) error {
		lc := pb.NewLeaseClient(clus.RandConn())
		lreq := &pb.LeaseKeepAliveRequest{ID: leaseID}
		lac, err := lc.LeaseKeepAlive(context.TODO())
		if err != nil {
			return err
		}
		defer lac.CloseSend()

		// renew long enough so lease would've expired otherwise
		for i := 0; i < 3; i++ {
			if err = lac.Send(lreq); err != nil {
				return err
			}
			lresp, rxerr := lac.Recv()
			if rxerr != nil {
				return rxerr
			}
			if lresp.ID != leaseID {
				return fmt.Errorf("expected lease ID %v, got %v", leaseID, lresp.ID)
			}
			time.Sleep(time.Duration(lresp.TTL/2) * time.Second)
		}
		_, err = lc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: leaseID})
		return err
	})
}

// TestV3LeaseExists creates a lease on a random client, then sends a keepalive on another
// client to confirm it's visible to the whole cluster.
func TestV3LeaseExists(t *testing.T) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	// create lease
	lresp, err := pb.NewLeaseClient(clus.RandConn()).LeaseCreate(
		context.TODO(),
		&pb.LeaseCreateRequest{TTL: 30})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}

	// confirm keepalive
	lac, err := pb.NewLeaseClient(clus.RandConn()).LeaseKeepAlive(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
	defer lac.CloseSend()
	if err = lac.Send(&pb.LeaseKeepAliveRequest{ID: lresp.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = lac.Recv(); err != nil {
		t.Fatal(err)
	}
}

// acquireLeaseAndKey creates a new lease and creates an attached key.
func acquireLeaseAndKey(clus *clusterV3, key string) (int64, error) {
	// create lease
	lresp, err := pb.NewLeaseClient(clus.RandConn()).LeaseCreate(
		context.TODO(),
		&pb.LeaseCreateRequest{TTL: 1})
	if err != nil {
		return 0, err
	}
	if lresp.Error != "" {
		return 0, fmt.Errorf(lresp.Error)
	}
	// attach to key
	put := &pb.PutRequest{Key: []byte(key), Lease: lresp.ID}
	if _, err := pb.NewKVClient(clus.RandConn()).Put(context.TODO(), put); err != nil {
		return 0, err
	}
	return lresp.ID, nil
}

// testLeaseRemoveLeasedKey performs some action while holding a lease with an
// attached key "foo", then confirms the key is gone.
func testLeaseRemoveLeasedKey(t *testing.T, act func(*clusterV3, int64) error) {
	clus := newClusterGRPC(t, &clusterConfig{size: 3})
	defer clus.Terminate(t)

	leaseID, err := acquireLeaseAndKey(clus, "foo")
	if err != nil {
		t.Fatal(err)
	}

	if err = act(clus, leaseID); err != nil {
		t.Fatal(err)
	}

	// confirm no key
	rreq := &pb.RangeRequest{Key: []byte("foo")}
	rresp, err := pb.NewKVClient(clus.RandConn()).Range(context.TODO(), rreq)
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 0 {
		t.Fatalf("lease removed but key remains")
	}
}
