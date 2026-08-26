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

package correctness

import (
	"testing"

	"github.com/anishathalye/porcupine"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func TestStorageModel(t *testing.T) {
	allScenarios := append(append([]modelTestCase(nil), commonModelScenarios...), negativeModelScenarios...)
	for _, tc := range allScenarios {
		t.Run(tc.name, func(t *testing.T) {
			testStorageModel(t, tc.operations)
		})
	}
}

func testStorageModel(t *testing.T, operations []modelTestOp) {
	state := StoragePorcupineModel.Init()
	for opIdx, op := range operations {
		ok, newState := StoragePorcupineModel.Step(state, op.req, op.res)
		if op.expectFailure {
			require.Falsef(t, ok, "Op #%d (%s) expected to fail but succeeded",
				opIdx, StoragePorcupineModel.DescribeOperation(op.req, op.res))
			continue
		}
		require.Truef(t, ok, "Op #%d (%s) failed unexpectedly on state: %s",
			opIdx, StoragePorcupineModel.DescribeOperation(op.req, op.res), StoragePorcupineModel.DescribeState(state))
		state = newState
	}
}

func TestStorageReplayRejectsCorruptedWriteRevisions(t *testing.T) {
	t.Run("duplicate write revision", func(t *testing.T) {
		ops := []porcupine.Operation{
			{
				ClientId: 0,
				Input:    StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				Output:   StorageResponse{Rev: 2},
			},
			{
				ClientId: 1,
				Input:    StorageRequest{Op: OpPut, Key: []byte("k2"), Value: []byte("v2")},
				Output:   StorageResponse{Rev: 2},
			},
		}
		_, err := NewStorageReplay(ops)
		require.ErrorContains(t, err, "non-consecutive or duplicate write revision")
	})

	t.Run("gapped write revision", func(t *testing.T) {
		ops := []porcupine.Operation{
			{
				ClientId: 0,
				Input:    StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				Output:   StorageResponse{Rev: 5},
			},
		}
		_, err := NewStorageReplay(ops)
		require.ErrorContains(t, err, "non-consecutive or duplicate write revision")
	})
}

func TestWatchValidationRejectsBrokenStreams(t *testing.T) {
	ops := []porcupine.Operation{
		{
			ClientId: 0,
			Input: StorageRequest{
				Op: OpTxn,
				TxnOps: []TxnSubOp{
					{Type: TxnSubOpPut, Key: []byte("k1"), Value: []byte("v1")},
					{Type: TxnSubOpPut, Key: []byte("k2"), Value: []byte("v2")},
				},
			},
			Output: StorageResponse{Rev: 2},
		},
		{
			ClientId: 0,
			Input:    StorageRequest{Op: OpDelete, Key: []byte("k1")},
			Output:   StorageResponse{Rev: 3, Deleted: 1},
		},
	}
	replay, err := NewStorageReplay(ops)
	require.NoError(t, err)

	t.Run("dropped trailing watch event fails RequireExact", func(t *testing.T) {
		err := checkSingleWatchStream(0, replay, RecordedWatchStream{
			WatchID:      1,
			Key:          []byte("k0"),
			End:          []byte("k9"),
			StartRev:     1,
			RequireExact: true,
			Responses: []mvcc.WatchResponse{
				{
					WatchID:  1,
					Revision: 2,
					Events: []*mvccpb.Event{
						{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2, Version: 1}},
						{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("k2"), Value: []byte("v2"), CreateRevision: 2, ModRevision: 2, Version: 1}},
					},
				},
			},
		})
		require.ErrorContains(t, err, "exact event count mismatch")
	})

	t.Run("split TxnWrite ModRevision across WatchResponses fails", func(t *testing.T) {
		err := checkSingleWatchStream(0, replay, RecordedWatchStream{
			WatchID:      1,
			Key:          []byte("k0"),
			End:          []byte("k9"),
			StartRev:     1,
			RequireExact: false,
			Responses: []mvcc.WatchResponse{
				{
					WatchID:  1,
					Revision: 2,
					Events: []*mvccpb.Event{
						{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2, Version: 1}},
					},
				},
				{
					WatchID:  1,
					Revision: 2,
					Events: []*mvccpb.Event{
						{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("k2"), Value: []byte("v2"), CreateRevision: 2, ModRevision: 2, Version: 1}},
					},
				},
			},
		})
		require.ErrorContains(t, err, "split ModRevision")
	})

	t.Run("non-zero DELETE event Kv fields fail", func(t *testing.T) {
		err := checkSingleWatchStream(0, replay, RecordedWatchStream{
			WatchID:      1,
			Key:          []byte("k0"),
			End:          []byte("k9"),
			StartRev:     1,
			RequireExact: true,
			Responses: []mvcc.WatchResponse{
				{
					WatchID:  1,
					Revision: 2,
					Events: []*mvccpb.Event{
						{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2, Version: 1}},
						{Type: mvccpb.Event_PUT, Kv: &mvccpb.KeyValue{Key: []byte("k2"), Value: []byte("v2"), CreateRevision: 2, ModRevision: 2, Version: 1}},
					},
				},
				{
					WatchID:  1,
					Revision: 3,
					Events: []*mvccpb.Event{
						{Type: mvccpb.Event_DELETE, Kv: &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("stale"), ModRevision: 3, Version: 1}},
					},
				},
			},
		})
		require.ErrorContains(t, err, "zeroed Value/CreateRevision/Version/Lease")
	})
}

func TestStorageStateEqual(t *testing.T) {
	testCases := []struct {
		name  string
		s1    *StorageState
		s2    *StorageState
		equal bool
	}{
		{
			name:  "fresh states are equal",
			s1:    newStorageState(),
			s2:    newStorageState(),
			equal: true,
		},
		{
			name: "different MVCC items are not equal",
			s1: func() *StorageState {
				s := newStorageState()
				s.items["k1"] = &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1")}
				return s
			}(),
			s2:    newStorageState(),
			equal: false,
		},
		{
			name: "different MVCC leases are not equal",
			s1: func() *StorageState {
				s := newStorageState()
				s.items["k1"] = &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1"), Lease: 101}
				return s
			}(),
			s2: func() *StorageState {
				s := newStorageState()
				s.items["k1"] = &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1"), Lease: 102}
				return s
			}(),
			equal: false,
		},
		{
			name: "matching MVCC items are equal",
			s1: func() *StorageState {
				s := newStorageState()
				s.items["k1"] = &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1"), Lease: 101}
				return s
			}(),
			s2: func() *StorageState {
				s := newStorageState()
				s.items["k1"] = &mvccpb.KeyValue{Key: []byte("k1"), Value: []byte("v1"), Lease: 101}
				return s
			}(),
			equal: true,
		},
		{
			name: "different backend buckets are not equal",
			s1: func() *StorageState {
				s := newStorageState()
				s.backendBuckets["meta"] = map[string][]byte{"idx": []byte("1")}
				return s
			}(),
			s2:    newStorageState(),
			equal: false,
		},
		{
			name: "matching backend buckets are equal",
			s1: func() *StorageState {
				s := newStorageState()
				s.backendBuckets["meta"] = map[string][]byte{"idx": []byte("1")}
				return s
			}(),
			s2: func() *StorageState {
				s := newStorageState()
				s.backendBuckets["meta"] = map[string][]byte{"idx": []byte("1")}
				return s
			}(),
			equal: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.equal, StoragePorcupineModel.Equal(tc.s1, tc.s2))
		})
	}
}

type modelTestOp struct {
	req           StorageRequest
	res           StorageResponse
	expectFailure bool
}

type modelTestCase struct {
	name       string
	operations []modelTestOp
}

var commonModelScenarios = []modelTestCase{
	{
		name: "MVCC FirstRev, Rev, Put, Get latest and negative rev",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpFirstRev},
				res: StorageResponse{FirstRev: -1},
			},
			{
				req: StorageRequest{Op: OpRev},
				res: StorageResponse{Rev: 1},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1"), Rev: 0},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2, Version: 1},
					},
					Count: 1,
					Rev:   2,
				},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1"), Rev: -1},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2, Version: 1},
					},
					Count: 1,
					Rev:   2,
				},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k2"), Rev: 0},
				res: StorageResponse{KVs: nil, Count: 0, Rev: 2},
			},
		},
	},
	{
		name: "MVCC Put, Update, RangeOptions matrix and no-op Delete",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1"), Lease: 101},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k2"), Value: []byte("v2"), Lease: 102},
				res: StorageResponse{Rev: 3},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1-updated"), Lease: 103},
				res: StorageResponse{Rev: 4},
			},
			{
				// Limit=1 without WithTotalCount caps Count to 1.
				req: StorageRequest{Op: OpRange, Key: []byte("k1"), End: []byte("k3"), Limit: 1, WithTotalCount: false},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v1-updated"), CreateRevision: 2, ModRevision: 4, Version: 2, Lease: 103},
					},
					Count: 1,
					Rev:   4,
				},
			},
			{
				// Limit=1 with WithTotalCount=true reports full Count=2.
				req: StorageRequest{Op: OpRange, Key: []byte("k1"), End: []byte("k3"), Limit: 1, WithTotalCount: true},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v1-updated"), CreateRevision: 2, ModRevision: 4, Version: 2, Lease: 103},
					},
					Count: 2,
					Rev:   4,
				},
			},
			{
				// FastKeysOnly=true omits Value and Lease while keeping revision metadata.
				req: StorageRequest{Op: OpRange, Key: []byte("k1"), End: []byte("k3"), FastKeysOnly: true, WithTotalCount: true},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), CreateRevision: 2, ModRevision: 4, Version: 2},
						{Key: []byte("k2"), CreateRevision: 3, ModRevision: 3, Version: 1},
					},
					Count: 2,
					Rev:   4,
				},
			},
			{
				// Empty range key >= end returns empty KVs and Count=0.
				req: StorageRequest{Op: OpRange, Key: []byte("k3"), End: []byte("k1")},
				res: StorageResponse{KVs: nil, Count: 0, Rev: 4},
			},
			{
				// Deleting non-existent key preserves currentRev=4 and Deleted=0.
				req: StorageRequest{Op: OpDelete, Key: []byte("missing")},
				res: StorageResponse{Rev: 4, Deleted: 0},
			},
			{
				req: StorageRequest{Op: OpDelete, Key: []byte("k1")},
				res: StorageResponse{Rev: 5, Deleted: 1},
			},
			{
				req: StorageRequest{Op: OpReadTxn, Key: []byte("k1"), End: []byte("k3")},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k2"), Value: []byte("v2"), CreateRevision: 3, ModRevision: 3, Version: 1, Lease: 102},
					},
					Count:    1,
					Rev:      5,
					FirstRev: -1,
				},
			},
		},
	},
	{
		name: "MVCC Intra-TxnWrite same-key transitions (Put->Put, Delete->Put, Put->Delete)",
		operations: []modelTestOp{
			{
				// Put->Put on k1 in one TxnWrite bumps Version to 2 at CreateRevision=2, ModRevision=2.
				req: StorageRequest{
					Op: OpTxn,
					TxnOps: []TxnSubOp{
						{Type: TxnSubOpPut, Key: []byte("k1"), Value: []byte("v1")},
						{Type: TxnSubOpPut, Key: []byte("k1"), Value: []byte("v2")},
					},
				},
				res: StorageResponse{Rev: 2, Deleted: 0},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1")},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v2"), CreateRevision: 2, ModRevision: 2, Version: 2},
					},
					Count: 1,
					Rev:   2,
				},
			},
			{
				// Delete->Put on k1 in one TxnWrite resets CreateRevision=3, Version=1.
				req: StorageRequest{
					Op: OpTxn,
					TxnOps: []TxnSubOp{
						{Type: TxnSubOpDeleteRange, Key: []byte("k1")},
						{Type: TxnSubOpPut, Key: []byte("k1"), Value: []byte("v3")},
					},
				},
				res: StorageResponse{Rev: 3, Deleted: 1},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1")},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v3"), CreateRevision: 3, ModRevision: 3, Version: 1},
					},
					Count: 1,
					Rev:   3,
				},
			},
			{
				// Put->Delete on fresh k2 in one TxnWrite advances currentRev to 4 despite net-empty k2.
				req: StorageRequest{
					Op: OpTxn,
					TxnOps: []TxnSubOp{
						{Type: TxnSubOpPut, Key: []byte("k2"), Value: []byte("temp")},
						{Type: TxnSubOpDeleteRange, Key: []byte("k2")},
					},
				},
				res: StorageResponse{Rev: 4, Deleted: 1},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k2")},
				res: StorageResponse{KVs: nil, Count: 0, Rev: 4},
			},
		},
	},
	{
		name: "MVCC Lease attach and Lessor Revoke",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpPut, Key: []byte("lk1"), Value: []byte("lv1"), Lease: RevocableModelLeaseID},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("lk2"), Value: []byte("lv2"), Lease: RevocableModelLeaseID},
				res: StorageResponse{Rev: 3},
			},
			{
				req: StorageRequest{Op: OpLeaseRevoke, Lease: RevocableModelLeaseID},
				res: StorageResponse{Rev: 4, Deleted: 2},
			},
			{
				req: StorageRequest{Op: OpRange, Key: []byte("lk1"), End: []byte("lk9")},
				res: StorageResponse{KVs: nil, Count: 0, Rev: 4},
			},
		},
	},
	{
		name: "Backend Put, Range and Delete across buckets",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpBackendPut, Bucket: "authUsers", Key: []byte("bkey1"), Value: []byte("bval1")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendPut, Bucket: "alarm", Key: []byte("akey1"), Value: []byte("aval1")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendRange, Bucket: "authUsers", Key: []byte("bkey1"), Limit: 1},
				res: StorageResponse{
					BackendKeys: [][]byte{[]byte("bkey1")},
					BackendVals: [][]byte{[]byte("bval1")},
				},
			},
			{
				req: StorageRequest{Op: OpBackendRange, Bucket: "alarm", Key: []byte("akey1"), Limit: 1},
				res: StorageResponse{
					BackendKeys: [][]byte{[]byte("akey1")},
					BackendVals: [][]byte{[]byte("aval1")},
				},
			},
			{
				req: StorageRequest{Op: OpBackendDelete, Bucket: "authUsers", Key: []byte("bkey1")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendRange, Bucket: "authUsers", Key: []byte("bkey1"), Limit: 1},
				res: StorageResponse{BackendKeys: nil, BackendVals: nil},
			},
			{
				req: StorageRequest{Op: OpBackendRange, Bucket: "alarm", Key: []byte("akey1"), Limit: 1},
				res: StorageResponse{
					BackendKeys: [][]byte{[]byte("akey1")},
					BackendVals: [][]byte{[]byte("aval1")},
				},
			},
		},
	},
	{
		name: "Backend ForEach, Commit and Hashes",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpBackendPut, Bucket: "alarm", Key: []byte("k1"), Value: []byte("v1")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendPut, Bucket: "alarm", Key: []byte("k2"), Value: []byte("v2")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendPut, Bucket: "alarm", Key: []byte("k1"), Value: []byte("v1-updated")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpCommit},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendForEach, Bucket: "alarm"},
				res: StorageResponse{
					BackendKeys: [][]byte{[]byte("k1"), []byte("k2")},
					BackendVals: [][]byte{[]byte("v1-updated"), []byte("v2")},
				},
			},
			{
				req: StorageRequest{Op: OpBackendHash},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpHash},
				res: StorageResponse{Rev: 1},
			},
			{
				req: StorageRequest{Op: OpHashByRev, Rev: 0},
				res: StorageResponse{Rev: 1, HashRev: 1, CompactRev: -1},
			},
		},
	},
	{
		name: "Cross-Layer Transaction (MVCC + Backend Meta)",
		operations: []modelTestOp{
			{
				req: StorageRequest{
					Op:      OpCrossLayerTxn,
					Bucket:  "meta",
					Key:     []byte("k1"),
					Value:   []byte("v1"),
					MetaKey: []byte("consistent_index"),
					MetaVal: []byte("0000000000000002"),
				},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1"), Rev: 0},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2, Version: 1},
					},
					Count: 1,
					Rev:   2,
				},
			},
			{
				req: StorageRequest{Op: OpBackendRange, Bucket: "meta", Key: []byte("consistent_index"), Limit: 1},
				res: StorageResponse{
					BackendKeys: [][]byte{[]byte("consistent_index")},
					BackendVals: [][]byte{[]byte("0000000000000002")},
				},
			},
		},
	},
	{
		name: "Compact(0), compaction progression, FirstRev and HashByRev errors",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpCompact, Rev: 0},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpCompact, Rev: 0},
				res: StorageResponse{Err: mvcc.ErrCompacted},
			},
			{
				req: StorageRequest{Op: OpFirstRev},
				res: StorageResponse{FirstRev: 0},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v2")},
				res: StorageResponse{Rev: 3},
			},
			{
				req: StorageRequest{Op: OpCompact, Rev: 2},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpFirstRev},
				res: StorageResponse{FirstRev: 2},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1"), Rev: 1},
				res: StorageResponse{Count: -1, Rev: 0, Err: mvcc.ErrCompacted},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1"), Rev: 10},
				res: StorageResponse{Count: -1, Rev: 3, Err: mvcc.ErrFutureRev},
			},
			{
				req: StorageRequest{Op: OpHashByRev, Rev: 1},
				res: StorageResponse{Err: mvcc.ErrCompacted},
			},
			{
				req: StorageRequest{Op: OpHashByRev, Rev: 10},
				res: StorageResponse{Rev: 3, Err: mvcc.ErrFutureRev},
			},
		},
	},
}

var negativeModelScenarios = []modelTestCase{
	{
		name: "Model rejects wrong MVCC value",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1"), Rev: 0},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("wrong-val"), CreateRevision: 2, ModRevision: 2, Version: 1},
					},
					Count: 1,
					Rev:   2,
				},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects wrong write revision on Put",
		operations: []modelTestOp{
			{
				req:           StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				res:           StorageResponse{Rev: 42},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects stale ReadTxn snapshot revision",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				res: StorageResponse{Rev: 2},
			},
			{
				req:           StorageRequest{Op: OpReadTxn, Key: []byte("k1")},
				res:           StorageResponse{KVs: nil, Count: 0, Rev: 1, FirstRev: -1},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects ErrCompacted on latest read (Rev == 0)",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpCompact, Rev: 1},
				res: StorageResponse{},
			},
			{
				req:           StorageRequest{Op: OpGet, Key: []byte("k1"), Rev: 0},
				res:           StorageResponse{Err: mvcc.ErrCompacted},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects wrong RangeResult.Count under Limit and WithTotalCount",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1")},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k2"), Value: []byte("v2")},
				res: StorageResponse{Rev: 3},
			},
			{
				req: StorageRequest{Op: OpRange, Key: []byte("k1"), End: []byte("k3"), Limit: 1, WithTotalCount: false},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						{Key: []byte("k1"), Value: []byte("v1"), CreateRevision: 2, ModRevision: 2, Version: 1},
					},
					Count: 2, // Must be capped to 1 when WithTotalCount == false!
					Rev:   3,
				},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects wrong CreateRevision, Version or Lease on overwrite",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v1"), Lease: 101},
				res: StorageResponse{Rev: 2},
			},
			{
				req: StorageRequest{Op: OpPut, Key: []byte("k1"), Value: []byte("v2"), Lease: 102},
				res: StorageResponse{Rev: 3},
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1")},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						// Wrong CreateRevision (3 instead of 2)
						{Key: []byte("k1"), Value: []byte("v2"), CreateRevision: 3, ModRevision: 3, Version: 2, Lease: 102},
					},
					Count: 1,
					Rev:   3,
				},
				expectFailure: true,
			},
			{
				req: StorageRequest{Op: OpGet, Key: []byte("k1")},
				res: StorageResponse{
					KVs: []*mvccpb.KeyValue{
						// Wrong Lease (101 instead of 102)
						{Key: []byte("k1"), Value: []byte("v2"), CreateRevision: 2, ModRevision: 3, Version: 2, Lease: 101},
					},
					Count: 1,
					Rev:   3,
				},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects non-Key BackendRange with multi-key Limit",
		operations: []modelTestOp{
			{
				req:           StorageRequest{Op: OpBackendRange, Bucket: "alarm", Key: []byte("a1"), End: []byte("a9"), Limit: 0},
				res:           StorageResponse{},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects wrong backend value",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpBackendPut, Bucket: "alarm", Key: []byte("bkey1"), Value: []byte("bval1")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendRange, Bucket: "alarm", Key: []byte("bkey1"), Limit: 1},
				res: StorageResponse{
					BackendKeys: [][]byte{[]byte("bkey1")},
					BackendVals: [][]byte{[]byte("bval-wrong")},
				},
				expectFailure: true,
			},
		},
	},
	{
		name: "Model rejects observing deleted backend key",
		operations: []modelTestOp{
			{
				req: StorageRequest{Op: OpBackendPut, Bucket: "alarm", Key: []byte("bkey1"), Value: []byte("bval1")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendDelete, Bucket: "alarm", Key: []byte("bkey1")},
				res: StorageResponse{},
			},
			{
				req: StorageRequest{Op: OpBackendRange, Bucket: "alarm", Key: []byte("bkey1"), Limit: 1},
				res: StorageResponse{
					BackendKeys: [][]byte{[]byte("bkey1")},
					BackendVals: [][]byte{[]byte("bval1")},
				},
				expectFailure: true,
			},
		},
	},
}
