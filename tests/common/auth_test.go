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

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestAuthEnable(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "hoo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'hoo', 'bar' but got %+v", resp.Kvs)
		}
	})
}

func TestAuthGracefulDisable(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		require.NoErrorf(t, setupAuth(cc, []authRole{}, []authUser{rootUser}), "failed to enable auth")
		donec := make(chan struct{})
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth(rootUserName, rootPassword)))

		go func() {
			defer close(donec)
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
			// the watcher should still work after reconnecting
			require.NoErrorf(t, rootAuthClient.Put(ctx, "key", "value", config.PutOptions{}), "failed to put key value")
		}()

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
