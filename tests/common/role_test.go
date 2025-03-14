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

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestRoleAdd_Simple(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				_, err := cc.RoleAdd(ctx, "root")
				require.NoError(t, err)
			})
		})
	}
}

func TestRoleAdd_Error(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterSize(1))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		_, err := cc.RoleAdd(ctx, "test-role")
		require.NoError(t, err)
		_, err = cc.RoleAdd(ctx, "test-role")
		require.ErrorContainsf(t, err, rpctypes.ErrRoleAlreadyExist.Error(), "want (%v) error, but got (%v)", rpctypes.ErrRoleAlreadyExist, err)
		_, err = cc.RoleAdd(ctx, "")
		require.ErrorContainsf(t, err, rpctypes.ErrRoleEmpty.Error(), "want (%v) error, but got (%v)", rpctypes.ErrRoleEmpty, err)
	})
}

func TestRootRole(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterSize(1))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		_, err := cc.RoleAdd(ctx, "root")
		require.NoError(t, err)
		resp, err := cc.RoleGet(ctx, "root")
		require.NoError(t, err)
		t.Logf("get role resp %+v", resp)
		// granting to root should be refused by server and a no-op
		_, err = cc.RoleGrantPermission(ctx, "root", "foo", "", clientv3.PermissionType(clientv3.PermReadWrite))
		require.NoError(t, err)
		resp2, err := cc.RoleGet(ctx, "root")
		require.NoError(t, err)
		t.Logf("get role resp %+v", resp2)
	})
}

func TestRoleGrantRevokePermission(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterSize(1))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		_, err := cc.RoleAdd(ctx, "role1")
		require.NoError(t, err)
		_, err = cc.RoleGrantPermission(ctx, "role1", "bar", "", clientv3.PermissionType(clientv3.PermRead))
		require.NoError(t, err)
		_, err = cc.RoleGrantPermission(ctx, "role1", "bar", "", clientv3.PermissionType(clientv3.PermWrite))
		require.NoError(t, err)
		_, err = cc.RoleGrantPermission(ctx, "role1", "bar", "foo", clientv3.PermissionType(clientv3.PermReadWrite))
		require.NoError(t, err)
		_, err = cc.RoleRevokePermission(ctx, "role1", "foo", "")
		require.ErrorContainsf(t, err, rpctypes.ErrPermissionNotGranted.Error(), "want error (%v), but got (%v)", rpctypes.ErrPermissionNotGranted, err)
		_, err = cc.RoleRevokePermission(ctx, "role1", "bar", "foo")
		require.NoError(t, err)
	})
}

func TestRoleDelete(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterSize(1))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		_, err := cc.RoleAdd(ctx, "role1")
		require.NoError(t, err)
		_, err = cc.RoleDelete(ctx, "role1")
		require.NoError(t, err)
	})
}
