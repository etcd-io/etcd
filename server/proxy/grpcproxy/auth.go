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

package grpcproxy

import (
	"context"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type AuthProxy struct {
	authClient pb.AuthClient
}

func NewAuthProxy(c *clientv3.Client) pb.AuthServer {
	return &AuthProxy{authClient: pb.NewAuthClient(c.ActiveConnection())}
}

func (ap *AuthProxy) AuthEnable(ctx context.Context, r *pb.AuthEnableRequest) (*pb.AuthEnableResponse, error) {
	return ap.authClient.AuthEnable(ctx, r)
}

func (ap *AuthProxy) AuthDisable(ctx context.Context, r *pb.AuthDisableRequest) (*pb.AuthDisableResponse, error) {
	return ap.authClient.AuthDisable(ctx, r)
}

func (ap *AuthProxy) AuthStatus(ctx context.Context, r *pb.AuthStatusRequest) (*pb.AuthStatusResponse, error) {
	return ap.authClient.AuthStatus(ctx, r)
}

func (ap *AuthProxy) Authenticate(ctx context.Context, r *pb.AuthenticateRequest) (*pb.AuthenticateResponse, error) {
	return ap.authClient.Authenticate(ctx, r)
}

func (ap *AuthProxy) RoleAdd(ctx context.Context, r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error) {
	return ap.authClient.RoleAdd(ctx, r)
}

func (ap *AuthProxy) RoleDelete(ctx context.Context, r *pb.AuthRoleDeleteRequest) (*pb.AuthRoleDeleteResponse, error) {
	return ap.authClient.RoleDelete(ctx, r)
}

func (ap *AuthProxy) RoleGet(ctx context.Context, r *pb.AuthRoleGetRequest) (*pb.AuthRoleGetResponse, error) {
	return ap.authClient.RoleGet(ctx, r)
}

func (ap *AuthProxy) RoleList(ctx context.Context, r *pb.AuthRoleListRequest) (*pb.AuthRoleListResponse, error) {
	return ap.authClient.RoleList(ctx, r)
}

func (ap *AuthProxy) RoleRevokePermission(ctx context.Context, r *pb.AuthRoleRevokePermissionRequest) (*pb.AuthRoleRevokePermissionResponse, error) {
	return ap.authClient.RoleRevokePermission(ctx, r)
}

func (ap *AuthProxy) RoleGrantPermission(ctx context.Context, r *pb.AuthRoleGrantPermissionRequest) (*pb.AuthRoleGrantPermissionResponse, error) {
	return ap.authClient.RoleGrantPermission(ctx, r)
}

func (ap *AuthProxy) UserAdd(ctx context.Context, r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error) {
	return ap.authClient.UserAdd(ctx, r)
}

func (ap *AuthProxy) UserDelete(ctx context.Context, r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error) {
	return ap.authClient.UserDelete(ctx, r)
}

func (ap *AuthProxy) UserGet(ctx context.Context, r *pb.AuthUserGetRequest) (*pb.AuthUserGetResponse, error) {
	return ap.authClient.UserGet(ctx, r)
}

func (ap *AuthProxy) UserList(ctx context.Context, r *pb.AuthUserListRequest) (*pb.AuthUserListResponse, error) {
	return ap.authClient.UserList(ctx, r)
}

func (ap *AuthProxy) UserGrantRole(ctx context.Context, r *pb.AuthUserGrantRoleRequest) (*pb.AuthUserGrantRoleResponse, error) {
	return ap.authClient.UserGrantRole(ctx, r)
}

func (ap *AuthProxy) UserRevokeRole(ctx context.Context, r *pb.AuthUserRevokeRoleRequest) (*pb.AuthUserRevokeRoleResponse, error) {
	return ap.authClient.UserRevokeRole(ctx, r)
}

func (ap *AuthProxy) UserChangePassword(ctx context.Context, r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error) {
	return ap.authClient.UserChangePassword(ctx, r)
}
