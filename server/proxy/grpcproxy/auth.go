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
	"go.etcd.io/etcd/server/v3/proxy/grpcproxy/cache"
)

type AuthProxy struct {
	authClient pb.AuthClient
	cache      cache.Cache
	// we want compile errors if new methods are added
	pb.UnsafeAuthServer
}

// NewAuthProxy returns an AuthServer that forwards auth RPCs to the backend.
// kvp may be nil; when non-nil, any auth-mutating RPC flushes the KV proxy
// cache so no caller can keep reading data cached under stale permissions.
func NewAuthProxy(c *clientv3.Client, kvp *kvProxy) pb.AuthServer {
	ap := &AuthProxy{authClient: pb.NewAuthClient(c.ActiveConnection())}
	if kvp != nil {
		ap.cache = kvp.cache
	}
	return ap
}

func (ap *AuthProxy) AuthEnable(ctx context.Context, r *pb.AuthEnableRequest) (*pb.AuthEnableResponse, error) {
	resp, err := ap.authClient.AuthEnable(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) AuthDisable(ctx context.Context, r *pb.AuthDisableRequest) (*pb.AuthDisableResponse, error) {
	resp, err := ap.authClient.AuthDisable(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) flushCacheOnMutation(err error) {
	if err == nil && ap.cache != nil {
		ap.cache.Clear()
	}
}

func (ap *AuthProxy) AuthStatus(ctx context.Context, r *pb.AuthStatusRequest) (*pb.AuthStatusResponse, error) {
	return ap.authClient.AuthStatus(ctx, r)
}

func (ap *AuthProxy) Authenticate(ctx context.Context, r *pb.AuthenticateRequest) (*pb.AuthenticateResponse, error) {
	return ap.authClient.Authenticate(ctx, r)
}

func (ap *AuthProxy) RoleAdd(ctx context.Context, r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error) {
	resp, err := ap.authClient.RoleAdd(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) RoleDelete(ctx context.Context, r *pb.AuthRoleDeleteRequest) (*pb.AuthRoleDeleteResponse, error) {
	resp, err := ap.authClient.RoleDelete(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) RoleGet(ctx context.Context, r *pb.AuthRoleGetRequest) (*pb.AuthRoleGetResponse, error) {
	return ap.authClient.RoleGet(ctx, r)
}

func (ap *AuthProxy) RoleList(ctx context.Context, r *pb.AuthRoleListRequest) (*pb.AuthRoleListResponse, error) {
	return ap.authClient.RoleList(ctx, r)
}

func (ap *AuthProxy) RoleRevokePermission(ctx context.Context, r *pb.AuthRoleRevokePermissionRequest) (*pb.AuthRoleRevokePermissionResponse, error) {
	resp, err := ap.authClient.RoleRevokePermission(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) RoleGrantPermission(ctx context.Context, r *pb.AuthRoleGrantPermissionRequest) (*pb.AuthRoleGrantPermissionResponse, error) {
	resp, err := ap.authClient.RoleGrantPermission(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) UserAdd(ctx context.Context, r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error) {
	resp, err := ap.authClient.UserAdd(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) UserDelete(ctx context.Context, r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error) {
	resp, err := ap.authClient.UserDelete(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) UserGet(ctx context.Context, r *pb.AuthUserGetRequest) (*pb.AuthUserGetResponse, error) {
	return ap.authClient.UserGet(ctx, r)
}

func (ap *AuthProxy) UserList(ctx context.Context, r *pb.AuthUserListRequest) (*pb.AuthUserListResponse, error) {
	return ap.authClient.UserList(ctx, r)
}

func (ap *AuthProxy) UserGrantRole(ctx context.Context, r *pb.AuthUserGrantRoleRequest) (*pb.AuthUserGrantRoleResponse, error) {
	resp, err := ap.authClient.UserGrantRole(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) UserRevokeRole(ctx context.Context, r *pb.AuthUserRevokeRoleRequest) (*pb.AuthUserRevokeRoleResponse, error) {
	resp, err := ap.authClient.UserRevokeRole(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}

func (ap *AuthProxy) UserChangePassword(ctx context.Context, r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error) {
	resp, err := ap.authClient.UserChangePassword(ctx, r)
	ap.flushCacheOnMutation(err)
	return resp, err
}
