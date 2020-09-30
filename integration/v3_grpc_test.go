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

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/etcdserver/api/v3rpc/rpctypes"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/v3/pkg/testutil"
	"go.etcd.io/etcd/v3/pkg/transport"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestV3PutOverwrite puts a key with the v3 api to a random cluster member,
// overwrites it, then checks that the change was applied.
func TestV3PutOverwrite(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
	key := []byte("foo")
	reqput := &pb.PutRequest{Key: key, Value: []byte("bar"), PrevKv: true}

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
	if pkv := respput2.PrevKv; pkv == nil || string(pkv.Value) != "bar" {
		t.Fatalf("expected PrevKv=bar, got response %+v", respput2)
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

// TestPutRestart checks if a put after an unrelated member restart succeeds
func TestV3PutRestart(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvIdx := rand.Intn(3)
	kvc := toGRPC(clus.Client(kvIdx)).KV

	stopIdx := kvIdx
	for stopIdx == kvIdx {
		stopIdx = rand.Intn(3)
	}

	clus.clients[stopIdx].Close()
	clus.Members[stopIdx].Stop(t)
	clus.Members[stopIdx].Restart(t)
	c, cerr := NewClientV3(clus.Members[stopIdx])
	if cerr != nil {
		t.Fatalf("cannot create client: %v", cerr)
	}
	clus.clients[stopIdx] = c

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	reqput := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}
	_, err := kvc.Put(ctx, reqput)
	if err != nil && err == ctx.Err() {
		t.Fatalf("expected grpc error, got local ctx error (%v)", err)
	}
}

// TestV3CompactCurrentRev ensures keys are present when compacting on current revision.
func TestV3CompactCurrentRev(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
	preq := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}
	for i := 0; i < 3; i++ {
		if _, err := kvc.Put(context.Background(), preq); err != nil {
			t.Fatalf("couldn't put key (%v)", err)
		}
	}
	// get key to add to proxy cache, if any
	if _, err := kvc.Range(context.TODO(), &pb.RangeRequest{Key: []byte("foo")}); err != nil {
		t.Fatal(err)
	}
	// compact on current revision
	_, err := kvc.Compact(context.Background(), &pb.CompactionRequest{Revision: 4})
	if err != nil {
		t.Fatalf("couldn't compact kv space (%v)", err)
	}
	// key still exists when linearized?
	_, err = kvc.Range(context.Background(), &pb.RangeRequest{Key: []byte("foo")})
	if err != nil {
		t.Fatalf("couldn't get key after compaction (%v)", err)
	}
	// key still exists when serialized?
	_, err = kvc.Range(context.Background(), &pb.RangeRequest{Key: []byte("foo"), Serializable: true})
	if err != nil {
		t.Fatalf("couldn't get serialized key after compaction (%v)", err)
	}
}

// TestV3HashKV ensures that multiple calls of HashKV on same node return same hash and compact rev.
func TestV3HashKV(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
	mvc := toGRPC(clus.RandClient()).Maintenance

	for i := 0; i < 10; i++ {
		resp, err := kvc.Put(context.Background(), &pb.PutRequest{Key: []byte("foo"), Value: []byte(fmt.Sprintf("bar%d", i))})
		if err != nil {
			t.Fatal(err)
		}

		rev := resp.Header.Revision
		hresp, err := mvc.HashKV(context.Background(), &pb.HashKVRequest{Revision: 0})
		if err != nil {
			t.Fatal(err)
		}
		if rev != hresp.Header.Revision {
			t.Fatalf("Put rev %v != HashKV rev %v", rev, hresp.Header.Revision)
		}

		prevHash := hresp.Hash
		prevCompactRev := hresp.CompactRevision
		for i := 0; i < 10; i++ {
			hresp, err := mvc.HashKV(context.Background(), &pb.HashKVRequest{Revision: 0})
			if err != nil {
				t.Fatal(err)
			}
			if rev != hresp.Header.Revision {
				t.Fatalf("Put rev %v != HashKV rev %v", rev, hresp.Header.Revision)
			}

			if prevHash != hresp.Hash {
				t.Fatalf("prevHash %v != Hash %v", prevHash, hresp.Hash)
			}

			if prevCompactRev != hresp.CompactRevision {
				t.Fatalf("prevCompactRev %v != CompactRevision %v", prevHash, hresp.Hash)
			}

			prevHash = hresp.Hash
			prevCompactRev = hresp.CompactRevision
		}
	}
}

func TestV3TxnTooManyOps(t *testing.T) {
	defer testutil.AfterTest(t)
	maxTxnOps := uint(128)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3, MaxTxnOps: maxTxnOps})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV

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
			&pb.RequestOp{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   keyf(),
						Value: []byte("bar"),
					},
				},
			})
	}
	addFailureOps := func(txn *pb.TxnRequest) {
		txn.Failure = append(txn.Failure,
			&pb.RequestOp{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   keyf(),
						Value: []byte("bar"),
					},
				},
			})
	}
	addTxnOps := func(txn *pb.TxnRequest) {
		newTxn := &pb.TxnRequest{}
		addSuccessOps(newTxn)
		txn.Success = append(txn.Success,
			&pb.RequestOp{Request: &pb.RequestOp_RequestTxn{
				RequestTxn: newTxn,
			},
			},
		)
	}

	tests := []func(txn *pb.TxnRequest){
		addCompareOps,
		addSuccessOps,
		addFailureOps,
		addTxnOps,
	}

	for i, tt := range tests {
		txn := &pb.TxnRequest{}
		for j := 0; j < int(maxTxnOps+1); j++ {
			tt(txn)
		}

		_, err := kvc.Txn(context.Background(), txn)
		if !eqErrGRPC(err, rpctypes.ErrGRPCTooManyOps) {
			t.Errorf("#%d: err = %v, want %v", i, err, rpctypes.ErrGRPCTooManyOps)
		}
	}
}

func TestV3TxnDuplicateKeys(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	putreq := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: &pb.PutRequest{Key: []byte("abc"), Value: []byte("def")}}}
	delKeyReq := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{
		RequestDeleteRange: &pb.DeleteRangeRequest{
			Key: []byte("abc"),
		},
	},
	}
	delInRangeReq := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{
		RequestDeleteRange: &pb.DeleteRangeRequest{
			Key: []byte("a"), RangeEnd: []byte("b"),
		},
	},
	}
	delOutOfRangeReq := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{
		RequestDeleteRange: &pb.DeleteRangeRequest{
			Key: []byte("abb"), RangeEnd: []byte("abc"),
		},
	},
	}
	txnDelReq := &pb.RequestOp{Request: &pb.RequestOp_RequestTxn{
		RequestTxn: &pb.TxnRequest{Success: []*pb.RequestOp{delInRangeReq}},
	},
	}
	txnDelReqTwoSide := &pb.RequestOp{Request: &pb.RequestOp_RequestTxn{
		RequestTxn: &pb.TxnRequest{
			Success: []*pb.RequestOp{delInRangeReq},
			Failure: []*pb.RequestOp{delInRangeReq}},
	},
	}

	txnPutReq := &pb.RequestOp{Request: &pb.RequestOp_RequestTxn{
		RequestTxn: &pb.TxnRequest{Success: []*pb.RequestOp{putreq}},
	},
	}
	txnPutReqTwoSide := &pb.RequestOp{Request: &pb.RequestOp_RequestTxn{
		RequestTxn: &pb.TxnRequest{
			Success: []*pb.RequestOp{putreq},
			Failure: []*pb.RequestOp{putreq}},
	},
	}

	kvc := toGRPC(clus.RandClient()).KV
	tests := []struct {
		txnSuccess []*pb.RequestOp

		werr error
	}{
		{
			txnSuccess: []*pb.RequestOp{putreq, putreq},

			werr: rpctypes.ErrGRPCDuplicateKey,
		},
		{
			txnSuccess: []*pb.RequestOp{putreq, delKeyReq},

			werr: rpctypes.ErrGRPCDuplicateKey,
		},
		{
			txnSuccess: []*pb.RequestOp{putreq, delInRangeReq},

			werr: rpctypes.ErrGRPCDuplicateKey,
		},
		// Then(Put(a), Then(Del(a)))
		{
			txnSuccess: []*pb.RequestOp{putreq, txnDelReq},

			werr: rpctypes.ErrGRPCDuplicateKey,
		},
		// Then(Del(a), Then(Put(a)))
		{
			txnSuccess: []*pb.RequestOp{delInRangeReq, txnPutReq},

			werr: rpctypes.ErrGRPCDuplicateKey,
		},
		// Then((Then(Put(a)), Else(Put(a))), (Then(Put(a)), Else(Put(a)))
		{
			txnSuccess: []*pb.RequestOp{txnPutReqTwoSide, txnPutReqTwoSide},

			werr: rpctypes.ErrGRPCDuplicateKey,
		},
		// Then(Del(x), (Then(Put(a)), Else(Put(a))))
		{
			txnSuccess: []*pb.RequestOp{delOutOfRangeReq, txnPutReqTwoSide},

			werr: nil,
		},
		// Then(Then(Del(a)), (Then(Del(a)), Else(Del(a))))
		{
			txnSuccess: []*pb.RequestOp{txnDelReq, txnDelReqTwoSide},

			werr: nil,
		},
		{
			txnSuccess: []*pb.RequestOp{delKeyReq, delInRangeReq, delKeyReq, delInRangeReq},

			werr: nil,
		},
		{
			txnSuccess: []*pb.RequestOp{putreq, delOutOfRangeReq},

			werr: nil,
		},
	}
	for i, tt := range tests {
		txn := &pb.TxnRequest{Success: tt.txnSuccess}
		_, err := kvc.Txn(context.Background(), txn)
		if !eqErrGRPC(err, tt.werr) {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
	}
}

// Testv3TxnRevision tests that the transaction header revision is set as expected.
func TestV3TxnRevision(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
	pr := &pb.PutRequest{Key: []byte("abc"), Value: []byte("def")}
	presp, err := kvc.Put(context.TODO(), pr)
	if err != nil {
		t.Fatal(err)
	}

	txnget := &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: &pb.RangeRequest{Key: []byte("abc")}}}
	txn := &pb.TxnRequest{Success: []*pb.RequestOp{txnget}}
	tresp, err := kvc.Txn(context.TODO(), txn)
	if err != nil {
		t.Fatal(err)
	}

	// did not update revision
	if presp.Header.Revision != tresp.Header.Revision {
		t.Fatalf("got rev %d, wanted rev %d", tresp.Header.Revision, presp.Header.Revision)
	}

	txndr := &pb.RequestOp{Request: &pb.RequestOp_RequestDeleteRange{RequestDeleteRange: &pb.DeleteRangeRequest{Key: []byte("def")}}}
	txn = &pb.TxnRequest{Success: []*pb.RequestOp{txndr}}
	tresp, err = kvc.Txn(context.TODO(), txn)
	if err != nil {
		t.Fatal(err)
	}

	// did not update revision
	if presp.Header.Revision != tresp.Header.Revision {
		t.Fatalf("got rev %d, wanted rev %d", tresp.Header.Revision, presp.Header.Revision)
	}

	txnput := &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: &pb.PutRequest{Key: []byte("abc"), Value: []byte("123")}}}
	txn = &pb.TxnRequest{Success: []*pb.RequestOp{txnput}}
	tresp, err = kvc.Txn(context.TODO(), txn)
	if err != nil {
		t.Fatal(err)
	}

	// updated revision
	if tresp.Header.Revision != presp.Header.Revision+1 {
		t.Fatalf("got rev %d, wanted rev %d", tresp.Header.Revision, presp.Header.Revision+1)
	}
}

// Testv3TxnCmpHeaderRev tests that the txn header revision is set as expected
// when compared to the Succeeded field in the txn response.
func TestV3TxnCmpHeaderRev(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV

	for i := 0; i < 10; i++ {
		// Concurrently put a key with a txn comparing on it.
		revc := make(chan int64, 1)
		errCh := make(chan error, 1)
		go func() {
			defer close(revc)
			pr := &pb.PutRequest{Key: []byte("k"), Value: []byte("v")}
			presp, err := kvc.Put(context.TODO(), pr)
			errCh <- err
			if err != nil {
				return
			}
			revc <- presp.Header.Revision
		}()

		// The read-only txn uses the optimized readindex server path.
		txnget := &pb.RequestOp{Request: &pb.RequestOp_RequestRange{
			RequestRange: &pb.RangeRequest{Key: []byte("k")}}}
		txn := &pb.TxnRequest{Success: []*pb.RequestOp{txnget}}
		// i = 0 /\ Succeeded => put followed txn
		cmp := &pb.Compare{
			Result:      pb.Compare_EQUAL,
			Target:      pb.Compare_VERSION,
			Key:         []byte("k"),
			TargetUnion: &pb.Compare_Version{Version: int64(i)},
		}
		txn.Compare = append(txn.Compare, cmp)

		tresp, err := kvc.Txn(context.TODO(), txn)
		if err != nil {
			t.Fatal(err)
		}

		prev := <-revc
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
		// put followed txn; should eval to false
		if prev > tresp.Header.Revision && !tresp.Succeeded {
			t.Errorf("#%d: got else but put rev %d followed txn rev (%+v)", i, prev, tresp)
		}
		// txn follows put; should eval to true
		if tresp.Header.Revision >= prev && tresp.Succeeded {
			t.Errorf("#%d: got then but put rev %d preceded txn (%+v)", i, prev, tresp)
		}
	}
}

// TestV3TxnRangeCompare tests range comparisons in txns
func TestV3TxnRangeCompare(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	// put keys, named by expected revision
	for _, k := range []string{"/a/2", "/a/3", "/a/4", "/f/5"} {
		if _, err := clus.Client(0).Put(context.TODO(), k, "x"); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		cmp pb.Compare

		wSuccess bool
	}{
		{
			// >= /a/; all create revs fit
			pb.Compare{
				Key:         []byte("/a/"),
				RangeEnd:    []byte{0},
				Target:      pb.Compare_CREATE,
				Result:      pb.Compare_LESS,
				TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 6},
			},
			true,
		},
		{
			// >= /a/; one create rev doesn't fit
			pb.Compare{
				Key:         []byte("/a/"),
				RangeEnd:    []byte{0},
				Target:      pb.Compare_CREATE,
				Result:      pb.Compare_LESS,
				TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 5},
			},
			false,
		},
		{
			// prefix /a/*; all create revs fit
			pb.Compare{
				Key:         []byte("/a/"),
				RangeEnd:    []byte("/a0"),
				Target:      pb.Compare_CREATE,
				Result:      pb.Compare_LESS,
				TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 5},
			},
			true,
		},
		{
			// prefix /a/*; one create rev doesn't fit
			pb.Compare{
				Key:         []byte("/a/"),
				RangeEnd:    []byte("/a0"),
				Target:      pb.Compare_CREATE,
				Result:      pb.Compare_LESS,
				TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 4},
			},
			false,
		},
		{
			// does not exist, does not succeed
			pb.Compare{
				Key:         []byte("/b/"),
				RangeEnd:    []byte("/b0"),
				Target:      pb.Compare_VALUE,
				Result:      pb.Compare_EQUAL,
				TargetUnion: &pb.Compare_Value{Value: []byte("x")},
			},
			false,
		},
		{
			// all keys are leased
			pb.Compare{
				Key:         []byte("/a/"),
				RangeEnd:    []byte("/a0"),
				Target:      pb.Compare_LEASE,
				Result:      pb.Compare_GREATER,
				TargetUnion: &pb.Compare_Lease{Lease: 0},
			},
			false,
		},
		{
			// no keys are leased
			pb.Compare{
				Key:         []byte("/a/"),
				RangeEnd:    []byte("/a0"),
				Target:      pb.Compare_LEASE,
				Result:      pb.Compare_EQUAL,
				TargetUnion: &pb.Compare_Lease{Lease: 0},
			},
			true,
		},
	}

	kvc := toGRPC(clus.Client(0)).KV
	for i, tt := range tests {
		txn := &pb.TxnRequest{}
		txn.Compare = append(txn.Compare, &tt.cmp)
		tresp, err := kvc.Txn(context.TODO(), txn)
		if err != nil {
			t.Fatal(err)
		}
		if tt.wSuccess != tresp.Succeeded {
			t.Errorf("#%d: expected %v, got %v", i, tt.wSuccess, tresp.Succeeded)
		}
	}
}

// TestV3TxnNested tests nested txns follow paths as expected.
func TestV3TxnNestedPath(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV

	cmpTrue := &pb.Compare{
		Result:      pb.Compare_EQUAL,
		Target:      pb.Compare_VERSION,
		Key:         []byte("k"),
		TargetUnion: &pb.Compare_Version{Version: int64(0)},
	}
	cmpFalse := &pb.Compare{
		Result:      pb.Compare_EQUAL,
		Target:      pb.Compare_VERSION,
		Key:         []byte("k"),
		TargetUnion: &pb.Compare_Version{Version: int64(1)},
	}

	// generate random path to eval txns
	topTxn := &pb.TxnRequest{}
	txn := topTxn
	txnPath := make([]bool, 10)
	for i := range txnPath {
		nextTxn := &pb.TxnRequest{}
		op := &pb.RequestOp{Request: &pb.RequestOp_RequestTxn{RequestTxn: nextTxn}}
		txnPath[i] = rand.Intn(2) == 0
		if txnPath[i] {
			txn.Compare = append(txn.Compare, cmpTrue)
			txn.Success = append(txn.Success, op)
		} else {
			txn.Compare = append(txn.Compare, cmpFalse)
			txn.Failure = append(txn.Failure, op)
		}
		txn = nextTxn
	}

	tresp, err := kvc.Txn(context.TODO(), topTxn)
	if err != nil {
		t.Fatal(err)
	}

	curTxnResp := tresp
	for i := range txnPath {
		if curTxnResp.Succeeded != txnPath[i] {
			t.Fatalf("expected path %+v, got response %+v", txnPath, *tresp)
		}
		curTxnResp = curTxnResp.Responses[0].Response.(*pb.ResponseOp_ResponseTxn).ResponseTxn
	}
}

// TestV3PutIgnoreValue ensures that writes with ignore_value overwrites with previous key-value pair.
func TestV3PutIgnoreValue(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
	key, val := []byte("foo"), []byte("bar")
	putReq := pb.PutRequest{Key: key, Value: val}

	// create lease
	lc := toGRPC(clus.RandClient()).Lease
	lresp, err := lc.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: 30})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}

	tests := []struct {
		putFunc  func() error
		putErr   error
		wleaseID int64
	}{
		{ // put failure for non-existent key
			func() error {
				preq := putReq
				preq.IgnoreValue = true
				_, err := kvc.Put(context.TODO(), &preq)
				return err
			},
			rpctypes.ErrGRPCKeyNotFound,
			0,
		},
		{ // txn failure for non-existent key
			func() error {
				preq := putReq
				preq.Value = nil
				preq.IgnoreValue = true
				txn := &pb.TxnRequest{}
				txn.Success = append(txn.Success, &pb.RequestOp{
					Request: &pb.RequestOp_RequestPut{RequestPut: &preq}})
				_, err := kvc.Txn(context.TODO(), txn)
				return err
			},
			rpctypes.ErrGRPCKeyNotFound,
			0,
		},
		{ // put success
			func() error {
				_, err := kvc.Put(context.TODO(), &putReq)
				return err
			},
			nil,
			0,
		},
		{ // txn success, attach lease
			func() error {
				preq := putReq
				preq.Value = nil
				preq.Lease = lresp.ID
				preq.IgnoreValue = true
				txn := &pb.TxnRequest{}
				txn.Success = append(txn.Success, &pb.RequestOp{
					Request: &pb.RequestOp_RequestPut{RequestPut: &preq}})
				_, err := kvc.Txn(context.TODO(), txn)
				return err
			},
			nil,
			lresp.ID,
		},
		{ // non-empty value with ignore_value should error
			func() error {
				preq := putReq
				preq.IgnoreValue = true
				_, err := kvc.Put(context.TODO(), &preq)
				return err
			},
			rpctypes.ErrGRPCValueProvided,
			0,
		},
		{ // overwrite with previous value, ensure no prev-kv is returned and lease is detached
			func() error {
				preq := putReq
				preq.Value = nil
				preq.IgnoreValue = true
				presp, err := kvc.Put(context.TODO(), &preq)
				if err != nil {
					return err
				}
				if presp.PrevKv != nil && len(presp.PrevKv.Key) != 0 {
					return fmt.Errorf("unexexpected previous key-value %v", presp.PrevKv)
				}
				return nil
			},
			nil,
			0,
		},
		{ // revoke lease, ensure detached key doesn't get deleted
			func() error {
				_, err := lc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: lresp.ID})
				return err
			},
			nil,
			0,
		},
	}

	for i, tt := range tests {
		if err := tt.putFunc(); !eqErrGRPC(err, tt.putErr) {
			t.Fatalf("#%d: err expected %v, got %v", i, tt.putErr, err)
		}
		if tt.putErr != nil {
			continue
		}
		rr, err := kvc.Range(context.TODO(), &pb.RangeRequest{Key: key})
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		if len(rr.Kvs) != 1 {
			t.Fatalf("#%d: len(rr.KVs) expected 1, got %d", i, len(rr.Kvs))
		}
		if !bytes.Equal(rr.Kvs[0].Value, val) {
			t.Fatalf("#%d: value expected %q, got %q", i, val, rr.Kvs[0].Value)
		}
		if rr.Kvs[0].Lease != tt.wleaseID {
			t.Fatalf("#%d: lease ID expected %d, got %d", i, tt.wleaseID, rr.Kvs[0].Lease)
		}
	}
}

// TestV3PutIgnoreLease ensures that writes with ignore_lease uses previous lease for the key overwrites.
func TestV3PutIgnoreLease(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV

	// create lease
	lc := toGRPC(clus.RandClient()).Lease
	lresp, err := lc.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: 30})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}

	key, val, val1 := []byte("zoo"), []byte("bar"), []byte("bar1")
	putReq := pb.PutRequest{Key: key, Value: val}

	tests := []struct {
		putFunc  func() error
		putErr   error
		wleaseID int64
		wvalue   []byte
	}{
		{ // put failure for non-existent key
			func() error {
				preq := putReq
				preq.IgnoreLease = true
				_, err := kvc.Put(context.TODO(), &preq)
				return err
			},
			rpctypes.ErrGRPCKeyNotFound,
			0,
			nil,
		},
		{ // txn failure for non-existent key
			func() error {
				preq := putReq
				preq.IgnoreLease = true
				txn := &pb.TxnRequest{}
				txn.Success = append(txn.Success, &pb.RequestOp{
					Request: &pb.RequestOp_RequestPut{RequestPut: &preq}})
				_, err := kvc.Txn(context.TODO(), txn)
				return err
			},
			rpctypes.ErrGRPCKeyNotFound,
			0,
			nil,
		},
		{ // put success
			func() error {
				preq := putReq
				preq.Lease = lresp.ID
				_, err := kvc.Put(context.TODO(), &preq)
				return err
			},
			nil,
			lresp.ID,
			val,
		},
		{ // txn success, modify value using 'ignore_lease' and ensure lease is not detached
			func() error {
				preq := putReq
				preq.Value = val1
				preq.IgnoreLease = true
				txn := &pb.TxnRequest{}
				txn.Success = append(txn.Success, &pb.RequestOp{
					Request: &pb.RequestOp_RequestPut{RequestPut: &preq}})
				_, err := kvc.Txn(context.TODO(), txn)
				return err
			},
			nil,
			lresp.ID,
			val1,
		},
		{ // non-empty lease with ignore_lease should error
			func() error {
				preq := putReq
				preq.Lease = lresp.ID
				preq.IgnoreLease = true
				_, err := kvc.Put(context.TODO(), &preq)
				return err
			},
			rpctypes.ErrGRPCLeaseProvided,
			0,
			nil,
		},
		{ // overwrite with previous value, ensure no prev-kv is returned and lease is detached
			func() error {
				presp, err := kvc.Put(context.TODO(), &putReq)
				if err != nil {
					return err
				}
				if presp.PrevKv != nil && len(presp.PrevKv.Key) != 0 {
					return fmt.Errorf("unexexpected previous key-value %v", presp.PrevKv)
				}
				return nil
			},
			nil,
			0,
			val,
		},
		{ // revoke lease, ensure detached key doesn't get deleted
			func() error {
				_, err := lc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: lresp.ID})
				return err
			},
			nil,
			0,
			val,
		},
	}

	for i, tt := range tests {
		if err := tt.putFunc(); !eqErrGRPC(err, tt.putErr) {
			t.Fatalf("#%d: err expected %v, got %v", i, tt.putErr, err)
		}
		if tt.putErr != nil {
			continue
		}
		rr, err := kvc.Range(context.TODO(), &pb.RangeRequest{Key: key})
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		if len(rr.Kvs) != 1 {
			t.Fatalf("#%d: len(rr.KVs) expected 1, got %d", i, len(rr.Kvs))
		}
		if !bytes.Equal(rr.Kvs[0].Value, tt.wvalue) {
			t.Fatalf("#%d: value expected %q, got %q", i, val, rr.Kvs[0].Value)
		}
		if rr.Kvs[0].Lease != tt.wleaseID {
			t.Fatalf("#%d: lease ID expected %d, got %d", i, tt.wleaseID, rr.Kvs[0].Lease)
		}
	}
}

// TestV3PutMissingLease ensures that a Put on a key with a bogus lease fails.
func TestV3PutMissingLease(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
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
			txn.Success = append(txn.Success, &pb.RequestOp{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: preq}})
			if tresp, err := kvc.Txn(context.TODO(), txn); err == nil {
				t.Errorf("succeeded txn success. req: %v. resp: %v", txn, tresp)
			}
		},
		// txn failure case
		func() {
			txn := &pb.TxnRequest{}
			txn.Failure = append(txn.Failure, &pb.RequestOp{
				Request: &pb.RequestOp_RequestPut{
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
			txn.Success = append(txn.Success, &pb.RequestOp{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: rreq}})
			txn.Failure = append(txn.Failure, &pb.RequestOp{
				Request: &pb.RequestOp_RequestPut{
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
		prevKV bool

		wantSet [][]byte
		deleted int64
	}{
		// delete middle
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/", "fop", false,
			[][]byte{[]byte("foo"), []byte("fop")}, 1,
		},
		// no delete
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/", "foo/", false,
			[][]byte{[]byte("foo"), []byte("foo/abc"), []byte("fop")}, 0,
		},
		// delete first
		{
			[]string{"foo", "foo/abc", "fop"},
			"fo", "fop", false,
			[][]byte{[]byte("fop")}, 2,
		},
		// delete tail
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/", "fos", false,
			[][]byte{[]byte("foo")}, 2,
		},
		// delete exact
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/abc", "", false,
			[][]byte{[]byte("foo"), []byte("fop")}, 1,
		},
		// delete none, [x,x)
		{
			[]string{"foo"},
			"foo", "foo", false,
			[][]byte{[]byte("foo")}, 0,
		},
		// delete middle with preserveKVs set
		{
			[]string{"foo", "foo/abc", "fop"},
			"foo/", "fop", true,
			[][]byte{[]byte("foo"), []byte("fop")}, 1,
		},
	}

	for i, tt := range tests {
		clus := NewClusterV3(t, &ClusterConfig{Size: 3})
		kvc := toGRPC(clus.RandClient()).KV

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
			RangeEnd: []byte(tt.end),
			PrevKv:   tt.prevKV,
		}
		dresp, err := kvc.DeleteRange(context.TODO(), dreq)
		if err != nil {
			t.Fatalf("couldn't delete range on test %d (%v)", i, err)
		}
		if tt.deleted != dresp.Deleted {
			t.Errorf("expected %d on test %v, got %d", tt.deleted, i, dresp.Deleted)
		}
		if tt.prevKV {
			if len(dresp.PrevKvs) != int(dresp.Deleted) {
				t.Errorf("preserve %d keys, want %d", len(dresp.PrevKvs), dresp.Deleted)
			}
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
		if !reflect.DeepEqual(tt.wantSet, keys) {
			t.Errorf("expected %v on test %v, got %v", tt.wantSet, i, keys)
		}
		// can't defer because tcp ports will be in use
		clus.Terminate(t)
	}
}

// TestV3TxnInvalidRange tests that invalid ranges are rejected in txns.
func TestV3TxnInvalidRange(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
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
	txn.Success = append(txn.Success, &pb.RequestOp{
		Request: &pb.RequestOp_RequestPut{
			RequestPut: preq}})

	rreq := &pb.RangeRequest{Key: []byte("foo"), Revision: 100}
	txn.Success = append(txn.Success, &pb.RequestOp{
		Request: &pb.RequestOp_RequestRange{
			RequestRange: rreq}})

	if _, err := kvc.Txn(context.TODO(), txn); !eqErrGRPC(err, rpctypes.ErrGRPCFutureRev) {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrGRPCFutureRev)
	}

	// compacted rev
	tv, _ := txn.Success[1].Request.(*pb.RequestOp_RequestRange)
	tv.RequestRange.Revision = 1
	if _, err := kvc.Txn(context.TODO(), txn); !eqErrGRPC(err, rpctypes.ErrGRPCCompacted) {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrGRPCCompacted)
	}
}

func TestV3TooLargeRequest(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV

	// 2MB request value
	largeV := make([]byte, 2*1024*1024)
	preq := &pb.PutRequest{Key: []byte("foo"), Value: largeV}

	_, err := kvc.Put(context.Background(), preq)
	if !eqErrGRPC(err, rpctypes.ErrGRPCRequestTooLarge) {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrGRPCRequestTooLarge)
	}
}

// TestV3Hash tests hash.
func TestV3Hash(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cli := clus.RandClient()
	kvc := toGRPC(cli).KV
	m := toGRPC(cli).Maintenance

	preq := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}

	for i := 0; i < 3; i++ {
		_, err := kvc.Put(context.Background(), preq)
		if err != nil {
			t.Fatalf("couldn't put key (%v)", err)
		}
	}

	resp, err := m.Hash(context.Background(), &pb.HashRequest{})
	if err != nil || resp.Hash == 0 {
		t.Fatalf("couldn't hash (%v, hash %d)", err, resp.Hash)
	}
}

// TestV3HashRestart ensures that hash stays the same after restart.
func TestV3HashRestart(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()
	resp, err := toGRPC(cli).Maintenance.Hash(context.Background(), &pb.HashRequest{})
	if err != nil || resp.Hash == 0 {
		t.Fatalf("couldn't hash (%v, hash %d)", err, resp.Hash)
	}
	hash1 := resp.Hash

	clus.Members[0].Stop(t)
	clus.Members[0].Restart(t)
	clus.waitLeader(t, clus.Members)
	kvc := toGRPC(clus.Client(0)).KV
	waitForRestart(t, kvc)

	cli = clus.RandClient()
	resp, err = toGRPC(cli).Maintenance.Hash(context.Background(), &pb.HashRequest{})
	if err != nil || resp.Hash == 0 {
		t.Fatalf("couldn't hash (%v, hash %d)", err, resp.Hash)
	}
	hash2 := resp.Hash

	if hash1 != hash2 {
		t.Fatalf("hash expected %d, got %d", hash1, hash2)
	}
}

// TestV3StorageQuotaAPI tests the V3 server respects quotas at the API layer
func TestV3StorageQuotaAPI(t *testing.T) {
	defer testutil.AfterTest(t)
	quotasize := int64(16 * os.Getpagesize())

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})

	// Set a quota on one node
	clus.Members[0].QuotaBackendBytes = quotasize
	clus.Members[0].Stop(t)
	clus.Members[0].Restart(t)

	defer clus.Terminate(t)
	kvc := toGRPC(clus.Client(0)).KV
	waitForRestart(t, kvc)

	key := []byte("abc")

	// test small put that fits in quota
	smallbuf := make([]byte, 512)
	if _, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: key, Value: smallbuf}); err != nil {
		t.Fatal(err)
	}

	// test big put
	bigbuf := make([]byte, quotasize)
	_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: key, Value: bigbuf})
	if !eqErrGRPC(err, rpctypes.ErrGRPCNoSpace) {
		t.Fatalf("big put got %v, expected %v", err, rpctypes.ErrGRPCNoSpace)
	}

	// test big txn
	puttxn := &pb.RequestOp{
		Request: &pb.RequestOp_RequestPut{
			RequestPut: &pb.PutRequest{
				Key:   key,
				Value: bigbuf,
			},
		},
	}
	txnreq := &pb.TxnRequest{}
	txnreq.Success = append(txnreq.Success, puttxn)
	_, txnerr := kvc.Txn(context.TODO(), txnreq)
	if !eqErrGRPC(txnerr, rpctypes.ErrGRPCNoSpace) {
		t.Fatalf("big txn got %v, expected %v", err, rpctypes.ErrGRPCNoSpace)
	}
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
				// ["\0", "\0") => all in range
				{Key: []byte{0}, RangeEnd: []byte{0}},
			},

			[][]string{
				{"a", "b", "c", "d", "e"},
				{"b", "c"},
				{},
				{},
				{},
				{"a", "b", "c", "d", "e"},
			},
			[]bool{false, false, false, false, false, false},
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
				{ // sort ASCEND by default
					Key: []byte("a"), RangeEnd: []byte("z"),
					Limit:      10,
					SortOrder:  pb.RangeRequest_NONE,
					SortTarget: pb.RangeRequest_CREATE,
				},
			},

			[][]string{
				{"a"},
				{"d"},
				{"b"},
				{"c"},
				{},
				{"b", "a", "c", "d"},
			},
			[]bool{true, true, true, true, false, false},
		},
		// min/max mod rev
		{
			[]string{"rev2", "rev3", "rev4", "rev5", "rev6"},
			[]pb.RangeRequest{
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MinModRevision: 3,
				},
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MaxModRevision: 3,
				},
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MinModRevision: 3,
					MaxModRevision: 5,
				},
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MaxModRevision: 10,
				},
			},

			[][]string{
				{"rev3", "rev4", "rev5", "rev6"},
				{"rev2", "rev3"},
				{"rev3", "rev4", "rev5"},
				{"rev2", "rev3", "rev4", "rev5", "rev6"},
			},
			[]bool{false, false, false, false},
		},
		// min/max create rev
		{
			[]string{"rev2", "rev3", "rev2", "rev2", "rev6", "rev3"},
			[]pb.RangeRequest{
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MinCreateRevision: 3,
				},
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MaxCreateRevision: 3,
				},
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MinCreateRevision: 3,
					MaxCreateRevision: 5,
				},
				{
					Key: []byte{0}, RangeEnd: []byte{0},
					MaxCreateRevision: 10,
				},
			},

			[][]string{
				{"rev3", "rev6"},
				{"rev2", "rev3"},
				{"rev3"},
				{"rev2", "rev3", "rev6"},
			},
			[]bool{false, false, false, false},
		},
	}

	for i, tt := range tests {
		clus := NewClusterV3(t, &ClusterConfig{Size: 3})
		for _, k := range tt.putKeys {
			kvc := toGRPC(clus.RandClient()).KV
			req := &pb.PutRequest{Key: []byte(k), Value: []byte("bar")}
			if _, err := kvc.Put(context.TODO(), req); err != nil {
				t.Fatalf("#%d: couldn't put key (%v)", i, err)
			}
		}

		for j, req := range tt.reqs {
			kvc := toGRPC(clus.RandClient()).KV
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
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("unexpected error (%v)", err)
	} else if client == nil {
		// Ideally, no client would be returned. However, grpc will
		// return a connection without trying to handshake first so
		// the connection appears OK.
		return
	}
	defer client.Close()

	donec := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		reqput := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}
		_, perr := toGRPC(client).KV.Put(ctx, reqput)
		cancel()
		donec <- perr
	}()

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
	clus.Members[0].DialOptions = []grpc.DialOption{grpc.WithBlock()}
	client, err := NewClientV3(clus.Members[0])
	if client != nil || err == nil {
		t.Fatalf("expected no client")
	} else if err != context.DeadlineExceeded {
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
	if _, err := toGRPC(client).KV.Put(context.TODO(), reqput); err != nil {
		t.Fatalf("unexpected error on put over tls (%v)", err)
	}
}

// TestTLSReloadAtomicReplace ensures server reloads expired/valid certs
// when all certs are atomically replaced by directory renaming.
// And expects server to reject client requests, and vice versa.
func TestTLSReloadAtomicReplace(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "fixtures-tmp")
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(tmpDir)
	defer os.RemoveAll(tmpDir)

	certsDir, err := ioutil.TempDir(os.TempDir(), "fixtures-to-load")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(certsDir)

	certsDirExp, err := ioutil.TempDir(os.TempDir(), "fixtures-expired")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(certsDirExp)

	cloneFunc := func() transport.TLSInfo {
		tlsInfo, terr := copyTLSFiles(testTLSInfo, certsDir)
		if terr != nil {
			t.Fatal(terr)
		}
		if _, err = copyTLSFiles(testTLSInfoExpired, certsDirExp); err != nil {
			t.Fatal(err)
		}
		return tlsInfo
	}
	replaceFunc := func() {
		if err = os.Rename(certsDir, tmpDir); err != nil {
			t.Fatal(err)
		}
		if err = os.Rename(certsDirExp, certsDir); err != nil {
			t.Fatal(err)
		}
		// after rename,
		// 'certsDir' contains expired certs
		// 'tmpDir' contains valid certs
		// 'certsDirExp' does not exist
	}
	revertFunc := func() {
		if err = os.Rename(tmpDir, certsDirExp); err != nil {
			t.Fatal(err)
		}
		if err = os.Rename(certsDir, tmpDir); err != nil {
			t.Fatal(err)
		}
		if err = os.Rename(certsDirExp, certsDir); err != nil {
			t.Fatal(err)
		}
	}
	testTLSReload(t, cloneFunc, replaceFunc, revertFunc, false)
}

// TestTLSReloadCopy ensures server reloads expired/valid certs
// when new certs are copied over, one by one. And expects server
// to reject client requests, and vice versa.
func TestTLSReloadCopy(t *testing.T) {
	certsDir, err := ioutil.TempDir(os.TempDir(), "fixtures-to-load")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(certsDir)

	cloneFunc := func() transport.TLSInfo {
		tlsInfo, terr := copyTLSFiles(testTLSInfo, certsDir)
		if terr != nil {
			t.Fatal(terr)
		}
		return tlsInfo
	}
	replaceFunc := func() {
		if _, err = copyTLSFiles(testTLSInfoExpired, certsDir); err != nil {
			t.Fatal(err)
		}
	}
	revertFunc := func() {
		if _, err = copyTLSFiles(testTLSInfo, certsDir); err != nil {
			t.Fatal(err)
		}
	}
	testTLSReload(t, cloneFunc, replaceFunc, revertFunc, false)
}

// TestTLSReloadCopyIPOnly ensures server reloads expired/valid certs
// when new certs are copied over, one by one. And expects server
// to reject client requests, and vice versa.
func TestTLSReloadCopyIPOnly(t *testing.T) {
	certsDir, err := ioutil.TempDir(os.TempDir(), "fixtures-to-load")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(certsDir)

	cloneFunc := func() transport.TLSInfo {
		tlsInfo, terr := copyTLSFiles(testTLSInfoIP, certsDir)
		if terr != nil {
			t.Fatal(terr)
		}
		return tlsInfo
	}
	replaceFunc := func() {
		if _, err = copyTLSFiles(testTLSInfoExpiredIP, certsDir); err != nil {
			t.Fatal(err)
		}
	}
	revertFunc := func() {
		if _, err = copyTLSFiles(testTLSInfoIP, certsDir); err != nil {
			t.Fatal(err)
		}
	}
	testTLSReload(t, cloneFunc, replaceFunc, revertFunc, true)
}

func testTLSReload(
	t *testing.T,
	cloneFunc func() transport.TLSInfo,
	replaceFunc func(),
	revertFunc func(),
	useIP bool) {
	defer testutil.AfterTest(t)

	// 1. separate copies for TLS assets modification
	tlsInfo := cloneFunc()

	// 2. start cluster with valid certs
	clus := NewClusterV3(t, &ClusterConfig{
		Size:      1,
		PeerTLS:   &tlsInfo,
		ClientTLS: &tlsInfo,
		UseIP:     useIP,
	})
	defer clus.Terminate(t)

	// 3. concurrent client dialing while certs become expired
	errc := make(chan error, 1)
	go func() {
		for {
			cc, err := tlsInfo.ClientConfig()
			if err != nil {
				// errors in 'go/src/crypto/tls/tls.go'
				// tls: private key does not match public key
				// tls: failed to find any PEM data in key input
				// tls: failed to find any PEM data in certificate input
				// Or 'does not exist', 'not found', etc
				t.Log(err)
				continue
			}
			cli, cerr := clientv3.New(clientv3.Config{
				DialOptions: []grpc.DialOption{grpc.WithBlock()},
				Endpoints:   []string{clus.Members[0].GRPCAddr()},
				DialTimeout: time.Second,
				TLS:         cc,
			})
			if cerr != nil {
				errc <- cerr
				return
			}
			cli.Close()
		}
	}()

	// 4. replace certs with expired ones
	replaceFunc()

	// 5. expect dial time-out when loading expired certs
	select {
	case gerr := <-errc:
		if gerr != context.DeadlineExceeded {
			t.Fatalf("expected %v, got %v", context.DeadlineExceeded, gerr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("failed to receive dial timeout error")
	}

	// 6. replace expired certs back with valid ones
	revertFunc()

	// 7. new requests should trigger listener to reload valid certs
	tls, terr := tlsInfo.ClientConfig()
	if terr != nil {
		t.Fatal(terr)
	}
	cl, cerr := clientv3.New(clientv3.Config{
		Endpoints:   []string{clus.Members[0].GRPCAddr()},
		DialTimeout: 5 * time.Second,
		TLS:         tls,
	})
	if cerr != nil {
		t.Fatalf("expected no error, got %v", cerr)
	}
	cl.Close()
}

func TestGRPCRequireLeader(t *testing.T) {
	defer testutil.AfterTest(t)

	cfg := ClusterConfig{Size: 3}
	clus := newClusterV3NoClients(t, &cfg)
	defer clus.Terminate(t)

	clus.Members[1].Stop(t)
	clus.Members[2].Stop(t)

	client, err := NewClientV3(clus.Members[0])
	if err != nil {
		t.Fatalf("cannot create client: %v", err)
	}
	defer client.Close()

	// wait for election timeout, then member[0] will not have a leader.
	time.Sleep(time.Duration(3*electionTicks) * tickDuration)

	md := metadata.Pairs(rpctypes.MetadataRequireLeaderKey, rpctypes.MetadataHasLeader)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	reqput := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}
	if _, err := toGRPC(client).KV.Put(ctx, reqput); rpctypes.ErrorDesc(err) != rpctypes.ErrNoLeader.Error() {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrNoLeader)
	}
}

func TestGRPCStreamRequireLeader(t *testing.T) {
	defer testutil.AfterTest(t)

	cfg := ClusterConfig{Size: 3}
	clus := newClusterV3NoClients(t, &cfg)
	defer clus.Terminate(t)

	client, err := NewClientV3(clus.Members[0])
	if err != nil {
		t.Fatalf("failed to create client (%v)", err)
	}
	defer client.Close()

	wAPI := toGRPC(client).Watch
	md := metadata.Pairs(rpctypes.MetadataRequireLeaderKey, rpctypes.MetadataHasLeader)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	wStream, err := wAPI.Watch(ctx)
	if err != nil {
		t.Fatalf("wAPI.Watch error: %v", err)
	}

	clus.Members[1].Stop(t)
	clus.Members[2].Stop(t)

	// existing stream should be rejected
	_, err = wStream.Recv()
	if rpctypes.ErrorDesc(err) != rpctypes.ErrNoLeader.Error() {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrNoLeader)
	}

	// new stream should also be rejected
	wStream, err = wAPI.Watch(ctx)
	if err != nil {
		t.Fatalf("wAPI.Watch error: %v", err)
	}
	_, err = wStream.Recv()
	if rpctypes.ErrorDesc(err) != rpctypes.ErrNoLeader.Error() {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrNoLeader)
	}

	clus.Members[1].Restart(t)
	clus.Members[2].Restart(t)

	clus.waitLeader(t, clus.Members)
	time.Sleep(time.Duration(2*electionTicks) * tickDuration)

	// new stream should also be OK now after we restarted the other members
	wStream, err = wAPI.Watch(ctx)
	if err != nil {
		t.Fatalf("wAPI.Watch error: %v", err)
	}
	wreq := &pb.WatchRequest{
		RequestUnion: &pb.WatchRequest_CreateRequest{
			CreateRequest: &pb.WatchCreateRequest{Key: []byte("foo")},
		},
	}
	err = wStream.Send(wreq)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

// TestV3LargeRequests ensures that configurable MaxRequestBytes works as intended.
func TestV3LargeRequests(t *testing.T) {
	defer testutil.AfterTest(t)
	tests := []struct {
		maxRequestBytes uint
		valueSize       int
		expectError     error
	}{
		// don't set to 0. use 0 as the default.
		{256, 1024, rpctypes.ErrGRPCRequestTooLarge},
		{10 * 1024 * 1024, 9 * 1024 * 1024, nil},
		{10 * 1024 * 1024, 10 * 1024 * 1024, rpctypes.ErrGRPCRequestTooLarge},
		{10 * 1024 * 1024, 10*1024*1024 + 5, rpctypes.ErrGRPCRequestTooLarge},
	}
	for i, test := range tests {
		clus := NewClusterV3(t, &ClusterConfig{Size: 1, MaxRequestBytes: test.maxRequestBytes})
		kvcli := toGRPC(clus.Client(0)).KV
		reqput := &pb.PutRequest{Key: []byte("foo"), Value: make([]byte, test.valueSize)}
		_, err := kvcli.Put(context.TODO(), reqput)
		if !eqErrGRPC(err, test.expectError) {
			t.Errorf("#%d: expected error %v, got %v", i, test.expectError, err)
		}

		// request went through, expect large response back from server
		if test.expectError == nil {
			reqget := &pb.RangeRequest{Key: []byte("foo")}
			// limit receive call size with original value + gRPC overhead bytes
			_, err = kvcli.Range(context.TODO(), reqget, grpc.MaxCallRecvMsgSize(test.valueSize+512*1024))
			if err != nil {
				t.Errorf("#%d: range expected no error, got %v", i, err)
			}
		}

		clus.Terminate(t)
	}
}

func eqErrGRPC(err1 error, err2 error) bool {
	return !(err1 == nil && err2 != nil) || err1.Error() == err2.Error()
}

// waitForRestart tries a range request until the client's server responds.
// This is mainly a stop-gap function until grpcproxy's KVClient adapter
// (and by extension, clientv3) supports grpc.CallOption pass-through so
// FailFast=false works with Put.
func waitForRestart(t *testing.T, kvc pb.KVClient) {
	req := &pb.RangeRequest{Key: []byte("_"), Serializable: true}
	// TODO: Remove retry loop once the new grpc load balancer provides retry.
	var err error
	for i := 0; i < 10; i++ {
		if _, err = kvc.Range(context.TODO(), req, grpc.FailFast(false)); err != nil {
			if status, ok := status.FromError(err); ok && status.Code() == codes.Unavailable {
				time.Sleep(time.Millisecond * 250)
			} else {
				t.Fatal(err)
			}
		}
	}
	if err != nil {
		t.Fatalf("timed out waiting for restart: %v", err)
	}
}
