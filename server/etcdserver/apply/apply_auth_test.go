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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/crypto/bcrypt"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3alarm"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

func dummyIndexWaiter(_ uint64) <-chan struct{} {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return ch
}

func dummyApplyFunc(_ context.Context, _ *pb.InternalRaftRequest, _ membership.ShouldApplyV3) *Result {
	return &Result{}
}

type fakeRaftStatusGetter struct{}

func (*fakeRaftStatusGetter) MemberId() types.ID {
	return 0
}
func (*fakeRaftStatusGetter) Leader() types.ID {
	return 0
}
func (*fakeRaftStatusGetter) CommittedIndex() uint64 {
	return 0
}
func (*fakeRaftStatusGetter) AppliedIndex() uint64 {
	return 0
}
func (*fakeRaftStatusGetter) Term() uint64 {
	return 0
}

type fakeSnapshotServer struct{}

func (*fakeSnapshotServer) ForceSnapshot() {}

func defaultAuthApplierV3(t *testing.T) *authApplierV3 {
	lg := zaptest.NewLogger(t)
	be, _ := betesting.NewDefaultTmpBackend(t)
	t.Cleanup(func() {
		betesting.Close(t, be)
	})

	cluster := membership.NewCluster(lg)
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
	return newAuthApplierV3(
		authStore,
		newApplierV3Backend(
			lg,
			kv,
			alarmStore,
			authStore,
			lessor,
			cluster,
			&fakeRaftStatusGetter{},
			&fakeSnapshotServer{},
			consistentIndex,
			false,
		),
		lessor)
}

const (
	userRoot      = "root"
	roleRoot      = "root"
	userReadOnly  = "user_read_only"
	roleReadOnly  = "role_read_only"
	userWriteOnly = "user_write_only"
	roleWriteOnly = "role_write_only"

	key             = "key"
	rangeEnd        = "rangeEnd"
	keyOutsideRange = "rangeEnd_outside"

	LeaseId = 1
)

func mustCreateRolesAndEnableAuth(t *testing.T, authApplier *authApplierV3) {
	_, err := authApplier.UserAdd(&pb.AuthUserAddRequest{Name: userRoot, Options: &authpb.UserAddOptions{NoPassword: true}})
	require.NoError(t, err)
	_, err = authApplier.RoleAdd(&pb.AuthRoleAddRequest{Name: roleRoot})
	require.NoError(t, err)
	_, err = authApplier.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: userRoot, Role: roleRoot})
	require.NoError(t, err)

	_, err = authApplier.UserAdd(&pb.AuthUserAddRequest{Name: userReadOnly, Options: &authpb.UserAddOptions{NoPassword: true}})
	require.NoError(t, err)
	_, err = authApplier.RoleAdd(&pb.AuthRoleAddRequest{Name: roleReadOnly})
	require.NoError(t, err)
	_, err = authApplier.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: userReadOnly, Role: roleReadOnly})
	require.NoError(t, err)
	_, err = authApplier.RoleGrantPermission(&pb.AuthRoleGrantPermissionRequest{Name: roleReadOnly, Perm: &authpb.Permission{
		PermType: authpb.READ,
		Key:      []byte(key),
		RangeEnd: []byte(rangeEnd),
	}})
	require.NoError(t, err)

	_, err = authApplier.UserAdd(&pb.AuthUserAddRequest{Name: userWriteOnly, Options: &authpb.UserAddOptions{NoPassword: true}})
	require.NoError(t, err)
	_, err = authApplier.RoleAdd(&pb.AuthRoleAddRequest{Name: roleWriteOnly})
	require.NoError(t, err)
	_, err = authApplier.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: userWriteOnly, Role: roleWriteOnly})
	require.NoError(t, err)
	_, err = authApplier.RoleGrantPermission(&pb.AuthRoleGrantPermissionRequest{Name: roleWriteOnly, Perm: &authpb.Permission{
		PermType: authpb.WRITE,
		Key:      []byte(key),
		RangeEnd: []byte(rangeEnd),
	}})
	require.NoError(t, err)

	_, err = authApplier.AuthEnable()
	require.NoError(t, err)
}

// setAuthInfo manually sets the authInfo of the applier. In reality, authInfo is filled before Apply()
func setAuthInfo(authApplier *authApplierV3, userName string) {
	authApplier.authInfo = auth.AuthInfo{
		Username: userName,
		Revision: authApplier.as.Revision(),
	}
}

// TestAuthApplierV3_Apply ensures Apply() calls applyFunc() when permission is granted
// and returns an error when permission is denied
func TestAuthApplierV3_Apply(t *testing.T) {
	tcs := []struct {
		name         string
		request      *pb.InternalRaftRequest
		expectResult *Result
	}{
		{
			name: "request does not need admin permission",
			request: &pb.InternalRaftRequest{
				Header: &pb.RequestHeader{},
			},
			expectResult: &Result{},
		},
		{
			name: "request needs admin permission but permission denied",
			request: &pb.InternalRaftRequest{
				Header: &pb.RequestHeader{
					Username: userReadOnly,
				},
				AuthEnable: &pb.AuthEnableRequest{},
			},
			expectResult: &Result{
				Err: auth.ErrPermissionDenied,
			},
		},
		{
			name: "request needs admin permission and permitted",
			request: &pb.InternalRaftRequest{
				Header: &pb.RequestHeader{
					Username: userRoot,
				},
				AuthEnable: &pb.AuthEnableRequest{},
			},
			expectResult: &Result{},
		},
	}

	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := authApplier.Apply(ctx, tc.request, false, dummyApplyFunc)
			require.Equalf(t, result, tc.expectResult, "Apply: got %v, expect: %v", result, tc.expectResult)
		})
	}

}

// TestAuthApplierV3_AdminPermission ensures the admin permission is checked for certain
// operations
func TestAuthApplierV3_AdminPermission(t *testing.T) {
	tcs :=
		[]struct {
			name                  string
			request               *pb.InternalRaftRequest
			adminPermissionNeeded bool
		}{
			{
				name:                  "Range does not need admin permission",
				request:               &pb.InternalRaftRequest{Range: &pb.RangeRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "Put does not need admin permission",
				request:               &pb.InternalRaftRequest{Put: &pb.PutRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "DeleteRange does not need admin permission",
				request:               &pb.InternalRaftRequest{DeleteRange: &pb.DeleteRangeRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "Txn does not need admin permission",
				request:               &pb.InternalRaftRequest{Txn: &pb.TxnRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "Compaction does not need admin permission",
				request:               &pb.InternalRaftRequest{Compaction: &pb.CompactionRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "LeaseGrant does not need admin permission",
				request:               &pb.InternalRaftRequest{LeaseGrant: &pb.LeaseGrantRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "LeaseRevoke does not need admin permission",
				request:               &pb.InternalRaftRequest{LeaseRevoke: &pb.LeaseRevokeRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "Alarm does not need admin permission",
				request:               &pb.InternalRaftRequest{Alarm: &pb.AlarmRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "LeaseCheckpoint does not need admin permission",
				request:               &pb.InternalRaftRequest{LeaseCheckpoint: &pb.LeaseCheckpointRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "Authenticate does not need admin permission",
				request:               &pb.InternalRaftRequest{Authenticate: &pb.InternalAuthenticateRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "ClusterVersionSet does not need admin permission",
				request:               &pb.InternalRaftRequest{ClusterVersionSet: &membershippb.ClusterVersionSetRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "ClusterMemberAttrSet does not need admin permission",
				request:               &pb.InternalRaftRequest{ClusterMemberAttrSet: &membershippb.ClusterMemberAttrSetRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "DowngradeInfoSet does not need admin permission",
				request:               &pb.InternalRaftRequest{DowngradeInfoSet: &membershippb.DowngradeInfoSetRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "AuthUserGet does not need admin permission",
				request:               &pb.InternalRaftRequest{AuthUserGet: &pb.AuthUserGetRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "AuthRoleGet does not need admin permission",
				request:               &pb.InternalRaftRequest{AuthRoleGet: &pb.AuthRoleGetRequest{}},
				adminPermissionNeeded: false,
			},
			{
				name:                  "AuthEnable needs admin permission",
				request:               &pb.InternalRaftRequest{AuthEnable: &pb.AuthEnableRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthDisable needs admin permission",
				request:               &pb.InternalRaftRequest{AuthDisable: &pb.AuthDisableRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthStatus needs admin permission",
				request:               &pb.InternalRaftRequest{AuthStatus: &pb.AuthStatusRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthUserAdd needs admin permission",
				request:               &pb.InternalRaftRequest{AuthUserAdd: &pb.AuthUserAddRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthUserDelete needs admin permission",
				request:               &pb.InternalRaftRequest{AuthUserDelete: &pb.AuthUserDeleteRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthUserChangePassword needs admin permission",
				request:               &pb.InternalRaftRequest{AuthUserChangePassword: &pb.AuthUserChangePasswordRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthUserGrantRole needs admin permission",
				request:               &pb.InternalRaftRequest{AuthUserGrantRole: &pb.AuthUserGrantRoleRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthUserRevokeRole needs admin permission",
				request:               &pb.InternalRaftRequest{AuthUserRevokeRole: &pb.AuthUserRevokeRoleRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthUserList needs admin permission",
				request:               &pb.InternalRaftRequest{AuthUserList: &pb.AuthUserListRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthRoleList needs admin permission",
				request:               &pb.InternalRaftRequest{AuthRoleList: &pb.AuthRoleListRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthRoleAdd needs admin permission",
				request:               &pb.InternalRaftRequest{AuthRoleAdd: &pb.AuthRoleAddRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthRoleDelete needs admin permission",
				request:               &pb.InternalRaftRequest{AuthRoleDelete: &pb.AuthRoleDeleteRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthRoleGrantPermission needs admin permission",
				request:               &pb.InternalRaftRequest{AuthRoleGrantPermission: &pb.AuthRoleGrantPermissionRequest{}},
				adminPermissionNeeded: true,
			},
			{
				name:                  "AuthRoleRevokePermission needs admin permission",
				request:               &pb.InternalRaftRequest{AuthRoleRevokePermission: &pb.AuthRoleRevokePermissionRequest{}},
				adminPermissionNeeded: true,
			},
		}
	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.adminPermissionNeeded {
				tc.request.Header = &pb.RequestHeader{Username: userReadOnly}
			}
			result := authApplier.Apply(ctx, tc.request, false, dummyApplyFunc)
			require.Equal(t, result.Err == auth.ErrPermissionDenied, tc.adminPermissionNeeded,
				"Admin permission needed: got %v, expect: %v", result.Err == auth.ErrPermissionDenied, tc.adminPermissionNeeded)
		})
	}
}

// TestAuthApplierV3_Put verifies only users with write permissions in the key range can put
func TestAuthApplierV3_Put(t *testing.T) {
	tcs := []struct {
		name        string
		userName    string
		request     *pb.PutRequest
		expectError error
	}{
		{
			name:        "put permission denied",
			userName:    userReadOnly,
			request:     &pb.PutRequest{},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "prevKv is set, but user does not have read permission",
			userName: userWriteOnly,
			request: &pb.PutRequest{
				Key:    []byte(key),
				Value:  []byte("1"),
				PrevKv: true,
			},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "put success",
			userName: userWriteOnly,
			request: &pb.PutRequest{
				Key:   []byte(key),
				Value: []byte("1"),
			},
			expectError: nil,
		},
		{
			name:     "put success with PrevKv set",
			userName: userRoot,
			request: &pb.PutRequest{
				Key:    []byte(key),
				Value:  []byte("1"),
				PrevKv: true,
			},
			expectError: nil,
		},
	}

	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			setAuthInfo(authApplier, tc.userName)
			_, _, err := authApplier.Put(ctx, tc.request)
			require.Equalf(t, tc.expectError, err, "Put returned unexpected error (or lack thereof), expected: %v, got: %v", tc.expectError, err)
		})
	}
}

// TestAuthApplierV3_LeasePut verifies users cannot put with lease if the lease is attached with a key out of range
func TestAuthApplierV3_LeasePut(t *testing.T) {
	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := authApplier.LeaseGrant(&pb.LeaseGrantRequest{
		TTL: lease.MaxLeaseTTL,
		ID:  LeaseId,
	})
	require.NoError(t, err)

	// The user should be able to put the key
	setAuthInfo(authApplier, userWriteOnly)
	_, _, err = authApplier.Put(ctx, &pb.PutRequest{
		Key:   []byte(key),
		Value: []byte("1"),
		Lease: LeaseId,
	})
	require.NoError(t, err)

	// Put a key under the lease outside user's key range
	setAuthInfo(authApplier, userRoot)
	_, _, err = authApplier.Put(ctx, &pb.PutRequest{
		Key:   []byte(keyOutsideRange),
		Value: []byte("1"),
		Lease: LeaseId,
	})
	require.NoError(t, err)

	// The user should not be able to put the key anymore
	setAuthInfo(authApplier, userWriteOnly)
	_, _, err = authApplier.Put(ctx, &pb.PutRequest{
		Key:   []byte(key),
		Value: []byte("1"),
		Lease: LeaseId,
	})
	require.Equal(t, err, auth.ErrPermissionDenied)
}

// TestAuthApplierV3_Range verifies only users with read permissions can do range in the key range
func TestAuthApplierV3_Range(t *testing.T) {
	tcs := []struct {
		name        string
		userName    string
		request     *pb.RangeRequest
		expectError error
	}{
		{
			name:        "range permission denied",
			userName:    userWriteOnly,
			request:     &pb.RangeRequest{},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "range key out of range",
			userName: userReadOnly,
			request: &pb.RangeRequest{
				Key: []byte(keyOutsideRange),
			},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "range success",
			userName: userReadOnly,
			request: &pb.RangeRequest{
				Key:      []byte(key),
				RangeEnd: []byte(rangeEnd),
			},
			expectError: nil,
		},
	}

	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			setAuthInfo(authApplier, tc.userName)
			_, _, err := authApplier.Range(ctx, tc.request)
			require.Equalf(t, tc.expectError, err, "Range returned unexpected error (or lack thereof), expected: %v, got: %v", tc.expectError, err)
		})
	}
}

// TestAuthApplierV3_DeleteRange verifies only users with write permissions can do delete range in the key range
func TestAuthApplierV3_DeleteRange(t *testing.T) {
	tcs := []struct {
		name        string
		userName    string
		request     *pb.DeleteRangeRequest
		expectError error
	}{
		{
			name:        "delete range permission denied",
			userName:    userReadOnly,
			request:     &pb.DeleteRangeRequest{},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "delete range key out of range",
			userName: userWriteOnly,
			request: &pb.DeleteRangeRequest{
				Key: []byte(keyOutsideRange),
			},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "prevKv is set, but user does not have read permission",
			userName: userWriteOnly,
			request: &pb.DeleteRangeRequest{
				Key:      []byte(key),
				RangeEnd: []byte(rangeEnd),
				PrevKv:   true,
			},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "delete range success",
			userName: userWriteOnly,
			request: &pb.DeleteRangeRequest{
				Key:      []byte(key),
				RangeEnd: []byte(rangeEnd),
			},
			expectError: nil,
		},
		{
			name:     "delete range success with PrevKv",
			userName: userRoot,
			request: &pb.DeleteRangeRequest{
				Key:      []byte(key),
				RangeEnd: []byte(rangeEnd),
				PrevKv:   true,
			},
			expectError: nil,
		},
	}

	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			setAuthInfo(authApplier, tc.userName)
			_, _, err := authApplier.DeleteRange(context.Background(), tc.request)
			require.Equalf(t, tc.expectError, err, "Range returned unexpected error (or lack thereof), expected: %v, got: %v", tc.expectError, err)
		})
	}
}

// TestAuthApplierV3_Txn verifies txns can only be applied with proper permissions
func TestAuthApplierV3_Txn(t *testing.T) {
	tcs := []struct {
		name        string
		userName    string
		request     *pb.TxnRequest
		expectError error
	}{
		{
			name:     "txn range permission denied",
			userName: userWriteOnly,
			request: &pb.TxnRequest{
				Compare: []*pb.Compare{
					{
						Key: []byte(key),
					},
				},
			},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "txn put permission denied",
			userName: userReadOnly,
			request: &pb.TxnRequest{
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte(key),
							},
						},
					},
				},
			},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:     "txn success",
			userName: userRoot,
			request: &pb.TxnRequest{
				Compare: []*pb.Compare{
					{
						Key: []byte(key),
					},
				},
				Success: []*pb.RequestOp{
					{
						Request: &pb.RequestOp_RequestPut{
							RequestPut: &pb.PutRequest{
								Key: []byte(key),
							},
						},
					},
				},
			},
			expectError: nil,
		},
	}

	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			setAuthInfo(authApplier, tc.userName)
			_, _, err := authApplier.Txn(ctx, tc.request)
			require.Equalf(t, tc.expectError, err, "Range returned unexpected error (or lack thereof), expected: %v, got: %v", tc.expectError, err)
		})
	}
}

// TestAuthApplierV3_LeaseRevoke verifies user cannot revoke a lease if the lease is attached with
// a key out of range by someone else
func TestAuthApplierV3_LeaseRevoke(t *testing.T) {
	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := authApplier.LeaseGrant(&pb.LeaseGrantRequest{
		TTL: lease.MaxLeaseTTL,
		ID:  LeaseId,
	})
	require.NoError(t, err)

	// The user should be able to revoke the lease
	setAuthInfo(authApplier, userWriteOnly)
	_, err = authApplier.LeaseRevoke(&pb.LeaseRevokeRequest{
		ID: LeaseId,
	})
	require.NoError(t, err)

	_, err = authApplier.LeaseGrant(&pb.LeaseGrantRequest{
		TTL: lease.MaxLeaseTTL,
		ID:  LeaseId,
	})
	require.NoError(t, err)

	// Put a key under the lease outside user's key range
	setAuthInfo(authApplier, userRoot)
	_, _, err = authApplier.Put(ctx, &pb.PutRequest{
		Key:   []byte(keyOutsideRange),
		Value: []byte("1"),
		Lease: LeaseId,
	})
	require.NoError(t, err)

	// The user should not be able to revoke the lease anymore
	setAuthInfo(authApplier, userWriteOnly)
	_, err = authApplier.LeaseRevoke(&pb.LeaseRevokeRequest{
		ID: LeaseId,
	})
	require.Equal(t, err, auth.ErrPermissionDenied)
}

// TestAuthApplierV3_UserGet verifies UserGet can only be performed by the user itself or the root
func TestAuthApplierV3_UserGet(t *testing.T) {
	tcs := []struct {
		name        string
		userName    string
		request     *pb.AuthUserGetRequest
		expectError error
	}{
		{
			name:        "UserGet permission denied with non-root role and requests other user",
			userName:    userWriteOnly,
			request:     &pb.AuthUserGetRequest{Name: userReadOnly},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:        "UserGet success with non-root role but requests itself",
			userName:    userWriteOnly,
			request:     &pb.AuthUserGetRequest{Name: userWriteOnly},
			expectError: nil,
		},
		{
			name:        "UserGet success with root role",
			userName:    userRoot,
			request:     &pb.AuthUserGetRequest{Name: userWriteOnly},
			expectError: nil,
		},
	}

	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			setAuthInfo(authApplier, tc.userName)
			_, err := authApplier.UserGet(tc.request)
			require.Equalf(t, tc.expectError, err, "Range returned unexpected error (or lack thereof), expected: %v, got: %v", tc.expectError, err)
		})
	}
}

// TestAuthApplierV3_RoleGet verifies RoleGet can only be performed by the user in the role itself or the root
func TestAuthApplierV3_RoleGet(t *testing.T) {
	tcs := []struct {
		name        string
		userName    string
		request     *pb.AuthRoleGetRequest
		expectError error
	}{
		{
			name:        "RoleGet permission denied with non-root role and requests other role",
			userName:    userWriteOnly,
			request:     &pb.AuthRoleGetRequest{Role: roleReadOnly},
			expectError: auth.ErrPermissionDenied,
		},
		{
			name:        "RoleGet success with non-root role but requests itself",
			userName:    userWriteOnly,
			request:     &pb.AuthRoleGetRequest{Role: roleWriteOnly},
			expectError: nil,
		},
		{
			name:        "RoleGet success with root role",
			userName:    userRoot,
			request:     &pb.AuthRoleGetRequest{Role: roleWriteOnly},
			expectError: nil,
		},
	}

	authApplier := defaultAuthApplierV3(t)
	mustCreateRolesAndEnableAuth(t, authApplier)
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			setAuthInfo(authApplier, tc.userName)
			_, err := authApplier.RoleGet(tc.request)
			require.Equalf(t, tc.expectError, err, "Range returned unexpected error (or lack thereof), expected: %v, got: %v", tc.expectError, err)
		})
	}
}

func TestCheckLeasePutsKeys(t *testing.T) {
	aa := defaultAuthApplierV3(t)
	assert.NoError(t, aa.checkLeasePutsKeys(lease.NewLease(lease.LeaseID(1), 3600)), "auth is disabled, should allow puts")
	mustCreateRolesAndEnableAuth(t, aa)
	aa.authInfo = auth.AuthInfo{Username: "root"}
	assert.NoError(t, aa.checkLeasePutsKeys(lease.NewLease(lease.LeaseID(1), 3600)), "auth is enabled, should allow puts for root")

	l := lease.NewLease(lease.LeaseID(1), 3600)
	l.SetLeaseItem(lease.LeaseItem{Key: "a"})
	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: 0}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrUserEmpty, "auth is enabled, should not allow bob, non existing at rev 0")
	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: 1}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrAuthOldRevision, "auth is enabled, old revision")

	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: aa.as.Revision()}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrPermissionDenied, "auth is enabled, bob does not have permissions, bob does not exist")
	_, err := aa.as.UserAdd(&pb.AuthUserAddRequest{Name: "bob", Options: &authpb.UserAddOptions{NoPassword: true}})
	assert.NoError(t, err, "bob should be added without error")
	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: aa.as.Revision()}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrPermissionDenied, "auth is enabled, bob exists yet does not have permissions")

	// allow bob to access "a"
	_, err = aa.as.RoleAdd(&pb.AuthRoleAddRequest{Name: "bobsrole"})
	assert.NoError(t, err, "bobsrole should be added without error")
	_, err = aa.as.RoleGrantPermission(&pb.AuthRoleGrantPermissionRequest{
		Name: "bobsrole",
		Perm: &authpb.Permission{
			PermType: authpb.READWRITE,
			Key:      []byte("a"),
			RangeEnd: nil,
		},
	})
	assert.NoError(t, err, "bobsrole should be granted permissions without error")
	_, err = aa.as.UserGrantRole(&pb.AuthUserGrantRoleRequest{
		User: "bob",
		Role: "bobsrole",
	})
	assert.NoError(t, err, "bob should be granted bobsrole without error")

	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: aa.as.Revision()}
	assert.NoError(t, aa.checkLeasePutsKeys(l), "bob should be able to access key 'a'")

}
