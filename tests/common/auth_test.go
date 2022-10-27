package common

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

var defaultAuthToken = fmt.Sprintf("jwt,pub-key=%s,priv-key=%s,sign-method=RS256,ttl=1s",
	mustAbsPath("../fixtures/server.crt"), mustAbsPath("../fixtures/server.key.insecure"))

func TestAuthEnable(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
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
		err := cc.Put(ctx, "hoo", "a", config.PutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		authEnable(ctx, cc, t)

		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		// test-user doesn't have the permission, it must fail
		if err = testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}); err == nil {
			t.Fatalf("want error but got nil")
		}
		if err = rootAuthClient.AuthDisable(ctx); err != nil {
			t.Fatalf("failed to auth disable %v", err)
		}
		// now ErrAuthNotEnabled of Authenticate() is simply ignored
		if err = testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// now the key can be accessed
		if err = cc.Put(ctx, "hoo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// confirm put succeeded
		resp, err := cc.Get(ctx, "hoo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
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
		authEnable(ctx, cc, t)
		donec := make(chan struct{})
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))

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
			if err := rootAuthClient.Put(ctx, "key", "value", config.PutOptions{}); err != nil {
				t.Errorf("failed to put key value %v", err)
			}
		}()

		wCtx, wCancel := context.WithCancel(ctx)
		defer wCancel()

		watchCh := rootAuthClient.Watch(wCtx, "key", config.WatchOptions{Revision: 1})
		wantedLen := 1
		watchTimeout := 10 * time.Second
		wanted := []testutils.KV{{Key: "key", Val: "value"}}
		kvs, err := testutils.KeyValuesFromWatchChan(watchCh, wantedLen, watchTimeout)
		if err != nil {
			t.Fatalf("failed to get key-values from watch channel %s", err)
		}
		assert.Equal(t, wanted, kvs)
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
		if err != nil {
			t.Fatal(err)
		}
		if resp.Enabled {
			t.Fatal("want not enabled but enabled")
		}
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		resp, err = rootAuthClient.AuthStatus(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !resp.Enabled {
			t.Fatalf("want enabled but got not enabled")
		}
	})
}

func TestAuthRoleUpdate(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		if err := cc.Put(ctx, "foo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		// try put to not granted key
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		putFailPerm(ctx, testUserAuthClient, "hoo", "bar", t)
		// grant a new key
		if _, err := rootAuthClient.RoleGrantPermission(ctx, "test-role", "hoo", "", clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
			t.Fatal(err)
		}
		// try a newly granted key
		if err := testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// confirm put succeeded
		resp, err := testUserAuthClient.Get(ctx, "hoo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "hoo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		}
		// revoke the newly granted key
		if _, err = rootAuthClient.RoleRevokePermission(ctx, "test-role", "hoo", ""); err != nil {
			t.Fatal(err)
		}
		// try put to the revoked key
		putFailPerm(ctx, testUserAuthClient, "hoo", "bar", t)
		// confirm a key still granted can be accessed
		resp, err = testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "foo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		}
	})
}

func TestAuthUserDeleteDuringOps(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		if err := cc.Put(ctx, "foo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)

		// create a key
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		if err := testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// confirm put succeeded
		resp, err := testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "foo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		}
		// delete the user
		if _, err = rootAuthClient.UserDelete(ctx, "test-user"); err != nil {
			t.Fatal(err)
		}
		// check the user is deleted
		err = testUserAuthClient.Put(ctx, "foo", "baz", config.PutOptions{})
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrAuthFailed.Error()) {
			t.Errorf("want error %s but got %v", rpctypes.ErrAuthFailed.Error(), err)
		}
	})
}

func TestAuthRoleRevokeDuringOps(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		if err := cc.Put(ctx, "foo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		// create a key
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		if err := testUserAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// confirm put succeeded
		resp, err := testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "foo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		}
		// create a new role
		if _, err = rootAuthClient.RoleAdd(ctx, "test-role2"); err != nil {
			t.Fatal(err)
		}
		// grant a new key to the new role
		if _, err = rootAuthClient.RoleGrantPermission(ctx, "test-role2", "hoo", "", clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
			t.Fatal(err)
		}
		// grant the new role to the user
		if _, err = rootAuthClient.UserGrantRole(ctx, "test-user", "test-role2"); err != nil {
			t.Fatal(err)
		}

		// try a newly granted key
		if err := testUserAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// confirm put succeeded
		resp, err = testUserAuthClient.Get(ctx, "hoo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "hoo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'hoo' 'bar' but got %+v", resp.Kvs)
		}
		// revoke a role from the user
		if _, err = rootAuthClient.UserRevokeRole(ctx, "test-user", "test-role"); err != nil {
			t.Fatal(err)
		}
		// check the role is revoked and permission is lost from the user
		putFailPerm(ctx, testUserAuthClient, "foo", "baz", t)

		// try a key that can be accessed from the remaining role
		if err := testUserAuthClient.Put(ctx, "hoo", "bar2", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// confirm put succeeded
		resp, err = testUserAuthClient.Get(ctx, "hoo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "hoo" || string(resp.Kvs[0].Value) != "bar2" {
			t.Fatalf("want key value pair 'hoo' 'bar2' but got %+v", resp.Kvs)
		}
	})
}

func TestAuthWriteKey(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		if err := cc.Put(ctx, "foo", "a", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)

		// confirm root role can access to all keys
		if err := rootAuthClient.Put(ctx, "foo", "bar", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		resp, err := rootAuthClient.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "foo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		}
		// try invalid user
		_, err = clus.Client(WithAuth("a", "b"))
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrAuthFailed.Error()) {
			t.Errorf("want error %s but got %v", rpctypes.ErrAuthFailed.Error(), err)
		}
		// confirm put failed
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		resp, err = testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "foo" || string(resp.Kvs[0].Value) != "bar" {
			t.Fatalf("want key value pair 'foo' 'bar' but got %+v", resp.Kvs)
		}
		// try good user
		if err = testUserAuthClient.Put(ctx, "foo", "bar2", config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
		// confirm put succeeded
		resp, err = testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "foo" || string(resp.Kvs[0].Value) != "bar2" {
			t.Fatalf("want key value pair 'foo' 'bar2' but got %+v", resp.Kvs)
		}
		// try bad password
		_, err = clus.Client(WithAuth("test-user", "badpass"))
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrAuthFailed.Error()) {
			t.Errorf("want error %s but got %v", rpctypes.ErrAuthFailed.Error(), err)
		}
		// confirm put failed
		resp, err = testUserAuthClient.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Kvs) != 1 || string(resp.Kvs[0].Key) != "foo" || string(resp.Kvs[0].Value) != "bar2" {
			t.Fatalf("want key value pair 'foo' 'bar2' but got %+v", resp.Kvs)
		}
	})
}

// TestAuthEmptyUserGet ensures that a get with an empty user will return an empty user error.
func TestAuthEmptyUserGet(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		_, err := cc.Get(ctx, "abc", config.GetOptions{})
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrUserEmpty.Error()) {
			t.Errorf("want error %s but got %v", rpctypes.ErrUserEmpty.Error(), err)
		}
	})
}

// TestAuthEmptyUserPut ensures that a put with an empty user will return an empty user error,
// and the consistent_index should be moved forward even the apply-->Put fails.
func TestAuthEmptyUserPut(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, SnapshotCount: 3}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		// The SnapshotCount is 3, so there must be at least 3 new snapshot files being created.
		// The VERIFY logic will check whether the consistent_index >= last snapshot index on
		// cluster terminating.
		for i := 0; i < 10; i++ {
			err := cc.Put(ctx, "foo", "bar", config.PutOptions{})
			if err == nil || !strings.Contains(err.Error(), rpctypes.ErrUserEmpty.Error()) {
				t.Errorf("want error %s but got %v", rpctypes.ErrUserEmpty.Error(), err)
			}
		}
	})
}

// TestAuthTokenWithDisable tests that auth won't crash if
// given a valid token when authentication is disabled
func TestAuthTokenWithDisable(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		rctx, cancel := context.WithCancel(context.TODO())
		donec := make(chan struct{})
		go func() {
			defer close(donec)
			for rctx.Err() == nil {
				rootAuthClient.Put(ctx, "abc", "def", config.PutOptions{})
			}
		}()
		time.Sleep(10 * time.Millisecond)
		if err := rootAuthClient.AuthDisable(ctx); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
		cancel()
		<-donec
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
			compare:   []string{`version("c2") = "1"`},
			ifSuccess: []string{"get s2"},
			ifFail:    []string{"get f2"},
			results:   []string{"SUCCESS", "s2", "v"},
		},
		// a key of compare case isn't granted
		{
			compare:   []string{`version("c1") = "1"`},
			ifSuccess: []string{"get s2"},
			ifFail:    []string{"get f2"},
			results:   []string{"etcdserver: permission denied"},
		},
		// a key of success case isn't granted
		{
			compare:   []string{`version("c2") = "1"`},
			ifSuccess: []string{"get s1"},
			ifFail:    []string{"get f2"},
			results:   []string{"etcdserver: permission denied"},
		},
		// a key of failure case isn't granted
		{
			compare:   []string{`version("c2") = "1"`},
			ifSuccess: []string{"get s2"},
			ifFail:    []string{"get f1"},
			results:   []string{"etcdserver: permission denied"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testRunner.BeforeTest(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.cfg))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())
			testutils.ExecuteUntil(ctx, t, func() {
				// keys with 1 suffix aren't granted to test-user
				// keys with 2 suffix are granted to test-user

				keys := []string{"c1", "s1", "f1"}
				grantedKeys := []string{"c2", "s2", "f2"}
				for _, key := range keys {
					if err := cc.Put(ctx, key, "v", config.PutOptions{}); err != nil {
						t.Fatal(err)
					}
				}
				for _, key := range grantedKeys {
					if err := cc.Put(ctx, key, "v", config.PutOptions{}); err != nil {
						t.Fatal(err)
					}
				}
				authEnable(ctx, cc, t)
				rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
				authSetupDefaultTestUser(ctx, rootAuthClient, t)
				// grant keys to test-user
				for _, key := range grantedKeys {
					if _, err := rootAuthClient.RoleGrantPermission(ctx, "test-role", key, "", clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
						t.Fatal(err)
					}
				}
				testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
				for _, req := range reqs {
					resp, err := testUserAuthClient.Txn(ctx, req.compare, req.ifSuccess, req.ifFail, config.TxnOptions{
						Interactive: true,
					})
					if strings.Contains(req.results[0], "denied") {
						assert.Contains(t, err.Error(), req.results[0])
					} else {
						assert.NoError(t, err)
						assert.Equal(t, req.results, getRespValues(resp))
					}
				}
			})
		})
	}
}

func TestAuthPrefixPerm(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		prefix := "/prefix/" // directory like prefix
		// grant keys to test-user
		if _, err := rootAuthClient.RoleGrantPermission(ctx, "test-role", prefix, clientv3.GetPrefixRangeEnd(prefix), clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
			t.Fatal(err)
		}
		// try a prefix granted permission
		testUserClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("%s%d", prefix, i)
			if err := testUserClient.Put(ctx, key, "val", config.PutOptions{}); err != nil {
				t.Fatal(err)
			}
		}
		putFailPerm(ctx, testUserClient, clientv3.GetPrefixRangeEnd(prefix), "baz", t)
		// grant the prefix2 keys to test-user
		prefix2 := "/prefix2/"
		if _, err := rootAuthClient.RoleGrantPermission(ctx, "test-role", prefix2, clientv3.GetPrefixRangeEnd(prefix2), clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("%s%d", prefix2, i)
			if err := testUserClient.Put(ctx, key, "val", config.PutOptions{}); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestAuthRevokeWithDelete(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		// create a new role
		if _, err := rootAuthClient.RoleAdd(ctx, "test-role2"); err != nil {
			t.Fatal(err)
		}
		// grant the new role to the user
		if _, err := rootAuthClient.UserGrantRole(ctx, "test-user", "test-role2"); err != nil {
			t.Fatal(err)
		}
		// check the result
		resp, err := rootAuthClient.UserGet(ctx, "test-user")
		if err != nil {
			t.Fatal(err)
		}
		assert.ElementsMatch(t, resp.Roles, []string{"test-role", "test-role2"})
		// delete the role, test-role2 must be revoked from test-user
		if _, err := rootAuthClient.RoleDelete(ctx, "test-role2"); err != nil {
			t.Fatal(err)
		}
		// check the result
		resp, err = rootAuthClient.UserGet(ctx, "test-user")
		if err != nil {
			t.Fatal(err)
		}
		assert.ElementsMatch(t, resp.Roles, []string{"test-role"})
	})
}

func TestAuthInvalidMgmt(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		_, err := rootAuthClient.RoleDelete(ctx, "root")
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrInvalidAuthMgmt.Error()) {
			t.Fatalf("want %v error but got %v error", rpctypes.ErrInvalidAuthMgmt, err)
		}
		_, err = rootAuthClient.UserRevokeRole(ctx, "root", "root")
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrInvalidAuthMgmt.Error()) {
			t.Fatalf("want %v error but got %v error", rpctypes.ErrInvalidAuthMgmt, err)
		}
	})
}

func TestAuthLeaseTestKeepAlive(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		resp, err := rootAuthClient.Grant(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		leaseID := resp.ID
		if err = rootAuthClient.Put(ctx, "key", "value", config.PutOptions{LeaseID: leaseID}); err != nil {
			t.Fatal(err)
		}
		if _, err = rootAuthClient.KeepAliveOnce(ctx, leaseID); err != nil {
			t.Fatal(err)
		}
		gresp, err := rootAuthClient.Get(ctx, "key", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(gresp.Kvs) != 1 || string(gresp.Kvs[0].Key) != "key" || string(gresp.Kvs[0].Value) != "value" {
			t.Fatalf("want kv pair ('key', 'value') but got %v", gresp.Kvs)
		}
	})
}

func TestAuthLeaseTestTimeToLiveExpired(t *testing.T) {
	tcs := []struct {
		name       string
		JWTEnabled bool
	}{
		{
			name:       "JWTEnabled",
			JWTEnabled: true,
		},
		{
			name:       "JWTDisabled",
			JWTEnabled: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testRunner.BeforeTest(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())
			testutils.ExecuteUntil(ctx, t, func() {
				authEnable(ctx, cc, t)
				rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
				authSetupDefaultTestUser(ctx, rootAuthClient, t)
				resp, err := rootAuthClient.Grant(ctx, 2)
				if err != nil {
					t.Fatal(err)
				}
				leaseID := resp.ID
				if err = rootAuthClient.Put(ctx, "key", "val", config.PutOptions{LeaseID: leaseID}); err != nil {
					t.Fatal(err)
				}
				// eliminate false positive
				time.Sleep(3 * time.Second)
				tresp, err := rootAuthClient.TimeToLive(ctx, leaseID, config.LeaseOption{})
				if err != nil {
					t.Fatal(err)
				}
				if tresp.TTL != -1 {
					t.Fatalf("want leaseID %v expired but not", leaseID)
				}
				gresp, err := rootAuthClient.Get(ctx, "key", config.GetOptions{})
				if err != nil || len(gresp.Kvs) != 0 {
					t.Fatalf("want nil err and no kvs but got (%v) error and %d kvs", err, len(gresp.Kvs))
				}
			})
		})
	}
}

func TestAuthLeaseGrantLeases(t *testing.T) {
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
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testRunner.BeforeTest(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.cfg))
			defer clus.Close()
			testutils.ExecuteUntil(ctx, t, func() {
				rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
				authSetupDefaultTestUser(ctx, rootAuthClient, t)
				resp, err := rootAuthClient.Grant(ctx, 10)
				if err != nil {
					t.Fatal(err)
				}
				leaseID := resp.ID
				lresp, err := rootAuthClient.Leases(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if len(lresp.Leases) != 1 || lresp.Leases[0].ID != leaseID {
					t.Fatalf("want %v leaseID but got %v leases", leaseID, lresp.Leases)
				}
			})
		})
	}
}

func TestAuthLeaseAttach(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		users := []struct {
			name     string
			password string
			role     string
			key      string
			end      string
		}{
			{
				name:     "user1",
				password: "user1-123",
				role:     "role1",
				key:      "k1",
				end:      "k3",
			},
			{
				name:     "user2",
				password: "user2-123",
				role:     "role2",
				key:      "k2",
				end:      "k4",
			},
		}
		for _, user := range users {
			authSetupTestUser(ctx, cc, user.name, user.password, user.role, user.key, user.end, t)
		}
		authEnable(ctx, cc, t)
		user1c := testutils.MustClient(clus.Client(WithAuth("user1", "user1-123")))
		user2c := testutils.MustClient(clus.Client(WithAuth("user2", "user2-123")))
		leaseResp, err := user1c.Grant(ctx, 90)
		testutil.AssertNil(t, err)
		leaseID := leaseResp.ID
		// permission of k2 is also granted to user2
		err = user1c.Put(ctx, "k2", "val", config.PutOptions{LeaseID: leaseID})
		testutil.AssertNil(t, err)
		_, err = user2c.Revoke(ctx, leaseID)
		testutil.AssertNil(t, err)

		leaseResp, err = user1c.Grant(ctx, 90)
		testutil.AssertNil(t, err)
		leaseID = leaseResp.ID
		// permission of k1 isn't granted to user2
		err = user1c.Put(ctx, "k1", "val", config.PutOptions{LeaseID: leaseID})
		testutil.AssertNil(t, err)
		_, err = user2c.Revoke(ctx, leaseID)
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrPermissionDenied.Error()) {
			t.Fatalf("want %v error but got %v error", rpctypes.ErrPermissionDenied, err)
		}
	})
}

func TestAuthLeaseRevoke(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	testutils.ExecuteUntil(ctx, t, func() {
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		// put with TTL 10 seconds and revoke
		resp, err := rootAuthClient.Grant(ctx, 10)
		if err != nil {
			t.Fatal(err)
		}
		leaseID := resp.ID
		if err = rootAuthClient.Put(ctx, "key", "val", config.PutOptions{LeaseID: leaseID}); err != nil {
			t.Fatal(err)
		}
		if _, err = rootAuthClient.Revoke(ctx, leaseID); err != nil {
			t.Fatal(err)
		}
		gresp, err := rootAuthClient.Get(ctx, "key", config.GetOptions{})
		if err != nil || len(gresp.Kvs) != 0 {
			t.Fatalf("want nil err and no kvs but got (%v) error and %d kvs", err, len(gresp.Kvs))
		}
	})
}

func TestAuthRoleGet(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		if _, err := rootAuthClient.RoleGet(ctx, "test-role"); err != nil {
			t.Fatal(err)
		}
		// test-user can get the information of test-role because it belongs to the role
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		if _, err := testUserAuthClient.RoleGet(ctx, "test-role"); err != nil {
			t.Fatal(err)
		}
		// test-user cannot get the information of root because it doesn't belong to the role
		_, err := testUserAuthClient.RoleGet(ctx, "root")
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrPermissionDenied.Error()) {
			t.Fatalf("want %v error but got %v", rpctypes.ErrPermissionDenied, err)
		}
	})
}

func TestAuthUserGet(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		resp, err := rootAuthClient.UserGet(ctx, "test-user")
		if err != nil {
			t.Fatal(err)
		}
		assert.ElementsMatch(t, resp.Roles, []string{"test-role"})
		// test-user can get the information of test-user itself
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		resp, err = testUserAuthClient.UserGet(ctx, "test-user")
		if err != nil {
			t.Fatal(err)
		}
		assert.ElementsMatch(t, resp.Roles, []string{"test-role"})
		// test-user cannot get the information of root
		_, err = testUserAuthClient.UserGet(ctx, "root")
		if err == nil || !strings.Contains(err.Error(), rpctypes.ErrPermissionDenied.Error()) {
			t.Fatalf("want %v error but got %v", rpctypes.ErrPermissionDenied, err)
		}
	})
}

func TestAuthRoleList(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		resp, err := rootAuthClient.RoleList(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assert.ElementsMatch(t, resp.Roles, []string{"test-role"})
	})
}

func TestAuthDefrag(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		var kvs = []testutils.KV{{Key: "key", Val: "val1"}, {Key: "key", Val: "val2"}, {Key: "key", Val: "val3"}}
		for i := range kvs {
			if err := cc.Put(ctx, kvs[i].Key, kvs[i].Val, config.PutOptions{}); err != nil {
				t.Fatalf("TestAuthDefrag #%d: put kv error (%v)", i, err)
			}
		}
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		// ordinary user cannot defrag
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))
		if err := testUserAuthClient.Defragment(ctx, config.DefragOption{Timeout: 5 * time.Second}); err == nil {
			t.Fatal("want error but got no error")
		}
		// root can defrag
		if err := rootAuthClient.Defragment(ctx, config.DefragOption{Timeout: 5 * time.Second}); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAuthEndpointHealth(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))

		if err := rootAuthClient.Health(ctx); err != nil {
			t.Fatal(err)
		}
		// health checking with an ordinary user "succeeds" since permission denial goes through consensus
		if err := testUserAuthClient.Health(ctx); err != nil {
			t.Fatal(err)
		}
		// succeed if permissions granted for ordinary user
		if _, err := rootAuthClient.RoleGrantPermission(ctx, "test-role", "health", "", clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
			t.Fatal(err)
		}
		if err := testUserAuthClient.Health(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAuthWatch(t *testing.T) {
	watchTimeout := 1 * time.Second
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

	tests := []struct {
		puts     []testutils.KV
		watchKey string
		opts     config.WatchOptions
		want     bool
		wanted   []testutils.KV
	}{
		{ // watch 1 key, should be successful
			puts:     []testutils.KV{{Key: "key", Val: "value"}},
			watchKey: "key",
			opts:     config.WatchOptions{Revision: 1},
			want:     true,
			wanted:   []testutils.KV{{Key: "key", Val: "value"}},
		},
		{ // watch 3 keys by range, should be successful
			puts:     []testutils.KV{{Key: "key1", Val: "value1"}, {Key: "key3", Val: "value3"}, {Key: "key2", Val: "value2"}},
			watchKey: "key",
			opts:     config.WatchOptions{RangeEnd: "key3", Revision: 1},
			want:     true,
			wanted:   []testutils.KV{{Key: "key1", Val: "value1"}, {Key: "key2", Val: "value2"}},
		},
		{ // watch 1 key, should not be successful
			puts:     []testutils.KV{},
			watchKey: "key5",
			opts:     config.WatchOptions{Revision: 1},
			want:     false,
			wanted:   []testutils.KV{},
		},
		{ // watch 3 keys by range, should not be successful
			puts:     []testutils.KV{},
			watchKey: "key",
			opts:     config.WatchOptions{RangeEnd: "key6", Revision: 1},
			want:     false,
			wanted:   []testutils.KV{},
		},
	}

	for _, tc := range tcs {
		for i, tt := range tests {
			t.Run(tc.name, func(t *testing.T) {
				testRunner.BeforeTest(t)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.cfg))
				defer clus.Close()
				testutils.ExecuteUntil(ctx, t, func() {
					rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
					authSetupDefaultTestUser(ctx, rootAuthClient, t)

					_, err := rootAuthClient.RoleGrantPermission(ctx, "test-role", "key", "key4", clientv3.PermissionType(clientv3.PermReadWrite))
					assert.NoError(t, err)
					testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))

					donec := make(chan struct{})
					go func(i int, puts []testutils.KV) {
						defer close(donec)
						for j := range puts {
							if err := testUserAuthClient.Put(ctx, puts[j].Key, puts[j].Val, config.PutOptions{}); err != nil {
								t.Errorf("test #%d-%d: put error (%v)", i, j, err)
							}
						}
					}(i, tt.puts)
					wCtx, wCancel := context.WithCancel(ctx)
					wch := testUserAuthClient.Watch(wCtx, tt.watchKey, tt.opts)
					if wch == nil {
						t.Fatalf("failed to watch %s", tt.watchKey)
					}
					kvs, err := testutils.KeyValuesFromWatchChan(wch, len(tt.wanted), watchTimeout)
					if err != nil {
						wCancel()
						assert.False(t, tt.want)
					} else {
						assert.Equal(t, tt.wanted, kvs)
					}
					wCancel()
					<-donec
				})
			})
		}
	}
}

func TestAuthJWTExpire(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, AuthToken: defaultAuthToken}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		// try a granted key
		if err := rootAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}); err != nil {
			t.Error(err)
		}
		// wait an expiration of my JWT token
		<-time.After(3 * time.Second)
		if err := rootAuthClient.Put(ctx, "hoo", "bar", config.PutOptions{}); err != nil {
			t.Error(err)
		}
	})
}

func TestAuthMemberRemove(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 3}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		authSetupDefaultTestUser(ctx, rootAuthClient, t)
		testUserAuthClient := testutils.MustClient(clus.Client(WithAuth("test-user", "pass")))

		memberList, err := rootAuthClient.MemberList(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(memberList.Members), "want 3 member but got %d", len(memberList.Members))
		id := memberList.Members[0].ID

		// 5 seconds is the minimum required amount of time peer is considered active
		time.Sleep(5 * time.Second)
		if _, err := testUserAuthClient.MemberRemove(ctx, id); err == nil {
			t.Fatalf("ordinary user must not be allowed to remove a member")
		}
		if _, err := rootAuthClient.MemberRemove(ctx, id); err != nil {
			t.Fatal(err)
		}
	})
}

// TestAuthRevisionConsistency ensures authRevision is the same after etcd restart
func TestAuthRevisionConsistency(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		// add user
		if _, err := rootAuthClient.UserAdd(ctx, "test-user", "pass", config.UserAddOptions{}); err != nil {
			t.Fatal(err)
		}
		// delete the same user
		if _, err := rootAuthClient.UserDelete(ctx, "test-user"); err != nil {
			t.Fatal(err)
		}
		sresp, err := rootAuthClient.AuthStatus(ctx)
		if err != nil {
			t.Fatal(err)
		}
		oldAuthRevision := sresp.AuthRevision
		// restart the node
		m := clus.Members()[0]
		m.Stop()
		if err = m.Start(ctx); err != nil {
			t.Fatal(err)
		}
		sresp, err = rootAuthClient.AuthStatus(ctx)
		if err != nil {
			t.Fatal(err)
		}
		newAuthRevision := sresp.AuthRevision
		// assert AuthRevision equal
		if newAuthRevision != oldAuthRevision {
			t.Fatalf("auth revison shouldn't change when restarting etcd, expected: %d, got: %d", oldAuthRevision, newAuthRevision)
		}
	})
}

// TestAuthKVRevision ensures kv revision is the same after auth mutating operations
func TestAuthKVRevision(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		err := cc.Put(ctx, "foo", "bar", config.PutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		gresp, err := cc.Get(ctx, "foo", config.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		rev := gresp.Header.Revision
		aresp, aerr := cc.UserAdd(ctx, "root", "123", config.UserAddOptions{NoPassword: false})
		if aerr != nil {
			t.Fatal(err)
		}
		if aresp.Header.Revision != rev {
			t.Fatalf("revision want %d, got %d", rev, aresp.Header.Revision)
		}
	})
}

// TestAuthConcurrent ensures concurrent auth ops don't cause old authRevision errors
func TestAuthRevConcurrent(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		authEnable(ctx, cc, t)
		rootAuthClient := testutils.MustClient(clus.Client(WithAuth("root", "root")))
		var wg sync.WaitGroup
		f := func(i int) {
			defer wg.Done()
			role, user := fmt.Sprintf("test-role-%d", i), fmt.Sprintf("test-user-%d", i)
			_, err := rootAuthClient.RoleAdd(ctx, role)
			testutil.AssertNil(t, err)
			_, err = rootAuthClient.RoleGrantPermission(ctx, role, "a", clientv3.GetPrefixRangeEnd("a"), clientv3.PermissionType(clientv3.PermReadWrite))
			testutil.AssertNil(t, err)
			_, err = rootAuthClient.UserAdd(ctx, user, "123", config.UserAddOptions{NoPassword: false})
			testutil.AssertNil(t, err)
			err = rootAuthClient.Put(ctx, "a", "b", config.PutOptions{})
			testutil.AssertNil(t, err)
		}
		// needs concurrency to trigger
		numRoles := 2
		wg.Add(numRoles)
		for i := 0; i < numRoles; i++ {
			go f(i)
		}
		wg.Wait()
	})
}

func authEnable(ctx context.Context, cc interfaces.Client, t *testing.T) {
	// create root user with root role
	_, err := cc.UserAdd(ctx, "root", "root", config.UserAddOptions{})
	if err != nil {
		t.Fatalf("failed to create root user %v", err)
	}
	_, err = cc.UserGrantRole(ctx, "root", "root")
	if err != nil {
		t.Fatalf("failed to grant root user root role %v", err)
	}
	if err = cc.AuthEnable(ctx); err != nil {
		t.Fatalf("failed to enable auth %v", err)
	}
}

func authSetupDefaultTestUser(ctx context.Context, cc interfaces.Client, t *testing.T) {
	authSetupTestUser(ctx, cc, "test-user", "pass", "test-role", "foo", "", t)
}

func authSetupTestUser(ctx context.Context, cc interfaces.Client, userName, password, roleName, key, end string, t *testing.T) {
	_, err := cc.UserAdd(ctx, userName, password, config.UserAddOptions{})
	if err != nil {
		t.Fatalf("failed to create test-user %v", err)
	}
	_, err = cc.RoleAdd(ctx, roleName)
	if err != nil {
		t.Fatalf("failed to create test-role %v", err)
	}
	_, err = cc.UserGrantRole(ctx, userName, roleName)
	if err != nil {
		t.Fatalf("failed to grant role test-role to user test-user %v", err)
	}
	_, err = cc.RoleGrantPermission(ctx, roleName, key, end, clientv3.PermissionType(clientv3.PermReadWrite))
	if err != nil {
		t.Fatalf("failed to grant role test-role readwrite permission to key foo %v", err)
	}
}

func putFailPerm(ctx context.Context, cc interfaces.Client, key, val string, t *testing.T) {
	err := cc.Put(ctx, key, val, config.PutOptions{})
	if err == nil || !strings.Contains(err.Error(), rpctypes.ErrPermissionDenied.Error()) {
		t.Errorf("want error %s but got %v", rpctypes.ErrPermissionDenied.Error(), err)
	}
}

func mustAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return abs
}
