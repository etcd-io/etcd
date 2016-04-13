// Copyright 2016 Nippon Telegraph and Telephone Corporation.
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

package clientv3

import (
	"fmt"
	"strings"

	"github.com/coreos/etcd/auth/authpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type (
	AuthEnableResponse             pb.AuthEnableResponse
	AuthenticateResponse           pb.AuthenticateResponse
	AuthUserAddResponse            pb.AuthUserAddResponse
	AuthUserDeleteResponse         pb.AuthUserDeleteResponse
	AuthUserChangePasswordResponse pb.AuthUserChangePasswordResponse
	AuthUserGrantResponse          pb.AuthUserGrantResponse
	AuthRoleAddResponse            pb.AuthRoleAddResponse
	AuthRoleGrantResponse          pb.AuthRoleGrantResponse

	PermissionType authpb.Permission_Type
)

const (
	PermRead      = authpb.READ
	PermWrite     = authpb.WRITE
	PermReadWrite = authpb.READWRITE
)

type Auth interface {
	// AuthEnable enables auth of an etcd cluster.
	AuthEnable(ctx context.Context) (*AuthEnableResponse, error)

	// Authenticate does authenticate with given user name and password.
	Authenticate(ctx context.Context, name string, password string) (*AuthenticateResponse, error)

	// UserAdd adds a new user to an etcd cluster.
	UserAdd(ctx context.Context, name string, password string) (*AuthUserAddResponse, error)

	// UserDelete deletes a user from an etcd cluster.
	UserDelete(ctx context.Context, name string) (*AuthUserDeleteResponse, error)

	// UserChangePassword changes a password of a user.
	UserChangePassword(ctx context.Context, name string, password string) (*AuthUserChangePasswordResponse, error)

	// UserGrant grants a role to a user.
	UserGrant(ctx context.Context, user string, role string) (*AuthUserGrantResponse, error)

	// RoleAdd adds a new role to an etcd cluster.
	RoleAdd(ctx context.Context, name string) (*AuthRoleAddResponse, error)

	// RoleGrant grants a permission to a role.
	RoleGrant(ctx context.Context, name string, key string, permType PermissionType) (*AuthRoleGrantResponse, error)
}

type auth struct {
	c *Client

	conn   *grpc.ClientConn // conn in-use
	remote pb.AuthClient
}

func NewAuth(c *Client) Auth {
	conn := c.ActiveConnection()
	return &auth{
		conn:   c.ActiveConnection(),
		remote: pb.NewAuthClient(conn),
		c:      c,
	}
}

func (auth *auth) AuthEnable(ctx context.Context) (*AuthEnableResponse, error) {
	resp, err := auth.remote.AuthEnable(ctx, &pb.AuthEnableRequest{})
	return (*AuthEnableResponse)(resp), err
}

func (auth *auth) Authenticate(ctx context.Context, name string, password string) (*AuthenticateResponse, error) {
	resp, err := auth.remote.Authenticate(ctx, &pb.AuthenticateRequest{Name: name, Password: password})
	return (*AuthenticateResponse)(resp), err
}

func (auth *auth) UserAdd(ctx context.Context, name string, password string) (*AuthUserAddResponse, error) {
	resp, err := auth.remote.UserAdd(ctx, &pb.AuthUserAddRequest{Name: name, Password: password})
	return (*AuthUserAddResponse)(resp), err
}

func (auth *auth) UserDelete(ctx context.Context, name string) (*AuthUserDeleteResponse, error) {
	resp, err := auth.remote.UserDelete(ctx, &pb.AuthUserDeleteRequest{Name: name})
	return (*AuthUserDeleteResponse)(resp), err
}

func (auth *auth) UserChangePassword(ctx context.Context, name string, password string) (*AuthUserChangePasswordResponse, error) {
	resp, err := auth.remote.UserChangePassword(ctx, &pb.AuthUserChangePasswordRequest{Name: name, Password: password})
	return (*AuthUserChangePasswordResponse)(resp), err
}

func (auth *auth) UserGrant(ctx context.Context, user string, role string) (*AuthUserGrantResponse, error) {
	resp, err := auth.remote.UserGrant(ctx, &pb.AuthUserGrantRequest{User: user, Role: role})
	return (*AuthUserGrantResponse)(resp), err
}

func (auth *auth) RoleAdd(ctx context.Context, name string) (*AuthRoleAddResponse, error) {
	resp, err := auth.remote.RoleAdd(ctx, &pb.AuthRoleAddRequest{Name: name})
	return (*AuthRoleAddResponse)(resp), err
}

func (auth *auth) RoleGrant(ctx context.Context, name string, key string, permType PermissionType) (*AuthRoleGrantResponse, error) {
	perm := &authpb.Permission{
		Key:      []byte(key),
		PermType: authpb.Permission_Type(permType),
	}
	resp, err := auth.remote.RoleGrant(ctx, &pb.AuthRoleGrantRequest{Name: name, Perm: perm})
	return (*AuthRoleGrantResponse)(resp), err
}

func StrToPermissionType(s string) (PermissionType, error) {
	val, ok := authpb.Permission_Type_value[strings.ToUpper(s)]
	if ok {
		return PermissionType(val), nil
	}
	return PermissionType(-1), fmt.Errorf("invalid permission type: %s", s)
}
