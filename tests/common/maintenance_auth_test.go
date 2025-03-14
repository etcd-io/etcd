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

package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

/*
Test Defragment
*/
func TestDefragmentWithNoAuth(t *testing.T) {
	testDefragmentWithAuth(t, false, true)
}

func TestDefragmentWithInvalidAuth(t *testing.T) {
	testDefragmentWithAuth(t, true, true, WithAuth("invalid", "invalid"))
}

func TestDefragmentWithRootAuth(t *testing.T) {
	testDefragmentWithAuth(t, false, false, WithAuth("root", "rootPass"))
}

func TestDefragmentWithUserAuth(t *testing.T) {
	testDefragmentWithAuth(t, false, true, WithAuth("user0", "user0Pass"))
}

func testDefragmentWithAuth(t *testing.T, expectConnectionError, expectOperationError bool, opts ...config.ClientOption) {
	testMaintenanceOperationWithAuth(t, expectConnectionError, expectOperationError, func(ctx context.Context, cc intf.Client) error {
		return cc.Defragment(ctx, config.DefragOption{Timeout: 10 * time.Second})
	}, opts...)
}

/*
Test Downgrade
*/
func TestDowngradeWithNoAuth(t *testing.T) {
	testDowngradeWithAuth(t, false, true)
}

func TestDowngradeWithInvalidAuth(t *testing.T) {
	testDowngradeWithAuth(t, true, true, WithAuth("invalid", "invalid"))
}

func TestDowngradeWithRootAuth(t *testing.T) {
	testDowngradeWithAuth(t, false, false, WithAuth("root", "rootPass"))
}

func TestDowngradeWithUserAuth(t *testing.T) {
	testDowngradeWithAuth(t, false, true, WithAuth("user0", "user0Pass"))
}

func testDowngradeWithAuth(t *testing.T, _expectConnectionError, _expectOperationError bool, _opts ...config.ClientOption) {
	// TODO(ahrtr): finish this after we added interface methods `Downgrade` into `Client`
	t.Skip()
}

/*
Test HashKV
*/
func TestHashKVWithNoAuth(t *testing.T) {
	testHashKVWithAuth(t, false, true)
}

func TestHashKVWithInvalidAuth(t *testing.T) {
	testHashKVWithAuth(t, true, true, WithAuth("invalid", "invalid"))
}

func TestHashKVWithRootAuth(t *testing.T) {
	testHashKVWithAuth(t, false, false, WithAuth("root", "rootPass"))
}

func TestHashKVWithUserAuth(t *testing.T) {
	testHashKVWithAuth(t, false, true, WithAuth("user0", "user0Pass"))
}

func testHashKVWithAuth(t *testing.T, expectConnectionError, expectOperationError bool, opts ...config.ClientOption) {
	testMaintenanceOperationWithAuth(t, expectConnectionError, expectOperationError, func(ctx context.Context, cc intf.Client) error {
		_, err := cc.HashKV(ctx, 0)
		return err
	}, opts...)
}

/*
Test MoveLeader
*/
func TestMoveLeaderWithNoAuth(t *testing.T) {
	testMoveLeaderWithAuth(t, false, true)
}

func TestMoveLeaderWithInvalidAuth(t *testing.T) {
	testMoveLeaderWithAuth(t, true, true, WithAuth("invalid", "invalid"))
}

func TestMoveLeaderWithRootAuth(t *testing.T) {
	testMoveLeaderWithAuth(t, false, false, WithAuth("root", "rootPass"))
}

func TestMoveLeaderWithUserAuth(t *testing.T) {
	testMoveLeaderWithAuth(t, false, true, WithAuth("user0", "user0Pass"))
}

func testMoveLeaderWithAuth(t *testing.T, _expectConnectionError, _expectOperationError bool, _opts ...config.ClientOption) {
	// TODO(ahrtr): finish this after we added interface methods `MoveLeader` into `Client`
	t.Skip()
}

/*
Test Snapshot
*/
func TestSnapshotWithNoAuth(t *testing.T) {
	testSnapshotWithAuth(t, false, true)
}

func TestSnapshotWithInvalidAuth(t *testing.T) {
	testSnapshotWithAuth(t, true, true, WithAuth("invalid", "invalid"))
}

func TestSnapshotWithRootAuth(t *testing.T) {
	testSnapshotWithAuth(t, false, false, WithAuth("root", "rootPass"))
}

func TestSnapshotWithUserAuth(t *testing.T) {
	testSnapshotWithAuth(t, false, true, WithAuth("user0", "user0Pass"))
}

func testSnapshotWithAuth(t *testing.T, _expectConnectionError, _expectOperationError bool, _opts ...config.ClientOption) {
	// TODO(ahrtr): finish this after we added interface methods `Snapshot` into `Client`
	t.Skip()
}

/*
Test Status
*/
func TestStatusWithNoAuth(t *testing.T) {
	testStatusWithAuth(t, false, true)
}

func TestStatusWithInvalidAuth(t *testing.T) {
	testStatusWithAuth(t, true, true, WithAuth("invalid", "invalid"))
}

func TestStatusWithRootAuth(t *testing.T) {
	testStatusWithAuth(t, false, false, WithAuth("root", "rootPass"))
}

func TestStatusWithUserAuth(t *testing.T) {
	testStatusWithAuth(t, false, true, WithAuth("user0", "user0Pass"))
}

func testStatusWithAuth(t *testing.T, expectConnectionError, expectOperationError bool, opts ...config.ClientOption) {
	testMaintenanceOperationWithAuth(t, expectConnectionError, expectOperationError, func(ctx context.Context, cc intf.Client) error {
		_, err := cc.Status(ctx)
		return err
	}, opts...)
}

func setupAuthForMaintenanceTest(c intf.Client) error {
	roles := []authRole{
		{
			role:       "role0",
			permission: clientv3.PermissionType(clientv3.PermReadWrite),
			key:        "foo",
		},
	}

	users := []authUser{
		{
			user: "root",
			pass: "rootPass",
			role: "root",
		},
		{
			user: "user0",
			pass: "user0Pass",
			role: "role0",
		},
	}

	return setupAuth(c, roles, users)
}

func testMaintenanceOperationWithAuth(t *testing.T, expectConnectError, expectOperationError bool, f func(context.Context, intf.Client) error, opts ...config.ClientOption) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	clus := testRunner.NewCluster(ctx, t)
	defer clus.Close()

	cc := testutils.MustClient(clus.Client())
	err := setupAuthForMaintenanceTest(cc)
	require.NoError(t, err)

	ccWithAuth, err := clus.Client(opts...)
	if expectConnectError {
		require.Errorf(t, err, "%s: expected connection error, but got successful response", t.Name())
		t.Logf("%s: connection error: %v", t.Name(), err)
		return
	}
	if err != nil {
		require.NoErrorf(t, err, "%s: unexpected connection error", t.Name())
		return
	}

	// sleep 1 second to wait for etcd cluster to finish the authentication process.
	// TODO(ahrtr): find a better way to do it.
	time.Sleep(1 * time.Second)
	testutils.ExecuteUntil(ctx, t, func() {
		err := f(ctx, ccWithAuth)

		if expectOperationError {
			require.Errorf(t, err, "%s: expected error, but got successful response", t.Name())
			t.Logf("%s: operation error: %v", t.Name(), err)
			return
		}

		require.NoErrorf(t, err, "%s: unexpected operation error", t.Name())
	})
}
