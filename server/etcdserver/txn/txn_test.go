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
	"context"
	"strings"
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.uber.org/zap/zaptest"

	"github.com/stretchr/testify/assert"
)

func TestReadonlyTxnError(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup cancelled context
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	// put some data to prevent early termination in rangeKeys
	// we are expecting failure on cancelled context check
	s.Put([]byte("foo"), []byte("bar"), lease.NoLease)

	txn := &pb.TxnRequest{
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: &pb.RangeRequest{
						Key: []byte("foo"),
					},
				},
			},
		},
	}

	_, _, err := Txn(ctx, zaptest.NewLogger(t), txn, false, s, &lease.FakeLessor{})
	if err == nil || !strings.Contains(err.Error(), "applyTxn: failed Range: rangeKeys: context cancelled: context canceled") {
		t.Fatalf("Expected context canceled error, got %v", err)
	}
}

func TestWriteTxnPanic(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup cancelled context
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	// write txn that puts some data and then fails in range due to cancelled context
	txn := &pb.TxnRequest{
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestPut{
					RequestPut: &pb.PutRequest{
						Key:   []byte("foo"),
						Value: []byte("bar"),
					},
				},
			},
			{
				Request: &pb.RequestOp_RequestRange{
					RequestRange: &pb.RangeRequest{
						Key: []byte("foo"),
					},
				},
			},
		},
	}

	assert.Panics(t, func() { Txn(ctx, zaptest.NewLogger(t), txn, false, s, &lease.FakeLessor{}) }, "Expected panic in Txn with writes")
}
