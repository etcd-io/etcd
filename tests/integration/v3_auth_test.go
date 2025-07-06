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

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/v3"
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

	tresp, terr := api.Lease.LeaseTimeToLive(
		context.TODO(),
		&pb.LeaseTimeToLiveRequest{
			ID:   int64(leaseID),
			Keys: true,
		},
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

		perm := &authpb.Permission{
			PermType: authpb.READWRITE,
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
		_, err = c.RoleGrantPermission(context.TODO(), role, "", clientv3.GetPrefixRangeEnd(""), clientv3.PermissionType(clientv3.PermReadWrite))
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
