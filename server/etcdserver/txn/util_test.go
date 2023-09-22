// Copyright 2022 The etcd Authors
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

package txn

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

// TestWarnOfExpensiveReadOnlyTxnRequest verifies WarnOfExpensiveReadOnlyTxnRequest
// never panic no matter what data the txnResponse contains.
func TestWarnOfExpensiveReadOnlyTxnRequest(t *testing.T) {
	kvs := []*mvccpb.KeyValue{
		{Key: []byte("k1"), Value: []byte("v1")},
		{Key: []byte("k2"), Value: []byte("v2")},
	}

	testCases := []struct {
		name    string
		txnResp *pb.TxnResponse
	}{
		{
			name: "all readonly responses",
			txnResp: &pb.TxnResponse{
				Responses: []*pb.ResponseOp{
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: &pb.RangeResponse{
								Kvs: kvs,
							},
						},
					},
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: &pb.RangeResponse{},
						},
					},
				},
			},
		},
		{
			name: "all readonly responses with partial nil responses",
			txnResp: &pb.TxnResponse{
				Responses: []*pb.ResponseOp{
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: &pb.RangeResponse{},
						},
					},
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: nil,
						},
					},
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: &pb.RangeResponse{
								Kvs: kvs,
							},
						},
					},
				},
			},
		},
		{
			name: "all readonly responses with all nil responses",
			txnResp: &pb.TxnResponse{
				Responses: []*pb.ResponseOp{
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: nil,
						},
					},
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: nil,
						},
					},
				},
			},
		},
		{
			name: "partial non readonly responses",
			txnResp: &pb.TxnResponse{
				Responses: []*pb.ResponseOp{
					{
						Response: &pb.ResponseOp_ResponseRange{
							ResponseRange: nil,
						},
					},
					{
						Response: &pb.ResponseOp_ResponsePut{},
					},
					{
						Response: &pb.ResponseOp_ResponseDeleteRange{},
					},
				},
			},
		},
		{
			name: "all non readonly responses",
			txnResp: &pb.TxnResponse{
				Responses: []*pb.ResponseOp{
					{
						Response: &pb.ResponseOp_ResponsePut{},
					},
					{
						Response: &pb.ResponseOp_ResponseDeleteRange{},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			start := time.Now().Add(-1 * time.Second)
			// WarnOfExpensiveReadOnlyTxnRequest shouldn't panic.
			WarnOfExpensiveReadOnlyTxnRequest(lg, 0, start, &pb.TxnRequest{}, tc.txnResp, nil)
		})
	}
}
