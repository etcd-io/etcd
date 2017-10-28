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

package readonly

import (
	"context"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type readOnlyAuthProxy struct {
	pb.AuthServer
}

func NewReadOnlyAuthProxy(ap pb.AuthServer) pb.AuthServer {
	return &readOnlyAuthProxy{AuthServer: ap}
}

func (ap *readOnlyAuthProxy) AuthEnable(ctx context.Context, r *pb.AuthEnableRequest) (*pb.AuthEnableResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) AuthDisable(ctx context.Context, r *pb.AuthDisableRequest) (*pb.AuthDisableResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) RoleAdd(ctx context.Context, r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) RoleDelete(ctx context.Context, r *pb.AuthRoleDeleteRequest) (*pb.AuthRoleDeleteResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) RoleRevokePermission(ctx context.Context, r *pb.AuthRoleRevokePermissionRequest) (*pb.AuthRoleRevokePermissionResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) RoleGrantPermission(ctx context.Context, r *pb.AuthRoleGrantPermissionRequest) (*pb.AuthRoleGrantPermissionResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) UserAdd(ctx context.Context, r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) UserDelete(ctx context.Context, r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) UserGrantRole(ctx context.Context, r *pb.AuthUserGrantRoleRequest) (*pb.AuthUserGrantRoleResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) UserRevokeRole(ctx context.Context, r *pb.AuthUserRevokeRoleRequest) (*pb.AuthUserRevokeRoleResponse, error) {
	return nil, ErrReadOnly
}

func (ap *readOnlyAuthProxy) UserChangePassword(ctx context.Context, r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error) {
	return nil, ErrReadOnly
}
