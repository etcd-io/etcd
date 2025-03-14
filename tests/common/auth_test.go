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
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

var (
	tokenTTL         = time.Second * 3
	defaultAuthToken = fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=RS256,ttl=%s",
		mustAbsPath("../fixtures/server.crt"), mustAbsPath("../fixtures/server.key.insecure"), tokenTTL)
)

const (
	PermissionDenied      = "etcdserver: permission denied"
	AuthenticationFailed  = "etcdserver: authentication failed, invalid user ID or password"
	InvalidAuthManagement = "etcdserver: invalid auth management"

	testPeerURL = "http://localhost:20011"
)

func TestAuthEnable(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
	})
}

func TestAuthDisable(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoError(t, cc.Put(ctx, "hoo", "a", config.PutOptions{}))
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")

		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		// test-user doesn't have the permission, it must fail
		require.Error(t, testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}))
		require.NoErrorf(t, rootAuthClient.AuthDisable(ctx), "failed to disable auth")
		// now ErrAuthNotEnabled of Authenticate() is simply ignored
		require.NoError(t, testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}))
		// now the key can be accessed
		require.NoError(t, cc.Put(ctx, "hoo", "bar", config.PutOptions{}))
		// confirm put succeeded
		resp, err := cc.Get(ctx, "hoo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'hoo', 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "hoo", string(resp.Kvs[0].Key), "want key value pair 'hoo', 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "bar", string(resp.Kvs[0].Value), "want key value pair 'hoo', 'bar' but got %+v", resp.Kvs)
	})
}

func TestAuthGracefulDisable(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
		donec := make(chan struct{})
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

		startedC := make(chan struct{}, 1)
		go func() {
			defer close(donec)
			defer close(startedC)
			// sleep a bit to let the watcher connects while auth is still enabled
			time.Sleep(time.Second)
			// now disable auth...
			if err := rootAuthClient.AuthDisable(ctx); err != nil {
				t.Errorf("failed to auth disable %v", err)
				return
			}
			// ...and restart the node
			clus.Members()[0].Stop()
			if err := clus.Members()[0].Start(ctx); err != nil {
				t.Errorf("failed to restart member %v", err)
				return
			}
			startedC <- struct{}{}
			// the watcher should still work after reconnecting
			assert.NoErrorf(t, rootAuthClient.Put(ctx, "key", "value", config.PutOptions{}), "failed to put key value")
		}()

		<-startedC

		wCtx, wCancel := context.WithCancel(ctx)
		defer wCancel()

		watchCh := rootAuthClient.Watch(wCtx, "key", config.WatchOptions{Revision: 1})
		wantedLen := 1
		watchTimeout := 10 * time.Second
		wanted := []testutils.KV{{Key: "key", Val: "value"}}
		kvs, err := testutils.KeyValuesFromWatchChan(watchCh, wantedLen, watchTimeout)
		require.NoErrorf(t, err, "failed to get key-values from watch channel %s", err)
		require.Equal(t, wanted, kvs)
		<-donec
	})
}

func TestAuthStatus(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		resp, err := cc.AuthStatus(ctx)
		require.NoError(t, err)
		require.Falsef(t, resp.Enabled, "want auth not enabled but enabled")

		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		resp, err = rootAuthClient.AuthStatus(ctx)
		require.NoError(t, err)
		require.Truef(t, resp.Enabled, "want enabled but got not enabled")
	})
}

func TestAuthRoleUpdate(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoError(t, cc.Put(ctx, "foo", "bar", config.PutOptions{}))
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")

		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		require.ErrorContains(t, testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}), PermissionDenied)
		// grant a new key
		_, err := rootAuthClient.RoleGrantPermission(ctx, testRoleName, "hoo", "", clientv3.PermissionType(clientv3.PermReadWrite))
		require.NoError(t, err)
		// try a newly granted key
		require.NoError(t, testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}))
		// confirm put succeeded
		resp, err := testUserAuthClient.Get(ctx, "hoo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "hoo", string(resp.Kvs[0].Key), "want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "bar", string(resp.Kvs[0].Value), "want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		// revoke the newly granted key
		_, err = rootAuthClient.RoleRevokePermission(ctx, testRoleName, "hoo", "")
		require.NoError(t, err)
		// try put to the revoked key
		require.ErrorContains(t, testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}), PermissionDenied)
		// confirm a key still granted can be accessed
		resp, err = testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "foo", string(resp.Kvs[0].Key), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "bar", string(resp.Kvs[0].Value), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
	})
}

func TestAuthUserDeleteDuringOps(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoError(t, cc.Put(ctx, "foo", "bar", config.PutOptions{}))
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")

		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		// create a key
		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}))
		// confirm put succeeded
		resp, err := testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "foo", string(resp.Kvs[0].Key), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "bar", string(resp.Kvs[0].Value), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		// delete the user
		_, err = rootAuthClient.UserDelete(ctx, testUserName)
		require.NoError(t, err)
		// check the user is deleted
		err = testUserAuthClient.Put(ctx, "foo", "baz", config.PutOptions{})
		require.ErrorContains(t, err, AuthenticationFailed)
	})
}

func TestAuthRoleRevokeDuringOps(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoError(t, cc.Put(ctx, "foo", "bar", config.PutOptions{}))
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")

		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		// create a key
		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}))
		// confirm put succeeded
		resp, err := testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "foo", string(resp.Kvs[0].Key), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "bar", string(resp.Kvs[0].Value), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		// create a new role
		_, err = rootAuthClient.RoleAdd(ctx, "test-role2")
		require.NoError(t, err)
		// grant a new key to the new role
		_, err = rootAuthClient.RoleGrantPermission(ctx, "test-role2", "hoo", "", clientv3.PermissionType(clientv3.PermReadWrite))
		require.NoError(t, err)
		// grant the new role to the user
		_, err = rootAuthClient.UserGrantRole(ctx, testUserName, "test-role2")
		require.NoError(t, err)

		// try a newly granted key
		require.NoError(t, testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}))
		// confirm put succeeded
		resp, err = testUserAuthClient.Get(ctx, "hoo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "hoo", string(resp.Kvs[0].Key), "want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "bar", string(resp.Kvs[0].Value), "want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		// revoke a role from the user
		_, err = rootAuthClient.UserRevokeRole(ctx, testUserName, testRoleName)
		require.NoError(t, err)
		// check the role is revoked and permission is lost from the user
		require.ErrorContains(t, testUserAuthClient.Put(ctx, "foo", "baz", config.PutOptions{}), PermissionDenied)

		// try a key that can be accessed from the remaining role
		require.NoError(t, testUserAuthClient.Put(ctx, "hoo", "bar2", config.PutOptions{}))
		// confirm put succeeded
		resp, err = testUserAuthClient.Get(ctx, "hoo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'hoo' 'bar2' but got %+v", resp.Kvs)
		require.Equalf(t, "hoo", string(resp.Kvs[0].Key), "want key value pair 'hoo' 'bar2' but got %+v", resp.Kvs)
		require.Equalf(t, "bar2", string(resp.Kvs[0].Value), "want key value pair 'hoo' 'bar2' but got %+v", resp.Kvs)
	})
}

func TestAuthWriteKey(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoError(t, cc.Put(ctx, "foo", "a", config.PutOptions{}))
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")

		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		// confirm root role can access to all keys
		require.NoError(t, rootAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}))
		resp, err := rootAuthClient.Get(ctx, "foo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "foo", string(resp.Kvs[0].Key), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		require.Equalf(t, "bar", string(resp.Kvs[0].Value), "want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		// try invalid user
		_, err = clus.Client(WithAuth("a", "b"))
		require.ErrorContains(t, err, AuthenticationFailed)

		// try good user
		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar2", config.PutOptions{}))
		// confirm put succeeded
		resp, err = testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, resp.Kvs, 1, "want key value pair 'foo' 'bar2' but got %+v", resp.Kvs)
		require.Equalf(t, "foo", string(resp.Kvs[0].Key), "want key value pair 'foo' 'bar2' but got %+v", resp.Kvs)
		require.Equalf(t, "bar2", string(resp.Kvs[0].Value), "want key value pair 'foo' 'bar2' but got %+v", resp.Kvs)

		// try bad password
		_, err = clus.Client(WithAuth(testUserName, "badpass"))
		require.ErrorContains(t, err, AuthenticationFailed)
	})
}

func TestAuthTxn(t *testing.T) {
	tcs := []struct {
		name string
		cfg  config.ClusterConfig
	}{
		{
			"NoJWT",
			config.ClusterConfig{ClusterSize: 1},
		},
		{
			"JWT",
			config.ClusterConfig{ClusterSize: 1, AuthToken: defaultAuthToken},
		},
	}

	reqs := []txnReq{
		{
			compare:       []string{`version("c2") = "1"`},
			ifSuccess:     []string{"get s2"},
			ifFail:        []string{"get f2"},
			expectResults: []string{"SUCCESS", "s2", "v"},
			expectError:   false,
		},
		// a key of compare case isn't granted
		{
			compare:       []string{`version("c1") = "1"`},
			ifSuccess:     []string{"get s2"},
			ifFail:        []string{"get f2"},
			expectResults: []string{PermissionDenied},
			expectError:   true,
		},
		// a key of success case isn't granted
		{
			compare:       []string{`version("c2") = "1"`},
			ifSuccess:     []string{"get s1"},
			ifFail:        []string{"get f2"},
			expectResults: []string{PermissionDenied},
			expectError:   true,
		},
		// a key of failure case isn't granted
		{
			compare:       []string{`version("c2") = "1"`},
			ifSuccess:     []string{"get s2"},
			ifFail:        []string{"get f1"},
			expectResults: []string{PermissionDenied},
			expectError:   true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testRunner.BeforeTest(t)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.cfg))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())
			testutils.ExecuteUntil(ctx, t, func() {
				// keys with 1 suffix aren't granted to test-user
				keys := []string{"c1", "s1", "f1"}
				// keys with 2 suffix are granted to test-user, see Line 399
				grantedKeys := []string{"c2", "s2", "f2"}
				for _, key := range keys {
					err := cc.Put(ctx, key, "v", config.PutOptions{})
					require.NoError(t, err)
				}
				for _, key := range grantedKeys {
					err := cc.Put(ctx, key, "v", config.PutOptions{})
					require.NoError(t, err)
				}

				require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
				rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
				testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

				// grant keys to test-user
				for _, key := range grantedKeys {
					_, err := rootAuthClient.RoleGrantPermission(ctx, testRoleName, key, "", clientv3.PermissionType(clientv3.PermReadWrite))
					require.NoError(t, err)
				}
				for _, req := range reqs {
					resp, err := testUserAuthClient.Txn(ctx, req.compare, req.ifSuccess, req.ifFail, config.TxnOptions{
						Interactive: true,
					})
					if req.expectError {
						require.ErrorContains(t, err, req.expectResults[0])
					} else {
						require.NoError(t, err)
						require.Equal(t, req.expectResults, getRespValues(resp))
					}
				}
			})
		})
	}
}

func TestAuthPrefixPerm(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))
		prefix := "/prefix/" // directory like prefix
		// grant keys to test-user
		_, err := rootAuthClient.RoleGrantPermission(ctx, "test-role", prefix, clientv3.GetPrefixRangeEnd(prefix), clientv3.PermissionType(clientv3.PermReadWrite))
		require.NoError(t, err)
		// try a prefix granted permission
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("%s%d", prefix, i)
			require.NoError(t, testUserAuthClient.Put(ctx, key, "val", config.PutOptions{}))
		}
		// expect put 'key with prefix end "/prefix0"' value failed
		require.ErrorContains(t, testUserAuthClient.Put(ctx, clientv3.GetPrefixRangeEnd(prefix), "baz", config.PutOptions{}), PermissionDenied)

		// grant the prefix2 keys to test-user
		prefix2 := "/prefix2/"
		_, err = rootAuthClient.RoleGrantPermission(ctx, "test-role", prefix2, clientv3.GetPrefixRangeEnd(prefix2), clientv3.PermissionType(clientv3.PermReadWrite))
		require.NoError(t, err)
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("%s%d", prefix2, i)
			require.NoError(t, testUserAuthClient.Put(ctx, key, "val", config.PutOptions{}))
		}
	})
}

func TestAuthLeaseKeepAlive(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

		resp, err := rootAuthClient.Grant(ctx, 10)
		require.NoError(t, err)
		leaseID := resp.ID
		require.NoError(t, rootAuthClient.Put(ctx, "key", "value", config.PutOptions{LeaseID: leaseID}))
		_, err = rootAuthClient.KeepAliveOnce(ctx, leaseID)
		require.NoError(t, err)

		gresp, err := rootAuthClient.Get(ctx, "key", config.GetOptions{})
		require.NoError(t, err)
		require.Lenf(t, gresp.Kvs, 1, "want kv pair ('key', 'value') but got %v", gresp.Kvs)
		require.Equalf(t, "key", string(gresp.Kvs[0].Key), "want kv pair ('key', 'value') but got %v", gresp.Kvs)
		require.Equalf(t, "value", string(gresp.Kvs[0].Value), "want kv pair ('key', 'value') but got %v", gresp.Kvs)
	})
}

func TestAuthRevokeWithDelete(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		// create a new role
		newTestRoleName := "test-role2"
		_, err := rootAuthClient.RoleAdd(ctx, newTestRoleName)
		require.NoError(t, err)
		// grant the new role to the user
		_, err = rootAuthClient.UserGrantRole(ctx, testUserName, newTestRoleName)
		require.NoError(t, err)
		// check the result
		resp, err := rootAuthClient.UserGet(ctx, testUserName)
		require.NoError(t, err)
		require.ElementsMatch(t, resp.Roles, []string{testRoleName, newTestRoleName})
		// delete the role, test-role2 must be revoked from test-user
		_, err = rootAuthClient.RoleDelete(ctx, newTestRoleName)
		require.NoError(t, err)
		// check the result
		resp, err = rootAuthClient.UserGet(ctx, testUserName)
		require.NoError(t, err)
		require.ElementsMatch(t, resp.Roles, []string{testRoleName})
	})
}

func TestAuthLeaseTimeToLiveExpired(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		resp, err := rootAuthClient.Grant(ctx, 2)
		require.NoError(t, err)
		leaseID := resp.ID
		require.NoError(t, rootAuthClient.Put(ctx, "key", "val", config.PutOptions{LeaseID: leaseID}))
		// eliminate false positive
		time.Sleep(3 * time.Second)
		tresp, err := rootAuthClient.TimeToLive(ctx, leaseID, config.LeaseOption{})
		require.NoError(t, err)
		require.Equal(t, int64(-1), tresp.TTL)

		gresp, err := rootAuthClient.Get(ctx, "key", config.GetOptions{})
		require.NoError(t, err)
		require.Empty(t, gresp.Kvs)
	})
}

func TestAuthLeaseGrantLeases(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []testCase{
		{
			name:   "NoJWT",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "JWT",
			config: config.ClusterConfig{ClusterSize: 1, AuthToken: defaultAuthToken},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
				rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

				resp, err := rootAuthClient.Grant(ctx, 10)
				require.NoError(t, err)

				leaseID := resp.ID
				lresp, err := rootAuthClient.Leases(ctx)
				require.NoError(t, err)
				require.Lenf(t, lresp.Leases, 1, "want %v leaseID but got %v leases", leaseID, lresp.Leases)
				require.Equalf(t, lresp.Leases[0].ID, leaseID, "want %v leaseID but got %v leases", leaseID, lresp.Leases)
			})
		})
	}
}

func TestAuthMemberAdd(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))
		_, err := testUserAuthClient.MemberAdd(ctx, "newmember", []string{testPeerURL})
		require.ErrorContains(t, err, PermissionDenied)
		_, err = rootAuthClient.MemberAdd(ctx, "newmember", []string{testPeerURL})
		require.NoError(t, err)
	})
}

func TestAuthMemberRemove(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clusterSize := 3
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: clusterSize}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		memberIDToEndpoints := getMemberIDToEndpoints(ctx, t, clus)
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

		memberID, clusterID := memberToRemove(ctx, t, rootAuthClient, clusterSize)
		delete(memberIDToEndpoints, memberID)
		endpoints := make([]string, 0, len(memberIDToEndpoints))
		for _, ep := range memberIDToEndpoints {
			endpoints = append(endpoints, ep)
		}
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))
		// ordinary user cannot remove a member
		_, err := testUserAuthClient.MemberRemove(ctx, memberID)
		require.ErrorContains(t, err, PermissionDenied)

		// root can remove a member, building a client excluding removed member endpoint
		rootAuthClient2 := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword), WithEndpoints(endpoints)))
		resp, err := rootAuthClient2.MemberRemove(ctx, memberID)
		require.NoError(t, err)
		require.Equal(t, resp.Header.ClusterId, clusterID)
		found := false
		for _, member := range resp.Members {
			if member.ID == memberID {
				found = true
				break
			}
		}
		require.Falsef(t, found, "expect removed member not found in member remove response")
	})
}

func TestAuthTestInvalidMgmt(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		_, err := rootAuthClient.UserDelete(ctx, rootUserName)
		require.ErrorContains(t, err, InvalidAuthManagement)
		_, err = rootAuthClient.UserRevokeRole(ctx, rootUserName, rootRoleName)
		require.ErrorContains(t, err, InvalidAuthManagement)
	})
}

func TestAuthLeaseRevoke(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

		lresp, err := rootAuthClient.Grant(ctx, 10)
		require.NoError(t, err)
		err = rootAuthClient.Put(ctx, "key", "value", config.PutOptions{LeaseID: lresp.ID})
		require.NoError(t, err)

		_, err = rootAuthClient.Revoke(ctx, lresp.ID)
		require.NoError(t, err)

		_, err = rootAuthClient.Get(ctx, "key", config.GetOptions{})
		require.NoError(t, err)
	})
}

func TestAuthRoleGet(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		resp, err := rootAuthClient.RoleGet(ctx, testRoleName)
		require.NoError(t, err)
		requireRolePermissionEqual(t, testRole, resp.Perm)

		// test-user can get the information of test-role because it belongs to the role
		resp, err = testUserAuthClient.RoleGet(ctx, testRoleName)
		require.NoError(t, err)
		requireRolePermissionEqual(t, testRole, resp.Perm)
		// test-user cannot get the information of root because it doesn't belong to the role
		_, err = testUserAuthClient.RoleGet(ctx, rootRoleName)
		require.ErrorContains(t, err, PermissionDenied)
	})
}

func TestAuthUserGet(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		resp, err := rootAuthClient.UserGet(ctx, testUserName)
		require.NoError(t, err)
		requireUserRolesEqual(t, testUser, resp.Roles)

		// test-user can get the information of test-user itself
		resp, err = testUserAuthClient.UserGet(ctx, testUserName)
		require.NoError(t, err)
		requireUserRolesEqual(t, testUser, resp.Roles)
		// test-user cannot get the information of root
		_, err = testUserAuthClient.UserGet(ctx, rootUserName)
		require.ErrorContains(t, err, PermissionDenied)
	})
}

func TestAuthRoleList(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

		resp, err := rootAuthClient.RoleList(ctx)
		require.NoError(t, err)
		requireUserRolesEqual(t, testUser, resp.Roles)
	})
}

func TestAuthJWTExpire(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, AuthToken: defaultAuthToken}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}))
		// wait an expiration of my JWT token
		<-time.After(3 * tokenTTL)

		// e2e test will generate a new token while
		// integration test that re-uses the same etcd client will refresh the token on server failure.
		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}))
	})
}

// TestAuthRevisionConsistency ensures auth revision is the same after member restarts
func TestAuthRevisionConsistency(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, AuthToken: defaultAuthToken}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

		// add user
		_, err := rootAuthClient.UserAdd(ctx, testUserName, testPassword, config.UserAddOptions{})
		require.NoError(t, err)
		// delete the same user
		_, err = rootAuthClient.UserDelete(ctx, testUserName)
		require.NoError(t, err)

		// get node0 auth revision
		aresp, err := rootAuthClient.AuthStatus(ctx)
		require.NoError(t, err)
		oldAuthRevision := aresp.AuthRevision

		// restart the node
		clus.Members()[0].Stop()
		require.NoError(t, clus.Members()[0].Start(ctx))

		aresp2, err := rootAuthClient.AuthStatus(ctx)
		require.NoError(t, err)
		newAuthRevision := aresp2.AuthRevision

		require.Equal(t, oldAuthRevision, newAuthRevision)
	})
}

// TestAuthTestCacheReload ensures permissions are persisted and will be reloaded after member restarts
func TestAuthTestCacheReload(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, AuthToken: defaultAuthToken}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		// create foo since that is within the permission set
		// expectation is to succeed
		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}))

		// restart the node
		clus.Members()[0].Stop()
		require.NoError(t, clus.Members()[0].Start(ctx))

		// nothing has changed, but it fails without refreshing cache after restart
		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar2", config.PutOptions{}))
	})
}

// TestAuthLeaseTimeToLive gated lease time to live with RBAC control
func TestAuthLeaseTimeToLive(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, AuthToken: defaultAuthToken}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{testRole}, []authUser{rootUser, testUser}), "failed to enable auth")
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth(testUserName, testPassword)))

		gresp, err := testUserAuthClient.Grant(ctx, 10)
		require.NoError(t, err)
		leaseID := gresp.ID

		require.NoError(t, testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{LeaseID: leaseID}))
		_, err = testUserAuthClient.TimeToLive(ctx, leaseID, config.LeaseOption{WithAttachedKeys: true})
		require.NoError(t, err)

		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))
		require.NoError(t, rootAuthClient.Put(ctx, "bar", "foo", config.PutOptions{LeaseID: leaseID}))

		// the lease is attached to bar, which test-user cannot access
		_, err = testUserAuthClient.TimeToLive(ctx, leaseID, config.LeaseOption{WithAttachedKeys: true})
		require.Errorf(t, err, "test-user must not be able to access to the lease, because it's attached to the key bar")

		// without --keys, access should be allowed
		_, err = testUserAuthClient.TimeToLive(ctx, leaseID, config.LeaseOption{WithAttachedKeys: false})
		require.NoError(t, err)
	})
}

func mustAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}
