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
	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/storage/backend"
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

type sizeOnlyBackend struct {
	backend.Backend
	size int64
}

func (b *sizeOnlyBackend) Size() int64 { return b.size }

func TestBackendQuotaFollowsCurrentBackend(t *testing.T) {
	const quotaBytes = int64(1024 * 1024)
	put := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}

	tests := []struct {
		name       string
		sizeBefore int64
		sizeAfter  int64
		wantBefore bool
		wantAfter  bool
	}{
		{
			name:       "full backend replaced by a small one",
			sizeBefore: quotaBytes,
			sizeAfter:  4096,
			wantBefore: false,
			wantAfter:  true,
		},
		{
			name:       "small backend replaced by a full one",
			sizeBefore: 4096,
			sizeAfter:  quotaBytes,
			wantBefore: true,
			wantAfter:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var current backend.Backend = &sizeOnlyBackend{size: tt.sizeBefore}
			q := NewBackendQuota(zaptest.NewLogger(t), quotaBytes, func() backend.Backend { return current }, "test")
			require.Equal(t, tt.wantBefore, q.Available(put))

			current = &sizeOnlyBackend{size: tt.sizeAfter}
			require.Equal(t, tt.wantAfter, q.Available(put))
			require.Equal(t, quotaBytes-tt.sizeAfter, q.Remaining())
		})
	}
}
