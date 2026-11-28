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
	"crypto/sha256"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/testing/protocmp"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

type testCase struct {
	name        string
	setup       testSetup
	op          *pb.RequestOp
	expectError string
}

type testSetup struct {
	compactRevision int64
	lease           int64
	key             []byte
}

var futureRev int64 = 1000

var rangeTestCases = []testCase{
	{
		name: "Range with revision 0 should succeed",
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestRange{
				RequestRange: &pb.RangeRequest{
					Revision: 0,
				},
			},
		},
	},
	{
		name: "Range on future rev should fail",
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestRange{
				RequestRange: &pb.RangeRequest{
					Revision: futureRev,
				},
			},
		},
		expectError: "mvcc: required revision is a future revision",
	},
	{
		name:  "Range on compacted rev should fail",
		setup: testSetup{compactRevision: 10},
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestRange{
				RequestRange: &pb.RangeRequest{
					Revision: 9,
				},
			},
		},
		expectError: "mvcc: required revision has been compacted",
	},
}

var putTestCases = []testCase{
	{
		name: "Put without lease should succeed",
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{},
			},
		},
	},
	{
		name: "Put with non-existing lease should fail",
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{
					Lease: 123,
				},
			},
		},
		expectError: "lease not found",
	},
	{
		name:  "Put with existing lease should succeed",
		setup: testSetup{lease: 123},
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{
					Lease: 123,
				},
			},
		},
	},
	{
		name: "Put with ignore value without previous key should fail",
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{
					IgnoreValue: true,
				},
			},
		},
		expectError: "etcdserver: key not found",
	},
	{
		name: "Put with ignore lease without previous key should fail",
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{
					IgnoreLease: true,
				},
			},
		},
		expectError: "etcdserver: key not found",
	},
	{
		name:  "Put with ignore value with previous key should succeeded",
		setup: testSetup{key: []byte("ignore-value")},
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{
					IgnoreValue: true,
					Key:         []byte("ignore-value"),
				},
			},
		},
	},
	{
		name:  "Put with ignore lease with previous key should succeed ",
		setup: testSetup{key: []byte("ignore-lease")},
		op: &pb.RequestOp{
			Request: &pb.RequestOp_RequestPut{
				RequestPut: &pb.PutRequest{
					IgnoreLease: true,
					Key:         []byte("ignore-lease"),
				},
			},
		},
	},
}

func TestCheckTxn(t *testing.T) {
	type txnTestCase struct {
		name        string
		setup       testSetup
		txn         *pb.TxnRequest
		expectError string
	}
	testCases := []txnTestCase{}
	for _, tc := range append(rangeTestCases, putTestCases...) {
		testCases = append(testCases, txnTestCase{
			name:  tc.name,
			setup: tc.setup,
			txn: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					tc.op,
				},
			},
			expectError: tc.expectError,
		})
	}
	invalidOperation := &pb.RequestOp{
		Request: &pb.RequestOp_RequestRange{
			RequestRange: &pb.RangeRequest{
				Revision: futureRev,
			},
		},
	}
	testCases = append(testCases, txnTestCase{
		name: "Invalid operation on failed path should succeed",
		txn: &pb.TxnRequest{
			Failure: []*pb.RequestOp{
				invalidOperation,
			},
		},
	})

	testCases = append(testCases, txnTestCase{
		name: "Invalid operation on subtransaction should fail",
		txn: &pb.TxnRequest{
			Success: []*pb.RequestOp{
				{
					Request: &pb.RequestOp_RequestTxn{
						RequestTxn: &pb.TxnRequest{
							Success: []*pb.RequestOp{
								invalidOperation,
							},
						},
					},
				},
			},
		},
		expectError: "mvcc: required revision is a future revision",
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, lessor := setup(t, tc.setup)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, _, err := Txn(ctx, zaptest.NewLogger(t), tc.txn, false, s, lessor, false)

			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if gotErr != tc.expectError {
				t.Errorf("Error not matching, got %q, expected %q", gotErr, tc.expectError)
			}
		})
	}
}

func TestCheckPut(t *testing.T) {
	for _, tc := range putTestCases {
		t.Run(tc.name, func(t *testing.T) {
			s, lessor := setup(t, tc.setup)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, _, err := Put(ctx, zaptest.NewLogger(t), lessor, s, tc.op.GetRequestPut())

			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if gotErr != tc.expectError {
				t.Errorf("Error not matching, got %q, expected %q", gotErr, tc.expectError)
			}
		})
	}
}

func TestCheckRange(t *testing.T) {
	for _, tc := range rangeTestCases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := setup(t, tc.setup)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, _, err := Range(ctx, zaptest.NewLogger(t), s, tc.op.GetRequestRange(), true)

			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if gotErr != tc.expectError {
				t.Errorf("Error not matching, got %q, expected %q", gotErr, tc.expectError)
			}
		})
	}
}

func setup(t *testing.T, setup testSetup) (mvcc.KV, lease.Lessor) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	t.Cleanup(func() {
		betesting.Close(t, b)
	})
	lessor := &lease.FakeLessor{LeaseSet: map[lease.LeaseID]struct{}{}}
	s := mvcc.NewStore(zaptest.NewLogger(t), b, lessor, mvcc.StoreConfig{})
	t.Cleanup(func() {
		s.Close()
	})

	if setup.compactRevision != 0 {
		for i := 0; int64(i) < setup.compactRevision; i++ {
			s.Put([]byte("a"), []byte("b"), 0)
		}
		s.Compact(traceutil.TODO(), setup.compactRevision)
	}
	if setup.lease != 0 {
		lessor.Grant(lease.LeaseID(setup.lease), 0)
	}
	if len(setup.key) != 0 {
		s.Put(setup.key, []byte("b"), 0)
	}
	return s, lessor
}

func TestSkippedRangeExecutionResponseHeaderRevision(t *testing.T) {
	tests := []struct {
		name string
		txn  *pb.TxnRequest
	}{
		{
			name: "range before write",
			txn: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{Key: []byte("foo")},
						},
					},
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")},
						},
					},
				},
			},
		},
		{
			name: "range after write",
			txn: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")},
						},
					},
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{Key: []byte("foo")},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			lg := zaptest.NewLogger(t)
			normalKV, normalLessor := setup(t, testSetup{})
			skippedKV, skippedLessor := setup(t, testSetup{})

			normalResp, _, err := Txn(ctx, lg, tt.txn, false, normalKV, normalLessor, false)
			require.NoError(t, err)
			skippedResp, _, err := Txn(ctx, lg, tt.txn, false, skippedKV, skippedLessor, true)
			require.NoError(t, err)

			var normalRangeHeader, skippedRangeHeader *pb.ResponseHeader
			for i, resp := range normalResp.Responses {
				rangeResp := resp.GetResponseRange()
				if rangeResp == nil {
					continue
				}
				normalRangeHeader = rangeResp.Header
				skippedRangeHeader = skippedResp.Responses[i].GetResponseRange().Header
				break
			}
			require.NotNil(t, normalRangeHeader)
			require.NotNil(t, skippedRangeHeader)
			require.Equal(t, normalRangeHeader.Revision, skippedRangeHeader.Revision)
		})
	}
}

func TestReadonlyTxnError(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup cancelled context
	ctx, cancel := context.WithCancel(t.Context())
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

	_, _, err := Txn(ctx, zaptest.NewLogger(t), txn, false, s, &lease.FakeLessor{}, false)
	if err == nil || !strings.Contains(err.Error(), "applyTxn: failed Range: rangeKeys: context cancelled: context canceled") {
		t.Fatalf("Expected context canceled error, got %v", err)
	}
}

// TestTxnReadOwnRevision checks the values and revisions visible inside a txn
// before and after it writes two different keys.
func TestTxnReadOwnRevision(t *testing.T) {
	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)
	lg := zaptest.NewLogger(t)
	lessor := &lease.FakeLessor{}
	s := mvcc.NewStore(lg, b, lessor, mvcc.StoreConfig{})
	defer s.Close()

	// Put foo at every revision from 2 to 10.
	for range 9 {
		s.Put([]byte("foo"), []byte("bar"), lease.NoLease)
	}
	require.Equal(t, int64(10), s.Rev())

	req := &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Key:         []byte("foo"),
				Target:      pb.Compare_MOD,
				Result:      pb.Compare_EQUAL,
				TargetUnion: &pb.Compare_ModRevision{ModRevision: 10},
			},
		},
		Success: []*pb.RequestOp{
			rangeOp("foo", "foo3"),
			putOp("foo2", "bar2"),
			putOp("foo", "bar1"),
			rangeOp("foo", "foo3", withSortAscend),
		},
	}
	resp, _, err := Txn(t.Context(), lg, req, false, s, lessor, false)
	require.NoError(t, err)
	require.True(t, resp.Succeeded)
	require.Equal(t, int64(11), resp.Header.Revision)
	require.Len(t, resp.Responses, 4)

	before := resp.Responses[0].GetResponseRange()
	require.Equal(t, int64(10), before.GetHeader().GetRevision())
	wantBefore := []*mvccpb.KeyValue{
		{Key: []byte("foo"), Value: []byte("bar"), CreateRevision: 2, ModRevision: 10, Version: 9},
	}
	require.Empty(t, cmp.Diff(wantBefore, before.Kvs, protocmp.Transform()))

	after := resp.Responses[3].GetResponseRange()
	require.Equal(t, int64(11), after.GetHeader().GetRevision())
	wantAfter := []*mvccpb.KeyValue{
		{Key: []byte("foo"), Value: []byte("bar1"), CreateRevision: 2, ModRevision: 11, Version: 10},
		{Key: []byte("foo2"), Value: []byte("bar2"), CreateRevision: 11, ModRevision: 11, Version: 1},
	}
	require.Empty(t, cmp.Diff(wantAfter, after.Kvs, protocmp.Transform()))
}

func TestWriteTxnPanicWithoutApply(t *testing.T) {
	b, bePath := betesting.NewDefaultTmpBackend(t)
	s := mvcc.NewStore(zaptest.NewLogger(t), b, &lease.FakeLessor{}, mvcc.StoreConfig{})
	defer s.Close()

	// setup cancelled context
	ctx, cancel := context.WithCancel(t.Context())
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

	// compute DB file hash before applying the txn
	dbHashBefore, err := computeFileHash(bePath)
	require.NoErrorf(t, err, "failed to compute DB file hash before txn")

	// we verify the following properties below:
	// 1. server panics after a write txn aply fails (invariant: server should never try to move on from a failed write)
	// 2. no writes from the txn are applied to the backend (invariant: failed write should have no side-effect on DB state besides panic)
	assert.Panicsf(t, func() { Txn(ctx, zaptest.NewLogger(t), txn, false, s, &lease.FakeLessor{}, false) }, "Expected panic in Txn with writes")
	dbHashAfter, err := computeFileHash(bePath)
	require.NoErrorf(t, err, "failed to compute DB file hash after txn")
	require.Equalf(t, dbHashBefore, dbHashAfter, "mismatch in DB hash before and after failed write txn")
}

func computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return string(h.Sum(nil)), nil
}

func rangeOp(key, end string, opts ...func(*pb.RangeRequest)) *pb.RequestOp {
	req := &pb.RangeRequest{Key: []byte(key), RangeEnd: []byte(end)}
	for _, opt := range opts {
		opt(req)
	}
	return &pb.RequestOp{Request: &pb.RequestOp_RequestRange{RequestRange: req}}
}

func withSortAscend(r *pb.RangeRequest) {
	r.SortOrder = pb.RangeRequest_ASCEND
}

func putOp(key, value string) *pb.RequestOp {
	req := &pb.PutRequest{Key: []byte(key), Value: []byte(value)}
	return &pb.RequestOp{Request: &pb.RequestOp_RequestPut{RequestPut: req}}
}
