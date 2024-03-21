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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3Auth(t *testing.T) {
	testCtl(t, testCurlV3Auth)
}
func TestCurlV3AuthClientTLSCertAuth(t *testing.T) {
	testCtl(t, testCurlV3Auth, withCfg(*e2e.NewConfigClientTLSCertAuthWithNoCN()))
}

func TestCurlV3AuthUserBasicOperations(t *testing.T) {
	testCtl(t, testCurlV3AuthUserBasicOperations)
}

func TestCurlV3AuthUserGrantRevokeRoles(t *testing.T) {
	testCtl(t, testCurlV3AuthUserGrantRevokeRoles)
}

func TestCurlV3AuthRoleBasicOperations(t *testing.T) {
	testCtl(t, testCurlV3AuthRoleBasicOperations)
}

func TestCurlV3AuthRoleManagePermission(t *testing.T) {
	testCtl(t, testCurlV3AuthRoleManagePermission)
}

func TestCurlV3AuthEnableDisableStatus(t *testing.T) {
	testCtl(t, testCurlV3AuthEnableDisableStatus)
}

func testCurlV3Auth(cx ctlCtx) {
	usernames := []string{"root", "nonroot", "nooption"}
	pwds := []string{"toor", "pass", "pass"}
	options := []*authpb.UserAddOptions{{NoPassword: false}, {NoPassword: false}, nil}

	// create users
	for i := 0; i < len(usernames); i++ {
		user, err := json.Marshal(&pb.AuthUserAddRequest{Name: usernames[i], Password: pwds[i], Options: options[i]})
		testutil.AssertNil(cx.t, err)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/add",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testCurlV3Auth failed to add user %v (%v)", usernames[i], err)
		}
	}

	// create root role
	rolereq, err := json.Marshal(&pb.AuthRoleAddRequest{Name: "root"})
	testutil.AssertNil(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/add",
		Value:    string(rolereq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3Auth failed to create role (%v)", err)
	}

	//grant root role
	for i := 0; i < len(usernames); i++ {
		grantroleroot, merr := json.Marshal(&pb.AuthUserGrantRoleRequest{User: usernames[i], Role: "root"})
		testutil.AssertNil(cx.t, merr)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/grant",
			Value:    string(grantroleroot),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testCurlV3Auth failed to grant role (%v)", err)
		}
	}

	// enable auth
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/enable",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3Auth failed to enable auth (%v)", err)
	}

	for i := 0; i < len(usernames); i++ {
		// put "bar[i]" into "foo[i]"
		putreq, err := json.Marshal(&pb.PutRequest{Key: []byte(fmt.Sprintf("foo%d", i)), Value: []byte(fmt.Sprintf("bar%d", i))})
		testutil.AssertNil(cx.t, err)

		// fail put no auth
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/kv/put",
			Value:    string(putreq),
			Expected: expect.ExpectedResponse{Value: "etcdserver: user name is empty"},
		}); err != nil {
			cx.t.Fatalf("testCurlV3Auth failed to put without token (%v)", err)
		}

		// auth request
		authreq, err := json.Marshal(&pb.AuthenticateRequest{Name: usernames[i], Password: pwds[i]})
		testutil.AssertNil(cx.t, err)

		var (
			authHeader string
			cmdArgs    []string
			lineFunc   = func(txt string) bool { return true }
		)

		cmdArgs = e2e.CURLPrefixArgsCluster(cx.epc.Cfg, cx.epc.Procs[rand.Intn(cx.epc.Cfg.ClusterSize)], "POST", e2e.CURLReq{
			Endpoint: "/v3/auth/authenticate",
			Value:    string(authreq),
		})
		proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
		testutil.AssertNil(cx.t, err)
		defer proc.Close()

		cURLRes, err := proc.ExpectFunc(context.Background(), lineFunc)
		testutil.AssertNil(cx.t, err)

		authRes := make(map[string]any)
		testutil.AssertNil(cx.t, json.Unmarshal([]byte(cURLRes), &authRes))

		token, ok := authRes[rpctypes.TokenFieldNameGRPC].(string)
		if !ok {
			cx.t.Fatalf("failed invalid token in authenticate response using user (%v)", usernames[i])
		}

		authHeader = "Authorization: " + token
		// put with auth
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/kv/put",
			Value:    string(putreq),
			Header:   authHeader,
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testCurlV3Auth failed to auth put with user (%v) (%v)", usernames[i], err)
		}
	}
}

func testCurlV3AuthUserBasicOperations(cx ctlCtx) {
	usernames := []string{"user1", "user2", "user3"}

	// create users
	for i := 0; i < len(usernames); i++ {
		user, err := json.Marshal(&pb.AuthUserAddRequest{Name: usernames[i], Password: "123"})
		require.NoError(cx.t, err)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/add",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testCurlV3AuthUserBasicOperations failed to add user %v (%v)", usernames[i], err)
		}
	}

	// change password
	user, err := json.Marshal(&pb.AuthUserChangePasswordRequest{Name: "user1", Password: "456"})
	require.NoError(cx.t, err)
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/changepw",
		Value:    string(user),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthUserBasicOperations failed to change user's password(%v)", err)
	}

	// get users
	usernames = []string{"user1", "userX"}
	expectedResponse := []string{"revision", "etcdserver: user name not found"}
	for i := 0; i < len(usernames); i++ {
		user, err = json.Marshal(&pb.AuthUserGetRequest{
			Name: usernames[i],
		})

		require.NoError(cx.t, err)
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/get",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}); err != nil {
			cx.t.Fatalf("testCurlV3AuthUserBasicOperations failed to get user %v (%v)", usernames[i], err)
		}
	}

	// delete users
	usernames = []string{"user2", "userX"}
	expectedResponse = []string{"revision", "etcdserver: user name not found"}
	for i := 0; i < len(usernames); i++ {
		user, err = json.Marshal(&pb.AuthUserDeleteRequest{
			Name: usernames[i],
		})
		require.NoError(cx.t, err)
		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/delete",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}); err != nil {
			cx.t.Fatalf("testCurlV3AuthUserBasicOperations failed to delete user %v (%v)", usernames[i], err)
		}
	}

	// list users
	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[rand.Intn(clus.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: "/v3/auth/user/list",
		Value:    "{}",
	})
	resp, err := runCommandAndReadJSONOutput(args)
	require.NoError(cx.t, err)

	users, ok := resp["users"]
	require.True(cx.t, ok)
	userSlice := users.([]any)
	require.Equal(cx.t, 2, len(userSlice))
	require.Equal(cx.t, "user1", userSlice[0])
	require.Equal(cx.t, "user3", userSlice[1])
}

func testCurlV3AuthUserGrantRevokeRoles(cx ctlCtx) {
	var (
		username = "user1"
		rolename = "role1"
	)

	// create user
	user, err := json.Marshal(&pb.AuthUserAddRequest{Name: username, Password: "123"})
	require.NoError(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/add",
		Value:    string(user),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthUserGrantRevokeRoles failed to add user %v (%v)", username, err)
	}

	// create role
	role, err := json.Marshal(&pb.AuthRoleAddRequest{Name: rolename})
	require.NoError(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/add",
		Value:    string(role),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthUserGrantRevokeRoles failed to add role %v (%v)", rolename, err)
	}

	// grant role to user
	grantRoleReq, err := json.Marshal(&pb.AuthUserGrantRoleRequest{
		User: username,
		Role: rolename,
	})
	require.NoError(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/grant",
		Value:    string(grantRoleReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthUserGrantRevokeRoles failed to grant role to user (%v)", err)
	}

	//  revoke role from user
	revokeRoleReq, err := json.Marshal(&pb.AuthUserRevokeRoleRequest{
		Name: username,
		Role: rolename,
	})
	require.NoError(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/revoke",
		Value:    string(revokeRoleReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthUserGrantRevokeRoles failed to revoke role from user (%v)", err)
	}
}

func testCurlV3AuthRoleBasicOperations(cx ctlCtx) {
	rolenames := []string{"role1", "role2", "role3"}

	// create roles
	for i := 0; i < len(rolenames); i++ {
		role, err := json.Marshal(&pb.AuthRoleAddRequest{Name: rolenames[i]})
		require.NoError(cx.t, err)

		if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/role/add",
			Value:    string(role),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}); err != nil {
			cx.t.Fatalf("testCurlV3AuthRoleBasicOperations failed to add role %v (%v)", rolenames[i], err)
		}
	}

	// get roles
	rolenames = []string{"role1", "roleX"}
	expectedResponse := []string{"revision", "etcdserver: role name not found"}
	for i := 0; i < len(rolenames); i++ {
		role, err := json.Marshal(&pb.AuthRoleGetRequest{
			Role: rolenames[i],
		})
		require.NoError(cx.t, err)
		if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/role/get",
			Value:    string(role),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}); err != nil {
			cx.t.Fatalf("testCurlV3AuthRoleBasicOperations failed to get role %v (%v)", rolenames[i], err)
		}
	}

	// delete roles
	rolenames = []string{"role2", "roleX"}
	expectedResponse = []string{"revision", "etcdserver: role name not found"}
	for i := 0; i < len(rolenames); i++ {
		role, err := json.Marshal(&pb.AuthRoleDeleteRequest{
			Role: rolenames[i],
		})
		require.NoError(cx.t, err)
		if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/role/delete",
			Value:    string(role),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}); err != nil {
			cx.t.Fatalf("testCurlV3AuthRoleBasicOperations failed to delete role %v (%v)", rolenames[i], err)
		}
	}

	// list roles
	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[rand.Intn(clus.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: "/v3/auth/role/list",
		Value:    "{}",
	})
	resp, err := runCommandAndReadJSONOutput(args)
	require.NoError(cx.t, err)

	roles, ok := resp["roles"]
	require.True(cx.t, ok)
	roleSlice := roles.([]any)
	require.Equal(cx.t, 2, len(roleSlice))
	require.Equal(cx.t, "role1", roleSlice[0])
	require.Equal(cx.t, "role3", roleSlice[1])
}

func testCurlV3AuthRoleManagePermission(cx ctlCtx) {
	var (
		rolename = "role1"
	)

	// create a role
	role, err := json.Marshal(&pb.AuthRoleAddRequest{Name: rolename})
	require.NoError(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/add",
		Value:    string(role),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthRoleManagePermission failed to add role %v (%v)", rolename, err)
	}

	// grant permission
	grantPermissionReq, err := json.Marshal(&pb.AuthRoleGrantPermissionRequest{
		Name: rolename,
		Perm: &authpb.Permission{
			PermType: authpb.READ,
			Key:      []byte("fakeKey"),
		},
	})
	require.NoError(cx.t, err)

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/grant",
		Value:    string(grantPermissionReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthRoleManagePermission failed to grant permission to role %v (%v)", rolename, err)
	}

	// revoke permission
	revokePermissionReq, err := json.Marshal(&pb.AuthRoleRevokePermissionRequest{
		Role: rolename,
		Key:  []byte("fakeKey"),
	})
	require.NoError(cx.t, err)

	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/revoke",
		Value:    string(revokePermissionReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthRoleManagePermission failed to revoke permission from role %v (%v)", rolename, err)
	}
}

func testCurlV3AuthEnableDisableStatus(cx ctlCtx) {
	// enable auth
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/enable",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "etcdserver: root user does not exist"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthEnableDisableStatus failed to enable auth (%v)", err)
	}

	// disable auth
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/disable",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthEnableDisableStatus failed to disable auth (%v)", err)
	}

	// auth status
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/status",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "revision"},
	}); err != nil {
		cx.t.Fatalf("testCurlV3AuthEnableDisableStatus failed to get auth status (%v)", err)
	}
}
