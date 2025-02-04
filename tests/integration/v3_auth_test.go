// Copyright 2017 The etcd Authors
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

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestV3AuthEmptyUserGet ensures that a get with an empty user will return an empty user error.
func TestV3AuthEmptyUserGet(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	api := integration.ToGRPC(clus.Client(0))
	authSetupRoot(t, api.Auth)

	_, err := api.KV.Range(ctx, &pb.RangeRequest{Key: []byte("abc")})
	if !eqErrGRPC(err, rpctypes.ErrUserEmpty) {
		t.Fatalf("got %v, expected %v", err, rpctypes.ErrUserEmpty)
	}
}

// TestV3AuthEmptyUserPut ensures that a put with an empty user will return an empty user error,
// and the consistent_index should be moved forward even the apply-->Put fails.
func TestV3AuthEmptyUserPut(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:          1,
		SnapshotCount: 3,
	})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	api := integration.ToGRPC(clus.Client(0))
	authSetupRoot(t, api.Auth)

	// The SnapshotCount is 3, so there must be at least 3 new snapshot files being created.
	// The VERIFY logic will check whether the consistent_index >= last snapshot index on
	// cluster terminating.
	for i := 0; i < 10; i++ {
		_, err := api.KV.Put(ctx, &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if !eqErrGRPC(err, rpctypes.ErrUserEmpty) {
			t.Fatalf("got %v, expected %v", err, rpctypes.ErrUserEmpty)
		}
	}
}

// TestV3AuthTokenWithDisable tests that auth won't crash if
// given a valid token when authentication is disabled
func TestV3AuthTokenWithDisable(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	c, cerr := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "root", Password: "123"})
	require.NoError(t, cerr)
	defer c.Close()

	rctx, cancel := context.WithCancel(context.TODO())
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		for rctx.Err() == nil {
			c.Put(rctx, "abc", "def")
		}
	}()

	time.Sleep(10 * time.Millisecond)
	_, err := c.AuthDisable(context.TODO())
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-donec
}

func TestV3AuthRevision(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	api := integration.ToGRPC(clus.Client(0))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	presp, perr := api.KV.Put(ctx, &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	cancel()
	require.NoError(t, perr)
	rev := presp.Header.Revision

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	aresp, aerr := api.Auth.UserAdd(ctx, &pb.AuthUserAddRequest{Name: "root", Password: "123", Options: &authpb.UserAddOptions{NoPassword: false}})
	cancel()
	require.NoError(t, aerr)
	if aresp.Header.Revision != rev {
		t.Fatalf("revision expected %d, got %d", rev, aresp.Header.Revision)
	}
}

// TestV3AuthWithLeaseRevokeWithRoot ensures that granted leases
// with root user be revoked after TTL.
func TestV3AuthWithLeaseRevokeWithRoot(t *testing.T) {
	testV3AuthWithLeaseRevokeWithRoot(t, integration.ClusterConfig{Size: 1})
}

// TestV3AuthWithLeaseRevokeWithRootJWT creates a lease with a JWT-token enabled cluster.
// And tests if server is able to revoke expiry lease item.
func TestV3AuthWithLeaseRevokeWithRootJWT(t *testing.T) {
	testV3AuthWithLeaseRevokeWithRoot(t, integration.ClusterConfig{Size: 1, AuthToken: integration.DefaultTokenJWT})
}

func testV3AuthWithLeaseRevokeWithRoot(t *testing.T, ccfg integration.ClusterConfig) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &ccfg)
	defer clus.Terminate(t)

	api := integration.ToGRPC(clus.Client(0))
	authSetupRoot(t, api.Auth)

	rootc, cerr := integration.NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, cerr)
	defer rootc.Close()

	leaseResp, err := rootc.Grant(context.TODO(), 2)
	require.NoError(t, err)
	leaseID := leaseResp.ID

	_, err = rootc.Put(context.TODO(), "foo", "bar", clientv3.WithLease(leaseID))
	require.NoError(t, err)

	// wait for lease expire
	time.Sleep(3 * time.Second)

	tresp, terr := rootc.TimeToLive(
		context.TODO(),
		leaseID,
		clientv3.WithAttachedKeys(),
	)
	if terr != nil {
		t.Error(terr)
	}
	if len(tresp.Keys) > 0 || tresp.GrantedTTL != 0 {
		t.Errorf("lease %016x should have been revoked, got %+v", leaseID, tresp)
	}
	if tresp.TTL != -1 {
		t.Errorf("lease %016x should have been expired, got %+v", leaseID, tresp)
	}
}

type user struct {
	name     string
	password string
	role     string
	key      string
	end      string
}

func TestV3AuthWithLeaseRevoke(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	users := []user{
		{
			name:     "user1",
			password: "user1-123",
			role:     "role1",
			key:      "k1",
			end:      "k2",
		},
	}
	authSetupUsers(t, integration.ToGRPC(clus.Client(0)).Auth, users)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	rootc, cerr := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "root", Password: "123"})
	require.NoError(t, cerr)
	defer rootc.Close()

	leaseResp, err := rootc.Grant(context.TODO(), 90)
	require.NoError(t, err)
	leaseID := leaseResp.ID
	// permission of k3 isn't granted to user1
	_, err = rootc.Put(context.TODO(), "k3", "val", clientv3.WithLease(leaseID))
	require.NoError(t, err)

	userc, cerr := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	require.NoError(t, cerr)
	defer userc.Close()
	_, err = userc.Revoke(context.TODO(), leaseID)
	if err == nil {
		t.Fatal("revoking from user1 should be failed with permission denied")
	}
}

func TestV3AuthWithLeaseAttach(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	users := []user{
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
	authSetupUsers(t, integration.ToGRPC(clus.Client(0)).Auth, users)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	user1c, cerr := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	require.NoError(t, cerr)
	defer user1c.Close()

	user2c, cerr := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user2", Password: "user2-123"})
	require.NoError(t, cerr)
	defer user2c.Close()

	leaseResp, err := user1c.Grant(context.TODO(), 90)
	require.NoError(t, err)
	leaseID := leaseResp.ID
	// permission of k2 is also granted to user2
	_, err = user1c.Put(context.TODO(), "k2", "val", clientv3.WithLease(leaseID))
	require.NoError(t, err)

	_, err = user2c.Revoke(context.TODO(), leaseID)
	require.NoError(t, err)

	leaseResp, err = user1c.Grant(context.TODO(), 90)
	require.NoError(t, err)
	leaseID = leaseResp.ID
	// permission of k1 isn't granted to user2
	_, err = user1c.Put(context.TODO(), "k1", "val", clientv3.WithLease(leaseID))
	require.NoError(t, err)

	_, err = user2c.Revoke(context.TODO(), leaseID)
	if err == nil {
		t.Fatal("revoking from user2 should be failed with permission denied")
	}
}

func authSetupUsers(t *testing.T, auth pb.AuthClient, users []user) {
	for _, user := range users {
		_, err := auth.UserAdd(context.TODO(), &pb.AuthUserAddRequest{Name: user.name, Password: user.password, Options: &authpb.UserAddOptions{NoPassword: false}})
		require.NoError(t, err)
		_, err = auth.RoleAdd(context.TODO(), &pb.AuthRoleAddRequest{Name: user.role})
		require.NoError(t, err)
		_, err = auth.UserGrantRole(context.TODO(), &pb.AuthUserGrantRoleRequest{User: user.name, Role: user.role})
		require.NoError(t, err)

		if len(user.key) == 0 {
			continue
		}

		perm := &authpb.Permission{
			PermType: authpb.READWRITE,
			Key:      []byte(user.key),
			RangeEnd: []byte(user.end),
		}
		_, err = auth.RoleGrantPermission(context.TODO(), &pb.AuthRoleGrantPermissionRequest{Name: user.role, Perm: perm})
		require.NoError(t, err)
	}
}

func authSetupRoot(t *testing.T, auth pb.AuthClient) {
	root := []user{
		{
			name:     "root",
			password: "123",
			role:     "root",
			key:      "",
		},
	}
	authSetupUsers(t, auth, root)
	_, err := auth.AuthEnable(context.TODO(), &pb.AuthEnableRequest{})
	require.NoError(t, err)
}

func TestV3AuthNonAuthorizedRPCs(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	nonAuthedKV := clus.Client(0).KV

	key := "foo"
	val := "bar"
	_, err := nonAuthedKV.Put(context.TODO(), key, val)
	if err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	respput, err := nonAuthedKV.Put(context.TODO(), key, val)
	if !eqErrGRPC(err, rpctypes.ErrGRPCUserEmpty) {
		t.Fatalf("could put key (%v), it should cause an error of permission denied", respput)
	}
}

func TestV3AuthOldRevConcurrent(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	c, cerr := integration.NewClient(t, clientv3.Config{
		Endpoints:   clus.Client(0).Endpoints(),
		DialTimeout: 5 * time.Second,
		Username:    "root",
		Password:    "123",
	})
	require.NoError(t, cerr)
	defer c.Close()

	var wg sync.WaitGroup
	f := func(i int) {
		defer wg.Done()
		role, user := fmt.Sprintf("test-role-%d", i), fmt.Sprintf("test-user-%d", i)
		_, err := c.RoleAdd(context.TODO(), role)
		require.NoError(t, err)
		_, err = c.RoleGrantPermission(context.TODO(), role, "\x00", clientv3.GetPrefixRangeEnd(""), clientv3.PermissionType(clientv3.PermReadWrite))
		require.NoError(t, err)
		_, err = c.UserAdd(context.TODO(), user, "123")
		require.NoError(t, err)
		_, err = c.Put(context.TODO(), "a", "b")
		assert.NoError(t, err)
	}
	// needs concurrency to trigger
	numRoles := 2
	wg.Add(numRoles)
	for i := 0; i < numRoles; i++ {
		go f(i)
	}
	wg.Wait()
}

func TestV3AuthWatchErrorAndWatchId0(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	users := []user{
		{
			name:     "user1",
			password: "user1-123",
			role:     "role1",
			key:      "k1",
			end:      "k2",
		},
	}
	authSetupUsers(t, integration.ToGRPC(clus.Client(0)).Auth, users)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	c, cerr := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	require.NoError(t, cerr)
	defer c.Close()

	watchStartCh, watchEndCh := make(chan any), make(chan any)

	go func() {
		wChan := c.Watch(ctx, "k1", clientv3.WithRev(1))
		watchStartCh <- struct{}{}
		watchResponse := <-wChan
		t.Logf("watch response from k1: %v", watchResponse)
		assert.NotEmpty(t, watchResponse.Events)
		watchEndCh <- struct{}{}
	}()

	// Chan for making sure that the above goroutine invokes Watch()
	// So the above Watch() can get watch ID = 0
	<-watchStartCh

	wChan := c.Watch(ctx, "non-allowed-key", clientv3.WithRev(1))
	watchResponse := <-wChan
	require.Error(t, watchResponse.Err()) // permission denied

	_, err := c.Put(ctx, "k1", "val")
	if err != nil {
		t.Fatalf("Unexpected error from Put: %v", err)
	}

	<-watchEndCh
}
