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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	PermissionDenied = "etcdserver: permission denied"
)

// TestV3AuthEmptyUserGet ensures that a get with an empty user will return an empty user error.
func TestV3AuthEmptyUserGet(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	api := toGRPC(clus.Client(0))
	authSetupRoot(t, api.Auth)

	_, err := api.KV.Range(ctx, &pb.RangeRequest{Key: []byte("abc")})
	if !eqErrGRPC(err, rpctypes.ErrUserEmpty) {
		t.Fatalf("got %v, expected %v", err, rpctypes.ErrUserEmpty)
	}
}

// TestV3AuthEmptyUserPut ensures that a put with an empty user will return an empty user error,
// and the consistent_index should be moved forward even the apply-->Put fails.
func TestV3AuthEmptyUserPut(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{
		Size:          1,
		SnapshotCount: 3,
	})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	api := toGRPC(clus.Client(0))
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
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	c, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "root", Password: "123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
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
	if _, err := c.AuthDisable(context.TODO()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-donec
}

func TestV3AuthRevision(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	api := toGRPC(clus.Client(0))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	presp, perr := api.KV.Put(ctx, &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	cancel()
	if perr != nil {
		t.Fatal(perr)
	}
	rev := presp.Header.Revision

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	aresp, aerr := api.Auth.UserAdd(ctx, &pb.AuthUserAddRequest{Name: "root", Password: "123", Options: &authpb.UserAddOptions{NoPassword: false}})
	cancel()
	if aerr != nil {
		t.Fatal(aerr)
	}
	if aresp.Header.Revision != rev {
		t.Fatalf("revision expected %d, got %d", rev, aresp.Header.Revision)
	}
}

// TestV3AuthWithLeaseRevokeWithRoot ensures that granted leases
// with root user be revoked after TTL.
func TestV3AuthWithLeaseRevokeWithRoot(t *testing.T) {
	testV3AuthWithLeaseRevokeWithRoot(t, ClusterConfig{Size: 1})
}

// TestV3AuthWithLeaseRevokeWithRootJWT creates a lease with a JWT-token enabled cluster.
// And tests if server is able to revoke expiry lease item.
func TestV3AuthWithLeaseRevokeWithRootJWT(t *testing.T) {
	testV3AuthWithLeaseRevokeWithRoot(t, ClusterConfig{Size: 1, AuthToken: defaultTokenJWT})
}

func testV3AuthWithLeaseRevokeWithRoot(t *testing.T, ccfg ClusterConfig) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ccfg)
	defer clus.Terminate(t)

	api := toGRPC(clus.Client(0))

	users := []user{
		{
			name:     "test-user",
			password: "test-user-123",
			role:     "test-role",
			key:      "foo",
		},
	}
	authSetupUsers(t, api.Auth, users)
	authSetupRoot(t, api.Auth)

	rootc, cerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer rootc.Close()

	testUserCli, terr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "test-user",
		Password:  "test-user-123",
	})
	require.NoError(t, terr)
	defer testUserCli.Close()

	anonCli, aerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
	})
	require.NoError(t, aerr)
	defer anonCli.Close()

	_, aerr = anonCli.Grant(context.TODO(), 2)
	require.ErrorContains(t, aerr, "etcdserver: user name is empty")

	_, terr = testUserCli.Grant(context.TODO(), 2)
	require.NoError(t, terr)

	leaseResp, err := rootc.Grant(context.TODO(), 2)
	if err != nil {
		t.Fatal(err)
	}
	leaseID := leaseResp.ID

	if _, err = rootc.Put(context.TODO(), "foo", "bar", clientv3.WithLease(leaseID)); err != nil {
		t.Fatal(err)
	}

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
	perm     string
	key      string
	end      string
}

func TestV3AuthWithLeaseRevoke(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
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
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)

	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	rootc, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "root", Password: "123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer rootc.Close()

	leaseResp, err := rootc.Grant(context.TODO(), 90)
	if err != nil {
		t.Fatal(err)
	}
	leaseID := leaseResp.ID
	// permission of k3 isn't granted to user1
	_, err = rootc.Put(context.TODO(), "k3", "val", clientv3.WithLease(leaseID))
	if err != nil {
		t.Fatal(err)
	}

	userc, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer userc.Close()
	_, err = userc.Revoke(context.TODO(), leaseID)
	if err == nil {
		t.Fatal("revoking from user1 should be failed with permission denied")
	}
}

func TestV3AuthWithLeaseAttach(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
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
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)

	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	user1c, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer user1c.Close()

	user2c, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user2", Password: "user2-123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer user2c.Close()

	leaseResp, err := user1c.Grant(context.TODO(), 90)
	if err != nil {
		t.Fatal(err)
	}
	leaseID := leaseResp.ID
	// permission of k2 is also granted to user2
	_, err = user1c.Put(context.TODO(), "k2", "val", clientv3.WithLease(leaseID))
	if err != nil {
		t.Fatal(err)
	}

	_, err = user2c.Revoke(context.TODO(), leaseID)
	if err != nil {
		t.Fatal(err)
	}

	leaseResp, err = user1c.Grant(context.TODO(), 90)
	if err != nil {
		t.Fatal(err)
	}
	leaseID = leaseResp.ID
	// permission of k1 isn't granted to user2
	_, err = user1c.Put(context.TODO(), "k1", "val", clientv3.WithLease(leaseID))
	if err != nil {
		t.Fatal(err)
	}

	_, err = user2c.Revoke(context.TODO(), leaseID)
	if err == nil {
		t.Fatal("revoking from user2 should be failed with permission denied")
	}
}

func authSetupUsers(t *testing.T, auth pb.AuthClient, users []user) {
	for _, user := range users {
		if _, err := auth.UserAdd(context.TODO(), &pb.AuthUserAddRequest{Name: user.name, Password: user.password, Options: &authpb.UserAddOptions{NoPassword: false}}); err != nil {
			t.Fatal(err)
		}
		if _, err := auth.RoleAdd(context.TODO(), &pb.AuthRoleAddRequest{Name: user.role}); err != nil {
			t.Fatal(err)
		}
		if _, err := auth.UserGrantRole(context.TODO(), &pb.AuthUserGrantRoleRequest{User: user.name, Role: user.role}); err != nil {
			t.Fatal(err)
		}

		if len(user.key) == 0 {
			continue
		}

		permType := authpb.READWRITE
		if len(user.perm) > 0 {
			val, ok := authpb.Permission_Type_value[strings.ToUpper(user.perm)]
			if ok {
				permType = authpb.Permission_Type(val)
			}
		}
		perm := &authpb.Permission{
			PermType: permType,
			Key:      []byte(user.key),
			RangeEnd: []byte(user.end),
		}
		if _, err := auth.RoleGrantPermission(context.TODO(), &pb.AuthRoleGrantPermissionRequest{Name: user.role, Perm: perm}); err != nil {
			t.Fatal(err)
		}
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
	if _, err := auth.AuthEnable(context.TODO(), &pb.AuthEnableRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestV3AuthNonAuthorizedRPCs(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	nonAuthedKV := clus.Client(0).KV

	key := "foo"
	val := "bar"
	_, err := nonAuthedKV.Put(context.TODO(), key, val)
	if err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	respput, err := nonAuthedKV.Put(context.TODO(), key, val)
	if !eqErrGRPC(err, rpctypes.ErrGRPCUserEmpty) {
		t.Fatalf("could put key (%v), it should cause an error of permission denied", respput)
	}
}

func TestV3AuthOldRevConcurrent(t *testing.T) {
	t.Skip() // TODO(jingyih): re-enable the test when #10408 is fixed.
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	c, cerr := NewClient(t, clientv3.Config{
		Endpoints:   clus.Client(0).Endpoints(),
		DialTimeout: 5 * time.Second,
		Username:    "root",
		Password:    "123",
	})
	testutil.AssertNil(t, cerr)
	defer c.Close()

	var wg sync.WaitGroup
	f := func(i int) {
		defer wg.Done()
		role, user := fmt.Sprintf("test-role-%d", i), fmt.Sprintf("test-user-%d", i)
		_, err := c.RoleAdd(context.TODO(), role)
		testutil.AssertNil(t, err)
		_, err = c.RoleGrantPermission(context.TODO(), role, "\x00", clientv3.GetPrefixRangeEnd(""), clientv3.PermissionType(clientv3.PermReadWrite))
		testutil.AssertNil(t, err)
		_, err = c.UserAdd(context.TODO(), user, "123")
		testutil.AssertNil(t, err)
		_, err = c.Put(context.TODO(), "a", "b")
		testutil.AssertNil(t, err)
	}
	// needs concurrency to trigger
	numRoles := 2
	wg.Add(numRoles)
	for i := 0; i < numRoles; i++ {
		go f(i)
	}
	wg.Wait()
}

func TestV3AuthRestartMember(t *testing.T) {
	BeforeTest(t)

	// create a cluster with 1 member
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	// create a client
	c, cerr := NewClient(t, clientv3.Config{
		Endpoints:   clus.Client(0).Endpoints(),
		DialTimeout: 5 * time.Second,
	})
	testutil.AssertNil(t, cerr)
	defer c.Close()

	authData := []struct {
		user string
		role string
		pass string
	}{
		{
			user: "root",
			role: "root",
			pass: "123",
		},
		{
			user: "user0",
			role: "role0",
			pass: "123",
		},
	}

	for _, authObj := range authData {
		// add a role
		_, err := c.RoleAdd(context.TODO(), authObj.role)
		testutil.AssertNil(t, err)
		// add a user
		_, err = c.UserAdd(context.TODO(), authObj.user, authObj.pass)
		testutil.AssertNil(t, err)
		// grant role to user
		_, err = c.UserGrantRole(context.TODO(), authObj.user, authObj.role)
		testutil.AssertNil(t, err)
	}

	// role grant permission to role0
	_, err := c.RoleGrantPermission(context.TODO(), authData[1].role, "foo", "", clientv3.PermissionType(clientv3.PermReadWrite))
	testutil.AssertNil(t, err)

	// enable auth
	_, err = c.AuthEnable(context.TODO())
	testutil.AssertNil(t, err)

	// create another client with ID:Password
	c2, cerr := NewClient(t, clientv3.Config{
		Endpoints:   clus.Client(0).Endpoints(),
		DialTimeout: 5 * time.Second,
		Username:    authData[1].user,
		Password:    authData[1].pass,
	})
	testutil.AssertNil(t, cerr)
	defer c2.Close()

	// create foo since that is within the permission set
	// expectation is to succeed
	_, err = c2.Put(context.TODO(), "foo", "bar")
	testutil.AssertNil(t, err)

	clus.Members[0].Stop(t)
	err = clus.Members[0].Restart(t)
	testutil.AssertNil(t, err)
	clus.Members[0].WaitOK(t)

	// nothing has changed, but it fails without refreshing cache after restart
	_, err = c2.Put(context.TODO(), "foo", "bar2")
	testutil.AssertNil(t, err)
}

func TestV3AuthWatchErrorAndWatchId0(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
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

	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)

	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	c, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer c.Close()

	watchStartCh, watchEndCh := make(chan interface{}), make(chan interface{})

	go func() {
		wChan := c.Watch(ctx, "k1", clientv3.WithRev(1))
		watchStartCh <- struct{}{}
		watchResponse := <-wChan
		t.Logf("watch response from k1: %v", watchResponse)
		testutil.AssertTrue(t, len(watchResponse.Events) != 0)
		watchEndCh <- struct{}{}
	}()

	// Chan for making sure that the above goroutine invokes Watch()
	// So the above Watch() can get watch ID = 0
	<-watchStartCh

	wChan := c.Watch(ctx, "non-allowed-key", clientv3.WithRev(1))
	watchResponse := <-wChan
	testutil.AssertNotNil(t, watchResponse.Err()) // permission denied

	_, err := c.Put(ctx, "k1", "val")
	if err != nil {
		t.Fatalf("Unexpected error from Put: %v", err)
	}

	<-watchEndCh
}

func TestV3AuthWithLeaseTimeToLive(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
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
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)

	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	user1c, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer user1c.Close()

	user2c, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user2", Password: "user2-123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer user2c.Close()

	anonCli, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints()})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer anonCli.Close()

	leaseResp, err := user1c.Grant(context.TODO(), 90)
	if err != nil {
		t.Fatal(err)
	}
	leaseID := leaseResp.ID
	_, err = user1c.Put(context.TODO(), "k1", "val", clientv3.WithLease(leaseID))
	if err != nil {
		t.Fatal(err)
	}
	// k2 can be accessed from both user1 and user2
	_, err = user1c.Put(context.TODO(), "k2", "val", clientv3.WithLease(leaseID))
	if err != nil {
		t.Fatal(err)
	}

	_, err = user1c.TimeToLive(context.TODO(), leaseID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = user2c.TimeToLive(context.TODO(), leaseID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = anonCli.TimeToLive(context.TODO(), leaseID)
	require.ErrorContains(t, err, "etcdserver: user name is empty")

	_, err = anonCli.TimeToLive(context.TODO(), leaseID, clientv3.WithAttachedKeys())
	require.ErrorContains(t, err, "etcdserver: user name is empty")

	_, err = user2c.TimeToLive(context.TODO(), leaseID, clientv3.WithAttachedKeys())
	if err == nil {
		t.Fatal("timetolive from user2 should be failed with permission denied")
	}

	rootc, cerr := NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "root", Password: "123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer rootc.Close()

	if _, err := rootc.RoleRevokePermission(context.TODO(), "role1", "k1", "k3"); err != nil {
		t.Fatal(err)
	}

	_, err = user1c.TimeToLive(context.TODO(), leaseID, clientv3.WithAttachedKeys())
	if err == nil {
		t.Fatal("timetolive from user2 should be failed with permission denied")
	}
}

func TestV3AuthWithLeaseRenew(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	users := []user{
		{
			name:     "test-user",
			password: "test-user-123",
			role:     "test-role",
			// test-user can only write keys in [k1, k3), i.e. k1 and k2.
			key: "k1",
			end: "k3",
		},
	}
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)
	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	rootCli, cerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, cerr)
	defer rootCli.Close()

	testUserClis := []*clientv3.Client{}
	for i := 0; i < len(clus.Members); i++ {
		testUserCli, err := NewClient(t, clientv3.Config{
			Endpoints: clus.Client(i).Endpoints(),
			Username:  "test-user",
			Password:  "test-user-123",
		})
		require.NoError(t, err)
		defer testUserCli.Close()

		testUserClis = append(testUserClis, testUserCli)
	}

	anonCli, cerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
	})
	require.NoError(t, cerr)
	defer anonCli.Close()

	leaseResp, err := rootCli.Grant(t.Context(), 90)
	require.NoError(t, err)
	leaseID := leaseResp.ID

	_, err = rootCli.Put(t.Context(), "k1", "val", clientv3.WithLease(leaseID))
	require.NoError(t, err)
	_, err = rootCli.Put(t.Context(), "k3", "val", clientv3.WithLease(leaseID))
	require.NoError(t, err)

	_, err = anonCli.KeepAliveOnce(t.Context(), leaseID)
	require.ErrorContainsf(t, err, "etcdserver: user name is empty", "should reject renew")

	_, err = rootCli.KeepAliveOnce(t.Context(), leaseID)
	require.NoError(t, err)

	for _, testUserCli := range testUserClis {
		_, err = testUserCli.KeepAliveOnce(t.Context(), leaseID)
		require.ErrorContainsf(t, err, "etcdserver: permission denied", "[%v] should reject renew", testUserCli.Endpoints())
	}

	leaseResp, err = rootCli.Grant(t.Context(), 90)
	require.NoError(t, err)
	leaseID = leaseResp.ID

	_, err = rootCli.Put(t.Context(), "k1", "val", clientv3.WithLease(leaseID))
	require.NoError(t, err)
	_, err = rootCli.Put(t.Context(), "k2", "val", clientv3.WithLease(leaseID))
	require.NoError(t, err)

	for _, testUserCli := range testUserClis {
		_, err = testUserCli.KeepAliveOnce(t.Context(), leaseID)
		require.NoErrorf(t, err, "[%v] should accept renew", testUserCli.Endpoints())
	}
}

func TestV3AuthAlarm(t *testing.T) {
	BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	clus := NewClusterV3(t, &ClusterConfig{
		Size:              1,
		QuotaBackendBytes: 1024 * 5,
	})
	defer clus.Terminate(t)

	users := []user{
		{
			name:     "test-user",
			password: "test-user-123",
			role:     "test-role",
			key:      "k1",
			end:      "k3",
		},
	}
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)
	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	rootCli, rerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, rerr)
	defer rootCli.Close()

	testUserCli, terr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "test-user",
		Password:  "test-user-123",
	})
	require.NoError(t, terr)
	defer testUserCli.Close()

	anonCli, aerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
	})
	require.NoError(t, aerr)
	defer anonCli.Close()

	for i := 0; ; i++ {
		_, err := rootCli.Put(ctx, fmt.Sprintf("%v", int64(i)), strings.Repeat("A", 1024))
		if err == nil {
			continue
		}

		require.ErrorContains(t, err, "etcdserver: mvcc: database space exceeded")
		break
	}

	_, err := anonCli.AlarmList(ctx)
	require.ErrorContains(t, err, "etcdserver: user name is empty")

	memberID := uint64(0)

	for i := 0; i < 10; i++ {
		resp, rerr := rootCli.AlarmList(ctx)
		require.NoError(t, rerr)

		if len(resp.Alarms) > 0 {
			memberID = resp.Header.MemberId
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.NotEqualf(t, uint64(0), memberID, "expect to find alarm with non-zero member ID")

	resp, err := testUserCli.AlarmList(ctx)
	require.NoError(t, err)
	require.Len(t, resp.Alarms, 1)

	_, err = testUserCli.AlarmDisarm(ctx, &clientv3.AlarmMember{
		MemberID: memberID,
		Alarm:    pb.AlarmType_NOSPACE,
	})
	require.ErrorContains(t, err, PermissionDenied)

	resp, err = rootCli.AlarmDisarm(ctx, &clientv3.AlarmMember{
		MemberID: memberID,
		Alarm:    pb.AlarmType_NOSPACE,
	})
	require.NoError(t, err)
	require.Lenf(t, resp.Alarms, 1, "expect 1 alarm from disarm but got %v", resp.Alarms)

	resp, err = rootCli.AlarmList(ctx)
	require.NoError(t, err)
	require.Emptyf(t, resp.Alarms, "expect no alarm after disarm but got %v", resp.Alarms)
}

func TestV3AuthMemberListAndStatus(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	users := []user{
		{
			name:     "test-user",
			password: "test-user-123",
			role:     "test-role",
			key:      "k1",
			end:      "k3",
		},
	}
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)
	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	rootCli, rerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, rerr)
	defer rootCli.Close()

	testUserCli, terr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "test-user",
		Password:  "test-user-123",
	})
	require.NoError(t, terr)
	defer testUserCli.Close()

	anonCli, aerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
	})
	require.NoError(t, aerr)
	defer anonCli.Close()

	_, err := anonCli.MemberList(ctx)
	require.ErrorContains(t, err, "etcdserver: user name is empty")

	_, err = testUserCli.MemberList(ctx)
	require.NoError(t, err)

	_, err = testUserCli.Status(ctx, clus.Client(0).Endpoints()[0])
	require.ErrorContains(t, err, PermissionDenied)

	_, err = rootCli.MemberList(ctx)
	require.NoError(t, err)

	_, err = rootCli.Status(ctx, clus.Client(0).Endpoints()[0])
	require.NoError(t, err)
}

func TestV3AuthLeaseLeases(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	users := []user{
		{
			name:     "test-user",
			password: "test-user-123",
			role:     "test-role",
			key:      "foo",
		},
	}
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)
	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	rootCli, rerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, rerr)
	defer rootCli.Close()

	testUserCli, terr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "test-user",
		Password:  "test-user-123",
	})
	require.NoError(t, terr)
	defer testUserCli.Close()

	anonCli, aerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
	})
	require.NoError(t, aerr)
	defer anonCli.Close()

	lresp, err := rootCli.Grant(ctx, 90)
	require.NoError(t, err)
	firstLeaseID := lresp.ID

	_, err = rootCli.Put(ctx, "foo", "value", clientv3.WithLease(firstLeaseID))
	require.NoError(t, err)

	lresp, err = rootCli.Grant(ctx, 90)
	require.NoError(t, err)
	secondLeaseID := lresp.ID

	_, err = rootCli.Put(ctx, "foo1", "value", clientv3.WithLease(secondLeaseID))
	require.NoError(t, err)

	_, err = testUserCli.Leases(ctx)
	require.ErrorContains(t, err, PermissionDenied)

	_, err = anonCli.Leases(ctx)
	require.ErrorContains(t, err, "etcdserver: user name is empty")

	resp, err := rootCli.Leases(ctx)
	require.NoError(t, err)
	require.Lenf(t, resp.Leases, 2, "want 2 leases but got %v", resp.Leases)

	leaseIDs := []clientv3.LeaseID{firstLeaseID, secondLeaseID}
	for _, lease := range resp.Leases {
		require.Containsf(t, leaseIDs, lease.ID, "unexpected lease ID %v, want one of %v", lease.ID, leaseIDs)
	}

	_, err = rootCli.Revoke(ctx, secondLeaseID)
	require.NoError(t, err)

	_, err = testUserCli.Leases(ctx)
	require.NoError(t, err)
}

func TestV3AuthCompact(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	users := []user{
		{
			name:     "test-user",
			password: "test-user-123",
			role:     "test-role",
			key:      "foo",
		},
	}
	authSetupUsers(t, toGRPC(clus.Client(0)).Auth, users)
	authSetupRoot(t, toGRPC(clus.Client(0)).Auth)

	rootCli, rerr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, rerr)
	defer rootCli.Close()

	testUserCli, terr := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "test-user",
		Password:  "test-user-123",
	})
	require.NoError(t, terr)
	defer testUserCli.Close()

	_, err := rootCli.Put(ctx, "key", "value")
	require.NoError(t, err)

	_, err = rootCli.Put(ctx, "key", "value")
	require.NoError(t, err)

	_, err = testUserCli.Compact(ctx, 1, clientv3.WithCompactPhysical())
	require.ErrorContains(t, err, PermissionDenied)

	_, err = rootCli.Compact(ctx, 1, clientv3.WithCompactPhysical())
	require.NoError(t, err)
}

func TestReadWithPrevKvInTXN(t *testing.T) {
	BeforeTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	users := []user{
		{
			name:     "user1",
			password: "user1-123",
			role:     "role1",
			perm:     "write",
			key:      "foo",
			end:      "zoo",
		},
	}
	anonCli := toGRPC(clus.Client(0))
	authSetupUsers(t, anonCli.Auth, users)
	authSetupRoot(t, anonCli.Auth)

	rootc, err := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, err)
	defer rootc.Close()

	userc, err := NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "user1",
		Password:  "user1-123",
	})
	require.NoError(t, err)
	defer userc.Close()

	_, err = rootc.Put(t.Context(), "foo", "bar")
	require.NoError(t, err)

	_, err = userc.Txn(t.Context()).
		Then(clientv3.OpPut("foo", "new", clientv3.WithPrevKV())).
		Commit()

	require.Error(t, err)
	require.Truef(t, eqErrGRPC(err, rpctypes.ErrGRPCPermissionDenied), "got %v, expected %v", err, rpctypes.ErrGRPCPermissionDenied)
}
