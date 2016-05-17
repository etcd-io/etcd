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

package clientv3

import (
	"fmt"
	"strings"

	"github.com/coreos/etcd/auth/authpb"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type (
	AuthEnableResponse             pb.AuthEnableResponse
	AuthDisableResponse            pb.AuthDisableResponse
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

	// AuthDisable disables auth of an etcd cluster.
	AuthDisable(ctx context.Context) (*AuthDisableResponse, error)

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
	return (*AuthEnableResponse)(resp), rpctypes.Error(err)
}

func (auth *auth) AuthDisable(ctx context.Context) (*AuthDisableResponse, error) {
	resp, err := auth.remote.AuthDisable(ctx, &pb.AuthDisableRequest{})
	return (*AuthDisableResponse)(resp), rpctypes.Error(err)
}

func (auth *auth) UserAdd(ctx context.Context, name string, password string) (*AuthUserAddResponse, error) {
	resp, err := auth.remote.UserAdd(ctx, &pb.AuthUserAddRequest{Name: name, Password: password})
	return (*AuthUserAddResponse)(resp), rpctypes.Error(err)
}

func (auth *auth) UserDelete(ctx context.Context, name string) (*AuthUserDeleteResponse, error) {
	resp, err := auth.remote.UserDelete(ctx, &pb.AuthUserDeleteRequest{Name: name})
	return (*AuthUserDeleteResponse)(resp), rpctypes.Error(err)
}

func (auth *auth) UserChangePassword(ctx context.Context, name string, password string) (*AuthUserChangePasswordResponse, error) {
	resp, err := auth.remote.UserChangePassword(ctx, &pb.AuthUserChangePasswordRequest{Name: name, Password: password})
	return (*AuthUserChangePasswordResponse)(resp), rpctypes.Error(err)
}

func (auth *auth) UserGrant(ctx context.Context, user string, role string) (*AuthUserGrantResponse, error) {
	resp, err := auth.remote.UserGrant(ctx, &pb.AuthUserGrantRequest{User: user, Role: role})
	return (*AuthUserGrantResponse)(resp), rpctypes.Error(err)
}

func (auth *auth) RoleAdd(ctx context.Context, name string) (*AuthRoleAddResponse, error) {
	resp, err := auth.remote.RoleAdd(ctx, &pb.AuthRoleAddRequest{Name: name})
	return (*AuthRoleAddResponse)(resp), rpctypes.Error(err)
}

func (auth *auth) RoleGrant(ctx context.Context, name string, key string, permType PermissionType) (*AuthRoleGrantResponse, error) {
	perm := &authpb.Permission{
		Key:      []byte(key),
		PermType: authpb.Permission_Type(permType),
	}
	resp, err := auth.remote.RoleGrant(ctx, &pb.AuthRoleGrantRequest{Name: name, Perm: perm})
	return (*AuthRoleGrantResponse)(resp), rpctypes.Error(err)
}

func StrToPermissionType(s string) (PermissionType, error) {
	val, ok := authpb.Permission_Type_value[strings.ToUpper(s)]
	if ok {
		return PermissionType(val), nil
	}
	return PermissionType(-1), fmt.Errorf("invalid permission type: %s", s)
}

type authenticator struct {
	conn   *grpc.ClientConn // conn in-use
	remote pb.AuthClient
}

func (auth *authenticator) authenticate(ctx context.Context, name string, password string) (*AuthenticateResponse, error) {
	resp, err := auth.remote.Authenticate(ctx, &pb.AuthenticateRequest{Name: name, Password: password})
	return (*AuthenticateResponse)(resp), rpctypes.Error(err)
}

func (auth *authenticator) close() {
	auth.conn.Close()
}

func newAuthenticator(endpoint string, opts []grpc.DialOption) (*authenticator, error) {
	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, err
	}

	return &authenticator{
		conn:   conn,
		remote: pb.NewAuthClient(conn),
	}, nil
}
