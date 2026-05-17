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

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestTxnRangeRequestConsistency(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := integration.ToGRPC(clus.Client(0)).KV

	_, err := kvc.Put(t.Context(), &pb.PutRequest{Key: []byte("key"), Value: []byte("val")})
	require.NoError(t, err)

	_, err = kvc.Compact(t.Context(), &pb.CompactionRequest{Revision: 2})
	require.NoError(t, err)

	txn := &pb.TxnRequest{
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: &pb.RangeRequest{
						Key:      []byte("key"),
						Revision: 1,
					},
				},
			},
			{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   []byte("should-not-exist"),
						Value: []byte("inconsistent"),
					},
				},
			},
		},
	}
	_, err = kvc.Txn(t.Context(), txn)
	require.ErrorContains(t, err, "mvcc: required revision has been compacted")

	for i := range clus.Members {
		resp, err := clus.Client(i).Get(t.Context(), "should-not-exist")
		require.NoError(t, err)
		require.Equalf(t, int64(0), resp.Count, "member %d should not have the key", i)
	}
}
