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

package storage

import (
	"testing"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

func TestCostTxn(t *testing.T) {
	putOp := func(key, value string) *pb.RequestOp {
		return &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{Key: []byte(key), Value: []byte(value)},
			},
		}
	}
	txnOp := func(txn *pb.TxnRequest) *pb.RequestOp {
		return &pb.RequestOp{
			Request: &pb.RequestOp_RequestTxn{RequestTxn: txn},
		}
	}

	tests := []struct {
		name string
		req  *pb.TxnRequest
		want int
	}{
		{
			name: "flat put",
			req: &pb.TxnRequest{
				Success: []*pb.RequestOp{putOp("foo", "bar")},
			},
			want: costPut(&pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}),
		},
		{
			name: "nested txn put in success branch must be counted",
			req: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					txnOp(&pb.TxnRequest{
						Success: []*pb.RequestOp{putOp("nested-key", "nested-value")},
					}),
				},
			},
			want: costPut(&pb.PutRequest{Key: []byte("nested-key"), Value: []byte("nested-value")}),
		},
		{
			name: "nested txn put in failure branch must be counted",
			req: &pb.TxnRequest{
				Failure: []*pb.RequestOp{
					txnOp(&pb.TxnRequest{
						Failure: []*pb.RequestOp{putOp("nested-key", "nested-value")},
					}),
				},
			},
			want: costPut(&pb.PutRequest{Key: []byte("nested-key"), Value: []byte("nested-value")}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, costTxn(tt.req))
		})
	}
}
