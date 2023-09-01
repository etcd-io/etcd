// Copyright 2016 The etcd Authors
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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"testing"

	// "github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestV3CurlAuthEnable(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlAuthEnable, withApiPrefix(p))
	}
}

func testV3CurlAuthEnable(cx ctlCtx) {
	name := "root"
	password := "123"
	hashedPassword := fmt.Sprintf("%x", sha256.Sum256([]byte(password)))
	wreq, err := json.Marshal(&pb.AuthUserAddRequest{Name: name, Password: password, HashedPassword: hashedPassword})
	if err != nil {
		cx.t.Fatal(err)
	}

	err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/auth/user/add"),
		Value:    string(wreq),
		Expected: expect.ExpectedResponse{Value: `"revision":`},
	})
	if err != nil {
		cx.t.Fatal(err)
	}

	wreq, err = json.Marshal(&pb.AuthUserGrantRoleRequest{User: name, Role: "root"})
	if err != nil {
		cx.t.Fatal(err)
	}

	err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/auth/user/grant"),
		Value:    string(wreq),
		Expected: expect.ExpectedResponse{Value: `"revision":`},
	})
	if err != nil {
		cx.t.Fatal(err)
	}

	wreq, err = json.Marshal(&pb.AuthEnableRequest{})
	if err != nil {
		cx.t.Fatal(err)
	}

	err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: path.Join(cx.apiPrefix, "/auth/enable"),
		Value:    string(wreq),
		Expected: expect.ExpectedResponse{Value: `"revision":`},
	})
	if err != nil {
		cx.t.Fatal(err)
	}
}


// AuthEnable(ctx context.Context, r *pb.AuthEnableRequest) (*pb.AuthEnableResponse, error)
// AuthDisable(ctx context.Context, r *pb.AuthDisableRequest) (*pb.AuthDisableResponse, error)
// AuthStatus(ctx context.Context, r *pb.AuthStatusRequest) (*pb.AuthStatusResponse, error)
// Authenticate(ctx context.Context, r *pb.AuthenticateRequest) (*pb.AuthenticateResponse, error)
// UserAdd(ctx context.Context, r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error)
// UserDelete(ctx context.Context, r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error)
// UserChangePassword(ctx context.Context, r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error)
// UserGrantRole(ctx context.Context, r *pb.AuthUserGrantRoleRequest) (*pb.AuthUserGrantRoleResponse, error)
// UserGet(ctx context.Context, r *pb.AuthUserGetRequest) (*pb.AuthUserGetResponse, error)
// UserRevokeRole(ctx context.Context, r *pb.AuthUserRevokeRoleRequest) (*pb.AuthUserRevokeRoleResponse, error)
// RoleAdd(ctx context.Context, r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error)
// RoleGrantPermission(ctx context.Context, r *pb.AuthRoleGrantPermissionRequest) (*pb.AuthRoleGrantPermissionResponse, error)
// RoleGet(ctx context.Context, r *pb.AuthRoleGetRequest) (*pb.AuthRoleGetResponse, error)
// RoleRevokePermission(ctx context.Context, r *pb.AuthRoleRevokePermissionRequest) (*pb.AuthRoleRevokePermissionResponse, error)
// RoleDelete(ctx context.Context, r *pb.AuthRoleDeleteRequest) (*pb.AuthRoleDeleteResponse, error)
// UserList(ctx context.Context, r *pb.AuthUserListRequest) (*pb.AuthUserListResponse, error)
// RoleList(ctx context.Context, r *pb.AuthRoleListRequest) (*pb.AuthRoleListResponse, error)