// Copyright 2026 The etcd Authors
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

package cmd

import (
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	v3 "go.etcd.io/etcd/client/v3"
)

func TestTxnPutKeys(t *testing.T) {
	keys := txnPutKeys(8, 3)
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	for i, key := range keys {
		if len(key) != 8 {
			t.Fatalf("len(keys[%d]) = %d, want 8", i, len(key))
		}
	}
	if keys[0] == keys[1] || keys[1] == keys[2] || keys[0] == keys[2] {
		t.Fatalf("expected distinct keys, got %q %q %q", keys[0], keys[1], keys[2])
	}
}

func TestTxnPutObservedModRevisions(t *testing.T) {
	txnResp := &v3.TxnResponse{
		Responses: []*pb.ResponseOp{
			rangeResponseOp(7),
			rangeResponseOp(0),
		},
	}

	revisions, err := txnPutObservedModRevisions(txnResp, 2)
	if err != nil {
		t.Fatalf("txnPutObservedModRevisions returned error: %v", err)
	}
	if revisions[0] != 7 || revisions[1] != 0 {
		t.Fatalf("txnPutObservedModRevisions = %v, want [7 0]", revisions)
	}
}

func TestTxnPutObservedModRevisionsRejectsUnexpectedResponse(t *testing.T) {
	txnResp := &v3.TxnResponse{
		Responses: []*pb.ResponseOp{
			putResponseOp(),
		},
	}

	if _, err := txnPutObservedModRevisions(txnResp, 1); err == nil {
		t.Fatal("txnPutObservedModRevisions returned nil error for non-range response")
	}
}

func TestTxnPutRevisionStateAppliesMonotonicUpdates(t *testing.T) {
	state := newTxnPutRevisionState([]int64{5, 7})

	initial := state.snapshot([]int{0, 1})
	if initial[0] != 5 || initial[1] != 7 {
		t.Fatalf("initial snapshot = %v, want [5 7]", initial)
	}

	state.applyRevision([]int{0, 1}, 6)
	afterApply := state.snapshot([]int{0, 1})
	if afterApply[0] != 6 || afterApply[1] != 7 {
		t.Fatalf("after applyRevision snapshot = %v, want [6 7]", afterApply)
	}

	state.applyObservedRevisions([]int{0, 1}, []int64{4, 9})
	afterObserved := state.snapshot([]int{0, 1})
	if afterObserved[0] != 6 || afterObserved[1] != 9 {
		t.Fatalf("after applyObservedRevisions snapshot = %v, want [6 9]", afterObserved)
	}
}

func rangeResponseOp(modRevision int64) *pb.ResponseOp {
	rangeResp := &pb.RangeResponse{}
	if modRevision != 0 {
		rangeResp.Kvs = []*mvccpb.KeyValue{{ModRevision: modRevision}}
	}
	return &pb.ResponseOp{Response: &pb.ResponseOp_ResponseRange{ResponseRange: rangeResp}}
}

func putResponseOp() *pb.ResponseOp {
	return &pb.ResponseOp{Response: &pb.ResponseOp_ResponsePut{ResponsePut: &pb.PutResponse{}}}
}
