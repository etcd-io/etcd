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

package v3rpc

import (
	"context"
	"testing"

	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	txn "go.etcd.io/etcd/server/v3/etcdserver/txn"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func FuzzTxnRangeRequest(f *testing.F) {
	testcases := []pb.RangeRequest{
		{
			Key:        []byte{2},
			RangeEnd:   []byte{2},
			Limit:      3,
			Revision:   3,
			SortOrder:  2,
			SortTarget: 2,
		},
	}

	for _, tc := range testcases {
		soValue := pb.RangeRequest_SortOrder_value[tc.SortOrder.String()]
		soTarget := pb.RangeRequest_SortTarget_value[tc.SortTarget.String()]
		f.Add(tc.Key, tc.RangeEnd, tc.Limit, tc.Revision, soValue, soTarget)
	}

	f.Fuzz(func(t *testing.T,
		key []byte,
		rangeEnd []byte,
		limit int64,
		revision int64,
		sortOrder int32,
		sortTarget int32,
	) {
		fuzzRequest := &pb.RangeRequest{
			Key:        key,
			RangeEnd:   rangeEnd,
			Limit:      limit,
			SortOrder:  pb.RangeRequest_SortOrder(sortOrder),
			SortTarget: pb.RangeRequest_SortTarget(sortTarget),
		}

		verifyCheck(t, func() error {
			return checkRangeRequest(fuzzRequest)
		})

		execTransaction(t, &pb.RequestOp{
			Request: &pb.RequestOp_RequestRange{
				RequestRange: fuzzRequest,
			},
		})
	})
}

func FuzzTxnPutRequest(f *testing.F) {
	testcases := []pb.PutRequest{
		{
			Key:         []byte{2},
			Value:       []byte{2},
			Lease:       2,
			PrevKv:      false,
			IgnoreValue: false,
			IgnoreLease: false,
		},
	}

	for _, tc := range testcases {
		f.Add(tc.Key, tc.Value, tc.Lease, tc.PrevKv, tc.IgnoreValue, tc.IgnoreLease)
	}

	f.Fuzz(func(t *testing.T,
		key []byte,
		value []byte,
		leaseValue int64,
		prevKv bool,
		ignoreValue bool,
		IgnoreLease bool,
	) {
		fuzzRequest := &pb.PutRequest{
			Key:         key,
			Value:       value,
			Lease:       leaseValue,
			PrevKv:      prevKv,
			IgnoreValue: ignoreValue,
			IgnoreLease: IgnoreLease,
		}

		verifyCheck(t, func() error {
			return checkPutRequest(fuzzRequest)
		})

		execTransaction(t, &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: fuzzRequest,
			},
		})
	})
}

func FuzzTxnDeleteRangeRequest(f *testing.F) {
	testcases := []pb.DeleteRangeRequest{
		{
			Key:      []byte{2},
			RangeEnd: []byte{2},
			PrevKv:   false,
		},
	}

	for _, tc := range testcases {
		f.Add(tc.Key, tc.RangeEnd, tc.PrevKv)
	}

	f.Fuzz(func(t *testing.T,
		key []byte,
		rangeEnd []byte,
		prevKv bool,
	) {
		fuzzRequest := &pb.DeleteRangeRequest{
			Key:      key,
			RangeEnd: rangeEnd,
			PrevKv:   prevKv,
		}

		verifyCheck(t, func() error {
			return checkDeleteRequest(fuzzRequest)
		})

		execTransaction(t, &pb.RequestOp{
			Request: &pb.RequestOp_RequestDeleteRange{
				RequestDeleteRange: fuzzRequest,
			},
		})
	})
}

func verifyCheck(t *testing.T, check func() error) {
	errCheck := check()
	if errCheck != nil {
		t.Skip("Validation not passing. Skipping the apply.")
	}
}

func execTransaction(t *testing.T, req *pb.RequestOp) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup cancelled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := &pb.TxnRequest{
		Success: []*pb.RequestOp{req},
	}

	_, _, err := txn.Txn(ctx, zaptest.NewLogger(t), request, false, s, &lease.FakeLessor{})
	if err != nil {
		t.Skipf("Application erroring. %s", err.Error())
	}
}
