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
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/storage/storagepb"
)

// TestV3PutOverwrite puts a key with the v3 api to a random cluster member,
// overwrites it, then checks that the change was applied.
func TestV3PutOverwrite(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := clus.RandClient().KV
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
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := clus.RandClient().KV

	// unique keys
	i := new(int)
	keyf := func() []byte {
		*i++
		return []byte(fmt.Sprintf("key-%d", i))
	}

	addCompareOps := func(txn *pb.TxnRequest) {
		txn.Compare = append(txn.Compare,
			&pb.Compare{
				Result: pb.Compare_GREATER,
				Target: pb.Compare_CREATE,
				Key:    keyf(),
			})
	}
	addSuccessOps := func(txn *pb.TxnRequest) {
		txn.Success = append(txn.Success,
			&pb.RequestUnion{
				Request: &pb.RequestUnion_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   keyf(),
						Value: []byte("bar"),
					},
				},
			})
	}
	addFailureOps := func(txn *pb.TxnRequest) {
		txn.Failure = append(txn.Failure,
			&pb.RequestUnion{
				Request: &pb.RequestUnion_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   keyf(),
						Value: []byte("bar"),
					},
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

func TestV3TxnDuplicateKeys(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	putreq := &pb.RequestUnion{Request: &pb.RequestUnion_RequestPut{RequestPut: &pb.PutRequest{Key: []byte("abc"), Value: []byte("def")}}}
	delKeyReq := &pb.RequestUnion{Request: &pb.RequestUnion_RequestDeleteRange{
		RequestDeleteRange: &pb.DeleteRangeRequest{
			Key: []byte("abc"),
		},
	},
	}
	delInRangeReq := &pb.RequestUnion{Request: &pb.RequestUnion_RequestDeleteRange{
		RequestDeleteRange: &pb.DeleteRangeRequest{
			Key: []byte("a"), RangeEnd: []byte("b"),
		},
	},
	}
	delOutOfRangeReq := &pb.RequestUnion{Request: &pb.RequestUnion_RequestDeleteRange{
		RequestDeleteRange: &pb.DeleteRangeRequest{
			Key: []byte("abb"), RangeEnd: []byte("abc"),
		},
	},
	}

	kvc := clus.RandClient().KV
	tests := []struct {
		txnSuccess []*pb.RequestUnion

		werr error
	}{
		{
			txnSuccess: []*pb.RequestUnion{putreq, putreq},

			werr: v3rpc.ErrDuplicateKey,
		},
		{
			txnSuccess: []*pb.RequestUnion{putreq, delKeyReq},

			werr: v3rpc.ErrDuplicateKey,
		},
		{
			txnSuccess: []*pb.RequestUnion{putreq, delInRangeReq},

			werr: v3rpc.ErrDuplicateKey,
		},
		{
			txnSuccess: []*pb.RequestUnion{delKeyReq, delInRangeReq, delKeyReq, delInRangeReq},

			werr: nil,
		},
		{
			txnSuccess: []*pb.RequestUnion{putreq, delOutOfRangeReq},

			werr: nil,
		},
	}
	for i, tt := range tests {
		txn := &pb.TxnRequest{Success: tt.txnSuccess}
		_, err := kvc.Txn(context.Background(), txn)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
	}
}

// TestV3PutMissingLease ensures that a Put on a key with a bogus lease fails.
func TestV3PutMissingLease(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := clus.RandClient().KV
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
			txn.Success = append(txn.Success, &pb.RequestUnion{
				Request: &pb.RequestUnion_RequestPut{
					RequestPut: preq}})
			if tresp, err := kvc.Txn(context.TODO(), txn); err == nil {
				t.Errorf("succeeded txn success. req: %v. resp: %v", txn, tresp)
			}
		},
		// txn failure case
		func() {
			txn := &pb.TxnRequest{}
			txn.Failure = append(txn.Failure, &pb.RequestUnion{
				Request: &pb.RequestUnion_RequestPut{
					RequestPut: preq}})
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
			txn.Success = append(txn.Success, &pb.RequestUnion{
				Request: &pb.RequestUnion_RequestRange{
					RequestRange: rreq}})
			txn.Failure = append(txn.Failure, &pb.RequestUnion{
				Request: &pb.RequestUnion_RequestPut{
					RequestPut: preq}})
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
	defer testutil.AfterTest(t)
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
		clus := NewClusterV3(t, &ClusterConfig{Size: 3})
		kvc := clus.RandClient().KV

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
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := clus.RandClient().KV
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
	txn.Success = append(txn.Success, &pb.RequestUnion{
		Request: &pb.RequestUnion_RequestPut{
			RequestPut: preq}})

	rreq := &pb.RangeRequest{Key: []byte("foo"), Revision: 100}
	txn.Success = append(txn.Success, &pb.RequestUnion{
		Request: &pb.RequestUnion_RequestRange{
			RequestRange: rreq}})

	if _, err := kvc.Txn(context.TODO(), txn); err != v3rpc.ErrFutureRev {
		t.Errorf("err = %v, want %v", err, v3rpc.ErrFutureRev)
	}

	// compacted rev
	tv, _ := txn.Success[1].Request.(*pb.RequestUnion_RequestRange)
	tv.RequestRange.Revision = 1
	if _, err := kvc.Txn(context.TODO(), txn); err != v3rpc.ErrCompacted {
		t.Errorf("err = %v, want %v", err, v3rpc.ErrCompacted)
	}
}

// TestV3Hash tests hash.
func TestV3Hash(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := clus.RandClient().KV
	preq := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}

	for i := 0; i < 3; i++ {
		_, err := kvc.Put(context.Background(), preq)
		if err != nil {
			t.Fatalf("couldn't put key (%v)", err)
		}
	}

	resp, err := kvc.Hash(context.Background(), &pb.HashRequest{})
	if err != nil || resp.Hash == 0 {
		t.Fatalf("couldn't hash (%v, hash %d)", err, resp.Hash)
	}
}

// TestV3WatchFromCurrentRevision tests Watch APIs from current revision.
func TestV3WatchFromCurrentRevision(t *testing.T) {
	defer testutil.AfterTest(t)
	tests := []struct {
		putKeys      []string
		watchRequest *pb.WatchRequest

		wresps []*pb.WatchResponse
	}{
		// watch the key, matching
		{
			[]string{"foo"},
			&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Key: []byte("foo")}}},

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
			&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Key: []byte("helloworld")}}},

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
			&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Prefix: []byte("foo")}}},

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
			&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Prefix: []byte("helloworld")}}},

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
			&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Key: []byte("foo")}}},

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
			&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Prefix: []byte("foo")}}},

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
		clus := NewClusterV3(t, &ClusterConfig{Size: 3})

		wAPI := clus.RandClient().Watch
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wStream, err := wAPI.Watch(ctx)
		if err != nil {
			t.Fatalf("#%d: wAPI.Watch error: %v", i, err)
		}

		if err := wStream.Send(tt.watchRequest); err != nil {
			t.Fatalf("#%d: wStream.Send error: %v", i, err)
		}

		go func() {
			for _, k := range tt.putKeys {
				kvc := clus.RandClient().KV
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
	defer testutil.AfterTest(t)
	testV3WatchCancel(t, 0)
}

// TestV3WatchCancelUnsynced tests Watch APIs cancellation from unsynced map.
func TestV3WatchCancelUnsynced(t *testing.T) {
	defer testutil.AfterTest(t)
	testV3WatchCancel(t, 1)
}

func testV3WatchCancel(t *testing.T, startRev int64) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wStream, errW := clus.RandClient().Watch.Watch(ctx)
	if errW != nil {
		t.Fatalf("wAPI.Watch error: %v", errW)
	}

	wreq := &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
		CreateRequest: &pb.WatchCreateRequest{
			Key: []byte("foo"), StartRevision: startRev}}}
	if err := wStream.Send(wreq); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}

	wresp, errR := wStream.Recv()
	if errR != nil {
		t.Errorf("wStream.Recv error: %v", errR)
	}
	if !wresp.Created {
		t.Errorf("wresp.Created got = %v, want = true", wresp.Created)
	}

	creq := &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CancelRequest{
		CancelRequest: &pb.WatchCancelRequest{
			WatchId: wresp.WatchId}}}
	if err := wStream.Send(creq); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}

	cresp, err := wStream.Recv()
	if err != nil {
		t.Errorf("wStream.Recv error: %v", err)
	}
	if !cresp.Canceled {
		t.Errorf("cresp.Canceled got = %v, want = true", cresp.Canceled)
	}

	kvc := clus.RandClient().KV
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
	defer testutil.AfterTest(t)
	testV3WatchMultipleWatchers(t, 0)
}

func TestV3WatchMultipleWatchersUnsynced(t *testing.T) {
	defer testutil.AfterTest(t)
	testV3WatchMultipleWatchers(t, 1)
}

// testV3WatchMultipleWatchers tests multiple watchers on the same key
// and one watcher with matching prefix. It first puts the key
// that matches all watchers, and another key that matches only
// one watcher to test if it receives expected events.
func testV3WatchMultipleWatchers(t *testing.T, startRev int64) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	kvc := clus.RandClient().KV

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wStream, errW := clus.RandClient().Watch.Watch(ctx)
	if errW != nil {
		t.Fatalf("wAPI.Watch error: %v", errW)
	}

	watchKeyN := 4
	for i := 0; i < watchKeyN+1; i++ {
		var wreq *pb.WatchRequest
		if i < watchKeyN {
			wreq = &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Key: []byte("foo"), StartRevision: startRev}}}
		} else {
			wreq = &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
				CreateRequest: &pb.WatchCreateRequest{
					Prefix: []byte("fo"), StartRevision: startRev}}}
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
	defer testutil.AfterTest(t)
	testV3WatchMultipleEventsTxn(t, 0)
}

func TestV3WatchMultipleEventsTxnUnsynced(t *testing.T) {
	defer testutil.AfterTest(t)
	testV3WatchMultipleEventsTxn(t, 1)
}

// testV3WatchMultipleEventsTxn tests Watch APIs when it receives multiple events.
func testV3WatchMultipleEventsTxn(t *testing.T, startRev int64) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wStream, wErr := clus.RandClient().Watch.Watch(ctx)
	if wErr != nil {
		t.Fatalf("wAPI.Watch error: %v", wErr)
	}

	wreq := &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
		CreateRequest: &pb.WatchCreateRequest{
			Prefix: []byte("foo"), StartRevision: startRev}}}
	if err := wStream.Send(wreq); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}

	kvc := clus.RandClient().KV
	txn := pb.TxnRequest{}
	for i := 0; i < 3; i++ {
		ru := &pb.RequestUnion{}
		ru.Request = &pb.RequestUnion_RequestPut{
			RequestPut: &pb.PutRequest{
				Key: []byte(fmt.Sprintf("foo%d", i)), Value: []byte("bar")}}
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
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := clus.RandClient().KV

	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo0"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}
	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo1"), Value: []byte("bar")}); err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wStream, wErr := clus.RandClient().Watch.Watch(ctx)
	if wErr != nil {
		t.Fatalf("wAPI.Watch error: %v", wErr)
	}

	wreq := &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
		CreateRequest: &pb.WatchCreateRequest{
			Prefix: []byte("foo"), StartRevision: 1}}}
	if err := wStream.Send(wreq); err != nil {
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
	defer testutil.AfterTest(t)
	testV3WatchMultipleStreams(t, 0)
}

func TestV3WatchMultipleStreamsUnsynced(t *testing.T) {
	defer testutil.AfterTest(t)
	testV3WatchMultipleStreams(t, 1)
}

// testV3WatchMultipleStreams tests multiple watchers on the same key on multiple streams.
func testV3WatchMultipleStreams(t *testing.T, startRev int64) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	wAPI := clus.RandClient().Watch
	kvc := clus.RandClient().KV

	streams := make([]pb.Watch_WatchClient, 5)
	for i := range streams {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wStream, errW := wAPI.Watch(ctx)
		if errW != nil {
			t.Fatalf("wAPI.Watch error: %v", errW)
		}
		wreq := &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
			CreateRequest: &pb.WatchCreateRequest{
				Key: []byte("foo"), StartRevision: startRev}}}
		if err := wStream.Send(wreq); err != nil {
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
	defer testutil.AfterTest(t)
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
		clus := NewClusterV3(t, &ClusterConfig{Size: 3})
		for _, k := range tt.putKeys {
			kvc := clus.RandClient().KV
			req := &pb.PutRequest{Key: []byte(k), Value: []byte("bar")}
			if _, err := kvc.Put(context.TODO(), req); err != nil {
				t.Fatalf("#%d: couldn't put key (%v)", i, err)
			}
		}

		for j, req := range tt.reqs {
			kvc := clus.RandClient().KV
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
			wrev := int64(len(tt.putKeys) + 1)
			if resp.Header.Revision != wrev {
				t.Errorf("#%d.%d: bad header revision. got = %d. want = %d", i, j, resp.Header.Revision, wrev)
			}
		}
		clus.Terminate(t)
	}
}

func newClusterV3NoClients(t *testing.T, cfg *ClusterConfig) *ClusterV3 {
	cfg.UseV3 = true
	cfg.UseGRPC = true
	clus := &ClusterV3{cluster: NewClusterByConfig(t, cfg)}
	clus.Launch(t)
	return clus
}

// TestTLSGRPCRejectInsecureClient checks that connection is rejected if server is TLS but not client.
func TestTLSGRPCRejectInsecureClient(t *testing.T) {
	defer testutil.AfterTest(t)

	cfg := ClusterConfig{Size: 3, ClientTLS: &testTLSInfo}
	clus := newClusterV3NoClients(t, &cfg)
	defer clus.Terminate(t)

	// nil out TLS field so client will use an insecure connection
	clus.Members[0].ClientTLSInfo = nil
	client, err := NewClientV3(clus.Members[0])
	if err != nil && err != grpc.ErrClientConnTimeout {
		t.Fatalf("unexpected error (%v)", err)
	} else if client == nil {
		// Ideally, no client would be returned. However, grpc will
		// return a connection without trying to handshake first so
		// the connection appears OK.
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	conn := client.ActiveConnection()
	st, err := conn.State()
	if err != nil {
		t.Fatal(err)
	} else if st != grpc.Ready {
		t.Fatalf("expected Ready, got %v", st)
	}

	// rpc will fail to handshake, triggering a connection state change
	donec := make(chan error, 1)
	go func() {
		reqput := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}
		_, perr := client.KV.Put(ctx, reqput)
		donec <- perr
	}()

	st, err = conn.WaitForStateChange(ctx, st)
	if err != nil {
		t.Fatalf("unexpected error waiting for change (%v)", err)
	} else if st != grpc.Connecting && st != grpc.TransientFailure {
		t.Fatalf("expected connecting or transient failure state, got %v", st)
	}

	cancel()
	if perr := <-donec; perr == nil {
		t.Fatalf("expected client error on put")
	}
}

// TestTLSGRPCRejectSecureClient checks that connection is rejected if client is TLS but not server.
func TestTLSGRPCRejectSecureClient(t *testing.T) {
	defer testutil.AfterTest(t)

	cfg := ClusterConfig{Size: 3}
	clus := newClusterV3NoClients(t, &cfg)
	defer clus.Terminate(t)

	clus.Members[0].ClientTLSInfo = &testTLSInfo
	client, err := NewClientV3(clus.Members[0])
	if client != nil || err == nil {
		t.Fatalf("expected no client")
	} else if err != grpc.ErrClientConnTimeout {
		t.Fatalf("unexpected error (%v)", err)
	}
}

// TestTLSGRPCAcceptSecureAll checks that connection is accepted if both client and server are TLS
func TestTLSGRPCAcceptSecureAll(t *testing.T) {
	defer testutil.AfterTest(t)

	cfg := ClusterConfig{Size: 3, ClientTLS: &testTLSInfo}
	clus := newClusterV3NoClients(t, &cfg)
	defer clus.Terminate(t)

	client, err := NewClientV3(clus.Members[0])
	if err != nil {
		t.Fatalf("expected tls client (%v)", err)
	}
	defer client.Close()

	reqput := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}
	if _, err := client.KV.Put(context.TODO(), reqput); err != nil {
		t.Fatalf("unexpected error on put over tls (%v)", err)
	}
}
