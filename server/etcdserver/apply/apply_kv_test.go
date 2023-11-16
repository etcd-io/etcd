// Copyright 2023 The etcd Authors
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

package apply

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/crypto/bcrypt"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3alarm"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/etcdserver/errors"
	"go.etcd.io/etcd/server/v3/lease"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

const (
	putInherentCost = 256
	key2            = "key2"
	key3            = "key3"
	key4            = "key4"
	value64         = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789?!"
	value128        = value64 + value64
)

var (
	fourKeys = []pb.PutRequest{
		{Key: []byte(key), Value: []byte("key's value")},
		{Key: []byte(key2), Value: []byte("key2's value")},
		{Key: []byte(key3), Value: []byte("key3's value")},
		{Key: []byte(key4), Value: []byte("key4's value")},
	}
	updatedValue  = []byte("up2date value")
	updatedValue2 = []byte("up2date value2")
	zero          = []byte{0}
)

func putKeys(ctx context.Context, t *testing.T, keys []pb.PutRequest, app *applierV3backend) {
	var prevRev int64
	for _, k := range keys {
		resp, _, err := app.Put(ctx, &k)
		require.NoError(t, err)
		if prevRev > 0 {
			require.Greater(t, resp.Header.Revision, prevRev)
			prevRev = resp.Header.Revision
		}
	}
}

func defaultApplierV3Backend(t *testing.T) (*applierV3backend, backend.Backend) {
	lg := zaptest.NewLogger(t)
	be, _ := betesting.NewDefaultTmpBackend(t)
	cluster := membership.NewCluster(lg)
	lessor := lease.NewLessor(lg, be, cluster, lease.LessorConfig{})
	kv := mvcc.NewStore(lg, be, lessor, mvcc.StoreConfig{})
	t.Cleanup(func() {
		kv.Close()
		be.Close()
	})

	alarmStore, err := v3alarm.NewAlarmStore(lg, schema.NewAlarmBackend(lg, be))
	require.NoError(t, err)

	tp, err := auth.NewTokenProvider(lg, "simple", dummyIndexWaiter, 300*time.Second)
	require.NoError(t, err)

	authStore := auth.NewAuthStore(
		lg,
		schema.NewAuthBackend(lg, be),
		tp,
		bcrypt.DefaultCost,
	)
	consistentIndex := cindex.NewConsistentIndex(be)

	return &applierV3backend{
		lg:                           lg,
		kv:                           kv,
		alarmStore:                   alarmStore,
		authStore:                    authStore,
		lessor:                       lessor,
		cluster:                      cluster,
		raftStatus:                   &fakeRaftStatusGetter{},
		snapshotServer:               &fakeSnapshotServer{},
		consistentIndex:              consistentIndex,
		txnModeWriteWithSharedBuffer: false,
	}, be
}

func TestBackendApply(t *testing.T) {
	appV3be, _ := defaultApplierV3Backend(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := appV3be.Apply(ctx, &pb.InternalRaftRequest{}, true, dummyApplyFunc)
	require.Same(t, &emptyResult, result)
}

func TestBackendPutRangeDelete(t *testing.T) {
	appV3be, _ := defaultApplierV3Backend(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	putKeys(ctx, t, fourKeys, appV3be)

	// Single key value correctness
	respRange, err := appV3be.Range(ctx, &pb.RangeRequest{Key: []byte(key)})
	require.NoError(t, err)
	require.EqualValues(t, fourKeys[0].Value, respRange.Kvs[0].Value)
	keyModRevision := respRange.Kvs[0].ModRevision

	// test keys[1:] values correctness
	respRange, err = appV3be.Range(ctx, &pb.RangeRequest{
		Key:      fourKeys[1].Key,
		RangeEnd: zero,
	})
	require.NoError(t, err)
	require.EqualValues(t, 3, respRange.Count)
	for i := 1; i <= 3; i++ {
		require.EqualValues(t, fourKeys[i].Value, respRange.Kvs[i-1].Value)
	}

	// Updating value returning previous/overwritten KV
	respPut, _, err := appV3be.Put(ctx, &pb.PutRequest{
		Key:    []byte(key),
		Value:  updatedValue,
		PrevKv: true,
	})
	require.NoError(t, err)
	require.EqualValues(t, respPut.PrevKv.Value, fourKeys[0].Value)
	require.EqualValues(t, keyModRevision, respPut.PrevKv.ModRevision)

	respRange, err = appV3be.Range(ctx, &pb.RangeRequest{Key: []byte(key)})
	require.NoError(t, err)
	require.Greater(t, respRange.Kvs[0].ModRevision, keyModRevision)
	require.EqualValues(t, updatedValue, respRange.Kvs[0].Value)
	keyModRevision = respRange.Kvs[0].ModRevision

	// IgnoreValue should update ModRevision without updating Value
	respPut, _, err = appV3be.Put(ctx, &pb.PutRequest{
		Key:         fourKeys[0].Key,
		Value:       []byte("ignore me"),
		IgnoreValue: true,
	})
	require.NoError(t, err)
	require.Greater(t, respPut.Header.Revision, keyModRevision)

	respRange, err = appV3be.Range(ctx, &pb.RangeRequest{Key: []byte(key)})
	require.NoError(t, err)
	require.EqualValues(t, updatedValue, respRange.Kvs[0].Value)
	require.Greater(t, respRange.Kvs[0].ModRevision, keyModRevision)

	// delete first 3 keys in a row returning deleted KV
	respDelete, err := appV3be.DeleteRange(&pb.DeleteRangeRequest{
		Key:      []byte(key),
		RangeEnd: []byte(key4),
		PrevKv:   true,
	})
	require.NoError(t, err)
	require.EqualValues(t, 3, respDelete.Deleted)
	require.EqualValues(t, updatedValue, respDelete.PrevKvs[0].Value)
	require.Len(t, respDelete.PrevKvs, 3)

	// delete last key
	respDelete, err = appV3be.DeleteRange(&pb.DeleteRangeRequest{
		Key: []byte(key4),
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, respDelete.Deleted)

	// deleting non-existing key generates no error, just `Deleted: 0` response
	respDelete, err = appV3be.DeleteRange(&pb.DeleteRangeRequest{
		Key: []byte(key),
	})
	require.NoError(t, err)
	require.EqualValues(t, respDelete.Deleted, 0)
}

func TestRangeOptions(t *testing.T) {
	appV3be, _ := defaultApplierV3Backend(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	putKeys(ctx, t, fourKeys, appV3be) // revs 2, 3, 4 and 5
	putKeys(ctx, t, []pb.PutRequest{
		{Key: []byte(key), Value: updatedValue},  // rev 6
		{Key: []byte(key), Value: updatedValue2}, // rev 7
	}, appV3be)

	tcs := []struct {
		name       string
		request    *pb.RangeRequest
		expectedV  [][]byte
		expectedC  int64
		customTest func(t *testing.T, r *pb.RangeResponse)
	}{
		{
			name:      "Selecting single key revision",
			request:   &pb.RangeRequest{Key: []byte(key), Revision: 6},
			expectedV: [][]byte{updatedValue},
			expectedC: 1,
		},
		{
			// Should yield 3 KVs and count:4, since `key` has a revision6 value, but it's current ModRevision is 7
			name:      "All keys in ModRevision range",
			request:   &pb.RangeRequest{Key: zero, RangeEnd: zero, MinModRevision: 3, MaxModRevision: 6},
			expectedV: [][]byte{fourKeys[1].Value, fourKeys[2].Value, fourKeys[3].Value},
			expectedC: 4,
		},
		{
			name:      "All keys in CreateRevision range",
			request:   &pb.RangeRequest{Key: zero, RangeEnd: zero, MinCreateRevision: 2, MaxCreateRevision: 5},
			expectedV: [][]byte{updatedValue2, fourKeys[1].Value, fourKeys[2].Value, fourKeys[3].Value},
			expectedC: 4,
		},
		{
			name:       "Limited subset of all keys sorted desc by value",
			request:    &pb.RangeRequest{Key: zero, RangeEnd: zero, Limit: 3, SortOrder: pb.RangeRequest_DESCEND, SortTarget: pb.RangeRequest_VALUE},
			expectedV:  [][]byte{updatedValue2, fourKeys[3].Value, fourKeys[2].Value},
			expectedC:  4,
			customTest: func(t *testing.T, r *pb.RangeResponse) { require.True(t, r.More) },
		},
		{
			name:      "Limited subset of keys sorted asc by version",
			request:   &pb.RangeRequest{Key: zero, RangeEnd: fourKeys[2].Key, SortOrder: pb.RangeRequest_ASCEND, SortTarget: pb.RangeRequest_VERSION},
			expectedV: [][]byte{fourKeys[1].Value, updatedValue2},
			expectedC: 2,
		},
		{
			name:       "All keys only",
			request:    &pb.RangeRequest{Key: zero, RangeEnd: zero, KeysOnly: true},
			expectedV:  [][]byte{nil, nil, nil, nil},
			expectedC:  4,
			customTest: func(t *testing.T, r *pb.RangeResponse) { require.Len(t, r.Kvs, 4) },
		},
		{
			name:      "Count keys greater then or equal to key2",
			request:   &pb.RangeRequest{Key: []byte(key2), RangeEnd: zero, CountOnly: true},
			expectedV: [][]byte{},
			expectedC: 3,
		},
		// TODO: test Serializable
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := appV3be.Range(ctx, tc.request)
			require.NoError(t, err)
			require.EqualValues(t, tc.expectedC, resp.Count)
			for i, v := range tc.expectedV {
				require.EqualValues(t, v, resp.Kvs[i].Value)
			}
			if tc.customTest != nil {
				tc.customTest(t, resp)
			}
		})
	}
}

func TestBackendTxn(t *testing.T) {
	appV3be, _ := defaultApplierV3Backend(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	putKeys(ctx, t, fourKeys, appV3be)
	txnKey1 := &pb.PutRequest{Key: []byte("txnKey1"), Value: []byte("good")}
	range24 := &pb.RangeRequest{Key: fourKeys[1].Key, RangeEnd: fourKeys[3].Key}
	txnKey2 := &pb.PutRequest{Key: []byte("txnKey2"), Value: []byte("flop")}

	// single key (value equal) comparison (success), then put txnKey1
	response, _, err := appV3be.Txn(ctx, &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Result:      pb.Compare_EQUAL,
				Target:      pb.Compare_VALUE,
				Key:         fourKeys[0].Key,
				TargetUnion: &pb.Compare_Value{Value: fourKeys[0].Value},
			},
		},
		Success: []*pb.RequestOp{
			{Request: &pb.RequestOp_RequestPut{RequestPut: txnKey1}},
		},
	})
	require.NoError(t, err)
	require.True(t, response.Succeeded)
	require.Len(t, response.Responses, 1)

	// single key (mod greater) comparison (fail) then range[key2, key4)/put(txnKey2)
	response, _, err = appV3be.Txn(ctx, &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Result:      pb.Compare_GREATER,
				Target:      pb.Compare_MOD,
				Key:         fourKeys[0].Key,
				TargetUnion: &pb.Compare_ModRevision{ModRevision: 2},
			},
		},
		Failure: []*pb.RequestOp{
			{Request: &pb.RequestOp_RequestRange{RequestRange: range24}},
			{Request: &pb.RequestOp_RequestPut{RequestPut: txnKey2}},
		},
	})
	require.NoError(t, err)
	require.False(t, response.Succeeded)
	require.Len(t, response.Responses, 2)
	responseRange, ok := response.Responses[0].Response.(*pb.ResponseOp_ResponseRange)
	require.True(t, ok)
	require.EqualValues(t, 2, responseRange.ResponseRange.Count)
	responsePut, ok := response.Responses[1].Response.(*pb.ResponseOp_ResponsePut)
	require.True(t, ok)
	require.Nil(t, responsePut.ResponsePut.PrevKv)

	// multiple key (create less/not equal) comparison (success), then delete(key)
	response, _, err = appV3be.Txn(ctx, &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Result:      pb.Compare_LESS,
				Target:      pb.Compare_CREATE,
				Key:         fourKeys[0].Key,
				TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 5},
			},
			{
				Result:      pb.Compare_NOT_EQUAL,
				Target:      pb.Compare_VERSION,
				Key:         fourKeys[1].Key,
				TargetUnion: &pb.Compare_Version{Version: 3},
			},
		},
		Success: []*pb.RequestOp{
			{Request: &pb.RequestOp_RequestDeleteRange{
				RequestDeleteRange: &pb.DeleteRangeRequest{
					Key: fourKeys[0].Key,
				},
			}},
		},
	})
	require.NoError(t, err)
	require.True(t, response.Succeeded)
	deleteResponse, ok := response.Responses[0].Response.(*pb.ResponseOp_ResponseDeleteRange)
	require.True(t, ok)
	require.EqualValues(t, 1, deleteResponse.ResponseDeleteRange.Deleted)
}

func TestBackendCompaction(t *testing.T) {
	appV3be, _ := defaultApplierV3Backend(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// key(rev2) key2(rev3) key3(rev4,updated 6) key4(rev5, updated 7)
	putKeys(ctx, t, fourKeys, appV3be)
	putKeys(ctx, t, []pb.PutRequest{
		{Key: []byte(key3), Value: []byte("temp3 value")},
		{Key: []byte(key4), Value: []byte("temp4 value")},
	}, appV3be)

	_, _, _, err := appV3be.Compaction(&pb.CompactionRequest{Revision: 5})
	require.NoError(t, err)

	_, err = appV3be.Range(ctx, &pb.RangeRequest{
		Key:      []byte(key),
		RangeEnd: zero,
		Revision: 3,
	})
	require.EqualValues(t, mvcc.ErrCompacted, err)
}

func defaultQuotaApplierV3(t *testing.T, remainingSpace int) *quotaApplierV3 {
	appV3be, be := defaultApplierV3Backend(t)

	return &quotaApplierV3{
		appV3be,
		serverstorage.NewBackendQuota(appV3be.lg, be.Size()+int64(remainingSpace), be, "v3-applier"),
	}
}

func TestQuotaPut(t *testing.T) {
	qApp := defaultQuotaApplierV3(t, putInherentCost+128)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// `Put` beyond quota should yield "no space" error
	_, _, err := qApp.Put(ctx, &pb.PutRequest{
		Key:   []byte(key),
		Value: []byte(value128),
	})
	require.EqualValues(t, errors.ErrNoSpace, err)

	// Putting 64B should go OK: 256 + 64 < 256 + 128
	result, _, err := qApp.Put(ctx, &pb.PutRequest{
		Key:   []byte(key),
		Value: []byte(value64),
	})
	require.NoError(t, err)
	require.Greater(t, result.Header.Revision, int64(0))
}
