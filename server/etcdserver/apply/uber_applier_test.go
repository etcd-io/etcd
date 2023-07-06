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
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

const memberId = 111195

func defaultUberApplier(t *testing.T) UberApplier {
	lg := zaptest.NewLogger(t)
	be, _ := betesting.NewDefaultTmpBackend(t)
	t.Cleanup(func() {
		betesting.Close(t, be)
	})

	cluster := membership.NewCluster(lg)
	cluster.AddMember(&membership.Member{ID: memberId}, true)
	lessor := lease.NewLessor(lg, be, cluster, lease.LessorConfig{})
	kv := mvcc.NewStore(lg, be, lessor, mvcc.StoreConfig{})
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
	return NewUberApplier(
		lg,
		be,
		kv,
		alarmStore,
		authStore,
		lessor,
		cluster,
		&fakeRaftStatusGetter{},
		&fakeSnapshotServer{},
		consistentIndex,
		1*time.Hour,
		false,
		16*1024*1024, //16MB
	)
}

// TestUberApplier_Alarm_Corrupt tests the applier returns ErrCorrupt after alarm CORRUPT is activated
func TestUberApplier_Alarm_Corrupt(t *testing.T) {
	tcs := []struct {
		name        string
		request     *pb.InternalRaftRequest
		expectError error
	}{
		{
			name:        "Put request returns ErrCorrupt after alarm CORRUPT is activated",
			request:     &pb.InternalRaftRequest{Put: &pb.PutRequest{}},
			expectError: errors.ErrCorrupt,
		},
		{
			name:        "Range request returns ErrCorrupt after alarm CORRUPT is activated",
			request:     &pb.InternalRaftRequest{Range: &pb.RangeRequest{}},
			expectError: errors.ErrCorrupt,
		},
		{
			name:        "DeleteRange request returns ErrCorrupt after alarm CORRUPT is activated",
			request:     &pb.InternalRaftRequest{DeleteRange: &pb.DeleteRangeRequest{}},
			expectError: errors.ErrCorrupt,
		},
		{
			name:        "Txn request returns ErrCorrupt after alarm CORRUPT is activated",
			request:     &pb.InternalRaftRequest{Txn: &pb.TxnRequest{}},
			expectError: errors.ErrCorrupt,
		},
		{
			name:        "Compaction request returns ErrCorrupt after alarm CORRUPT is activated",
			request:     &pb.InternalRaftRequest{Compaction: &pb.CompactionRequest{}},
			expectError: errors.ErrCorrupt,
		},
		{
			name:        "LeaseGrant request returns ErrCorrupt after alarm CORRUPT is activated",
			request:     &pb.InternalRaftRequest{LeaseGrant: &pb.LeaseGrantRequest{}},
			expectError: errors.ErrCorrupt,
		},
		{
			name:        "LeaseRevoke request returns ErrCorrupt after alarm CORRUPT is activated",
			request:     &pb.InternalRaftRequest{LeaseRevoke: &pb.LeaseRevokeRequest{}},
			expectError: errors.ErrCorrupt,
		},
	}

	ua := defaultUberApplier(t)
	result := ua.Apply(&pb.InternalRaftRequest{
		Header: &pb.RequestHeader{},
		Alarm: &pb.AlarmRequest{
			Action:   pb.AlarmRequest_ACTIVATE,
			MemberID: memberId,
			Alarm:    pb.AlarmType_CORRUPT,
		},
	}, true)
	require.NotNil(t, result)
	require.Nil(t, result.Err)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result = ua.Apply(tc.request, true)
			require.NotNil(t, result)
			require.Equalf(t, tc.expectError, result.Err, "Apply: got %v, expect: %v", result.Err, tc.expectError)
		})
	}
}

// TestUberApplier_Alarm_Quota tests the applier returns ErrNoSpace after alarm NOSPACE is activated
func TestUberApplier_Alarm_Quota(t *testing.T) {
	tcs := []struct {
		name        string
		request     *pb.InternalRaftRequest
		expectError error
	}{
		{
			name:        "Put request returns ErrCorrupt after alarm NOSPACE is activated",
			request:     &pb.InternalRaftRequest{Put: &pb.PutRequest{Key: []byte(key)}},
			expectError: errors.ErrNoSpace,
		},
		{
			name: "Txn request cost > 0 returns ErrCorrupt after alarm NOSPACE is activated",
			request: &pb.InternalRaftRequest{Txn: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte(key),
							},
						},
					},
				}}},
			expectError: errors.ErrNoSpace,
		},
		{
			name: "Txn request cost = 0 is still allowed after alarm NOSPACE is activated",
			request: &pb.InternalRaftRequest{Txn: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key: []byte(key),
							},
						},
					},
				}}},
			expectError: nil,
		},
		{
			name: "Txn request cost = 0 in both branches is still allowed after alarm NOSPACE is activated",
			request: &pb.InternalRaftRequest{Txn: &pb.TxnRequest{
				Compare: []*pb.Compare{
					{
						Key:         []byte(key),
						Result:      pb.Compare_EQUAL,
						Target:      pb.Compare_CREATE,
						TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 0},
					},
				},
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestRange{
							RequestRange: &pb.RangeRequest{
								Key: []byte(key),
							},
						},
					},
				},
				Failure: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestDeleteRange{
							RequestDeleteRange: &pb.DeleteRangeRequest{
								Key: []byte(key),
							},
						},
					},
				}}},
			expectError: nil,
		},
		{
			name:        "LeaseGrant request returns ErrCorrupt after alarm NOSPACE is activated",
			request:     &pb.InternalRaftRequest{LeaseGrant: &pb.LeaseGrantRequest{}},
			expectError: errors.ErrNoSpace,
		},
	}

	ua := defaultUberApplier(t)
	result := ua.Apply(&pb.InternalRaftRequest{
		Header: &pb.RequestHeader{},
		Alarm: &pb.AlarmRequest{
			Action:   pb.AlarmRequest_ACTIVATE,
			MemberID: memberId,
			Alarm:    pb.AlarmType_NOSPACE,
		},
	}, true)
	require.NotNil(t, result)
	require.Nil(t, result.Err)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result = ua.Apply(tc.request, true)
			require.NotNil(t, result)
			require.Equalf(t, tc.expectError, result.Err, "Apply: got %v, expect: %v", result.Err, tc.expectError)
		})
	}
}

// TestUberApplier_Alarm_Deactivate tests the applier should be able to apply after alarm is deactivated
func TestUberApplier_Alarm_Deactivate(t *testing.T) {
	ua := defaultUberApplier(t)
	result := ua.Apply(&pb.InternalRaftRequest{
		Header: &pb.RequestHeader{},
		Alarm: &pb.AlarmRequest{
			Action:   pb.AlarmRequest_ACTIVATE,
			MemberID: memberId,
			Alarm:    pb.AlarmType_NOSPACE,
		},
	}, true)
	require.NotNil(t, result)
	require.Nil(t, result.Err)

	result = ua.Apply(&pb.InternalRaftRequest{Put: &pb.PutRequest{Key: []byte(key)}}, true)
	require.NotNil(t, result)
	require.Equalf(t, errors.ErrNoSpace, result.Err, "Apply: got %v, expect: %v", result.Err, errors.ErrNoSpace)

	result = ua.Apply(&pb.InternalRaftRequest{
		Header: &pb.RequestHeader{},
		Alarm: &pb.AlarmRequest{
			Action:   pb.AlarmRequest_DEACTIVATE,
			MemberID: memberId,
			Alarm:    pb.AlarmType_NOSPACE,
		},
	}, true)
	require.NotNil(t, result)
	require.Nil(t, result.Err)

	result = ua.Apply(&pb.InternalRaftRequest{Put: &pb.PutRequest{Key: []byte(key)}}, true)
	require.NotNil(t, result)
	require.Nil(t, result.Err)
}
