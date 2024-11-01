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
		require.NoError(cx.t, err)

		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/add",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}), "testCurlV3Auth failed to add user %v", usernames[i])
	}

	// create root role
	rolereq, err := json.Marshal(&pb.AuthRoleAddRequest{Name: "root"})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/add",
		Value:    string(rolereq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3Auth failed to create role")

	// grant root role
	for i := 0; i < len(usernames); i++ {
		grantroleroot, merr := json.Marshal(&pb.AuthUserGrantRoleRequest{User: usernames[i], Role: "root"})
		require.NoError(cx.t, merr)

		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/grant",
			Value:    string(grantroleroot),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}), "testCurlV3Auth failed to grant role")
	}

	// enable auth
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/enable",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3Auth failed to enable auth")

	for i := 0; i < len(usernames); i++ {
		// put "bar[i]" into "foo[i]"
		putreq, err := json.Marshal(&pb.PutRequest{Key: []byte(fmt.Sprintf("foo%d", i)), Value: []byte(fmt.Sprintf("bar%d", i))})
		require.NoError(cx.t, err)

		// fail put no auth
		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/kv/put",
			Value:    string(putreq),
			Expected: expect.ExpectedResponse{Value: "etcdserver: user name is empty"},
		}), "testCurlV3Auth failed to put without token")

		// auth request
		authreq, err := json.Marshal(&pb.AuthenticateRequest{Name: usernames[i], Password: pwds[i]})
		require.NoError(cx.t, err)

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
		require.NoError(cx.t, err)
		defer proc.Close()

		cURLRes, err := proc.ExpectFunc(context.Background(), lineFunc)
		require.NoError(cx.t, err)

		authRes := make(map[string]any)
		require.NoError(cx.t, json.Unmarshal([]byte(cURLRes), &authRes))

		token, ok := authRes[rpctypes.TokenFieldNameGRPC].(string)
		if !ok {
			cx.t.Fatalf("failed invalid token in authenticate response using user (%v)", usernames[i])
		}

		authHeader = "Authorization: " + token
		// put with auth
		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/kv/put",
			Value:    string(putreq),
			Header:   authHeader,
			Expected: expect.ExpectedResponse{Value: "revision"},
		}), "testCurlV3Auth failed to auth put with user (%v)", usernames[i])
	}
}

func testCurlV3AuthUserBasicOperations(cx ctlCtx) {
	usernames := []string{"user1", "user2", "user3"}

	// create users
	for i := 0; i < len(usernames); i++ {
		user, err := json.Marshal(&pb.AuthUserAddRequest{Name: usernames[i], Password: "123"})
		require.NoError(cx.t, err)

		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/add",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}), "testCurlV3AuthUserBasicOperations failed to add user %v", usernames[i])
	}

	// change password
	user, err := json.Marshal(&pb.AuthUserChangePasswordRequest{Name: "user1", Password: "456"})
	require.NoError(cx.t, err)
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/changepw",
		Value:    string(user),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthUserBasicOperations failed to change user's password")

	// get users
	usernames = []string{"user1", "userX"}
	expectedResponse := []string{"revision", "etcdserver: user name not found"}
	for i := 0; i < len(usernames); i++ {
		user, err = json.Marshal(&pb.AuthUserGetRequest{
			Name: usernames[i],
		})

		require.NoError(cx.t, err)
		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/get",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}), "testCurlV3AuthUserBasicOperations failed to get user %v", usernames[i])
	}

	// delete users
	usernames = []string{"user2", "userX"}
	expectedResponse = []string{"revision", "etcdserver: user name not found"}
	for i := 0; i < len(usernames); i++ {
		user, err = json.Marshal(&pb.AuthUserDeleteRequest{
			Name: usernames[i],
		})
		require.NoError(cx.t, err)
		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/user/delete",
			Value:    string(user),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}), "testCurlV3AuthUserBasicOperations failed to delete user %v", usernames[i])
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
	require.Len(cx.t, userSlice, 2)
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

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/add",
		Value:    string(user),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthUserGrantRevokeRoles failed to add user %v", username)

	// create role
	role, err := json.Marshal(&pb.AuthRoleAddRequest{Name: rolename})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/add",
		Value:    string(role),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthUserGrantRevokeRoles failed to add role %v", rolename)

	// grant role to user
	grantRoleReq, err := json.Marshal(&pb.AuthUserGrantRoleRequest{
		User: username,
		Role: rolename,
	})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/grant",
		Value:    string(grantRoleReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthUserGrantRevokeRoles failed to grant role to user")

	//  revoke role from user
	revokeRoleReq, err := json.Marshal(&pb.AuthUserRevokeRoleRequest{
		Name: username,
		Role: rolename,
	})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/user/revoke",
		Value:    string(revokeRoleReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthUserGrantRevokeRoles failed to revoke role from user")
}

func testCurlV3AuthRoleBasicOperations(cx ctlCtx) {
	rolenames := []string{"role1", "role2", "role3"}

	// create roles
	for i := 0; i < len(rolenames); i++ {
		role, err := json.Marshal(&pb.AuthRoleAddRequest{Name: rolenames[i]})
		require.NoError(cx.t, err)

		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/role/add",
			Value:    string(role),
			Expected: expect.ExpectedResponse{Value: "revision"},
		}), "testCurlV3AuthRoleBasicOperations failed to add role %v", rolenames[i])
	}

	// get roles
	rolenames = []string{"role1", "roleX"}
	expectedResponse := []string{"revision", "etcdserver: role name not found"}
	for i := 0; i < len(rolenames); i++ {
		role, err := json.Marshal(&pb.AuthRoleGetRequest{
			Role: rolenames[i],
		})
		require.NoError(cx.t, err)
		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/role/get",
			Value:    string(role),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}), "testCurlV3AuthRoleBasicOperations failed to get role %v", rolenames[i])
	}

	// delete roles
	rolenames = []string{"role2", "roleX"}
	expectedResponse = []string{"revision", "etcdserver: role name not found"}
	for i := 0; i < len(rolenames); i++ {
		role, err := json.Marshal(&pb.AuthRoleDeleteRequest{
			Role: rolenames[i],
		})
		require.NoError(cx.t, err)
		require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
			Endpoint: "/v3/auth/role/delete",
			Value:    string(role),
			Expected: expect.ExpectedResponse{Value: expectedResponse[i]},
		}), "testCurlV3AuthRoleBasicOperations failed to delete role %v", rolenames[i])
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
	require.Len(cx.t, roleSlice, 2)
	require.Equal(cx.t, "role1", roleSlice[0])
	require.Equal(cx.t, "role3", roleSlice[1])
}

func testCurlV3AuthRoleManagePermission(cx ctlCtx) {
	rolename := "role1"

	// create a role
	role, err := json.Marshal(&pb.AuthRoleAddRequest{Name: rolename})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/add",
		Value:    string(role),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthRoleManagePermission failed to add role %v", rolename)

	// grant permission
	grantPermissionReq, err := json.Marshal(&pb.AuthRoleGrantPermissionRequest{
		Name: rolename,
		Perm: &authpb.Permission{
			PermType: authpb.READ,
			Key:      []byte("fakeKey"),
		},
	})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/grant",
		Value:    string(grantPermissionReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthRoleManagePermission failed to grant permission to role %v", rolename)

	// revoke permission
	revokePermissionReq, err := json.Marshal(&pb.AuthRoleRevokePermissionRequest{
		Role: rolename,
		Key:  []byte("fakeKey"),
	})
	require.NoError(cx.t, err)

	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/role/revoke",
		Value:    string(revokePermissionReq),
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthRoleManagePermission failed to revoke permission from role %v", rolename)
}

func testCurlV3AuthEnableDisableStatus(cx ctlCtx) {
	// enable auth
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/enable",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "etcdserver: root user does not exist"},
	}), "testCurlV3AuthEnableDisableStatus failed to enable auth")

	// disable auth
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/disable",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthEnableDisableStatus failed to disable auth")

	// auth status
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/auth/status",
		Value:    "{}",
		Expected: expect.ExpectedResponse{Value: "revision"},
	}), "testCurlV3AuthEnableDisableStatus failed to get auth status")
}
