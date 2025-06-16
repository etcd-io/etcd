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

package clientv3test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestUserError(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authapi := clus.RandClient()

	_, err := authapi.UserAdd(t.Context(), "foo", "bar")
	require.NoError(t, err)

	_, err = authapi.UserAdd(t.Context(), "foo", "bar")
	require.ErrorIsf(t, err, rpctypes.ErrUserAlreadyExist, "expected %v, got %v", rpctypes.ErrUserAlreadyExist, err)

	_, err = authapi.UserDelete(t.Context(), "not-exist-user")
	require.ErrorIsf(t, err, rpctypes.ErrUserNotFound, "expected %v, got %v", rpctypes.ErrUserNotFound, err)

	_, err = authapi.UserGrantRole(t.Context(), "foo", "test-role-does-not-exist")
	require.ErrorIsf(t, err, rpctypes.ErrRoleNotFound, "expected %v, got %v", rpctypes.ErrRoleNotFound, err)
}

func TestAddUserAfterDelete(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authapi := clus.RandClient()
	authSetupRoot(t, authapi.Auth)
	cfg := clientv3.Config{
		Endpoints:   authapi.Endpoints(),
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	cfg.Username, cfg.Password = "root", "123"
	authed, err := integration2.NewClient(t, cfg)
	require.NoError(t, err)
	defer authed.Close()

	// add user
	_, err = authed.UserAdd(t.Context(), "foo", "bar")
	require.NoError(t, err)
	_, err = authapi.Authenticate(t.Context(), "foo", "bar")
	require.NoError(t, err)
	// delete user
	_, err = authed.UserDelete(t.Context(), "foo")
	require.NoError(t, err)
	if _, err = authed.Authenticate(t.Context(), "foo", "bar"); err == nil {
		t.Errorf("expect Authenticate error for old password")
	}
	// add user back
	_, err = authed.UserAdd(t.Context(), "foo", "bar")
	require.NoError(t, err)
	_, err = authed.Authenticate(t.Context(), "foo", "bar")
	require.NoError(t, err)
	// change password
	_, err = authed.UserChangePassword(t.Context(), "foo", "bar2")
	require.NoError(t, err)
	_, err = authed.UserChangePassword(t.Context(), "foo", "bar1")
	require.NoError(t, err)

	if _, err = authed.Authenticate(t.Context(), "foo", "bar"); err == nil {
		t.Errorf("expect Authenticate error for old password")
	}
	if _, err = authed.Authenticate(t.Context(), "foo", "bar2"); err == nil {
		t.Errorf("expect Authenticate error for old password")
	}
	_, err = authed.Authenticate(t.Context(), "foo", "bar1")
	require.NoError(t, err)
}

func TestUserErrorAuth(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authapi := clus.RandClient()
	authSetupRoot(t, authapi.Auth)

	// unauthenticated client
	_, err := authapi.UserAdd(t.Context(), "foo", "bar")
	require.ErrorIsf(t, err, rpctypes.ErrUserEmpty, "expected %v, got %v", rpctypes.ErrUserEmpty, err)

	// wrong id or password
	cfg := clientv3.Config{
		Endpoints:   authapi.Endpoints(),
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	cfg.Username, cfg.Password = "wrong-id", "123"
	_, err = integration2.NewClient(t, cfg)
	require.ErrorIsf(t, err, rpctypes.ErrAuthFailed, "expected %v, got %v", rpctypes.ErrAuthFailed, err)
	cfg.Username, cfg.Password = "root", "wrong-pass"
	_, err = integration2.NewClient(t, cfg)
	require.ErrorIsf(t, err, rpctypes.ErrAuthFailed, "expected %v, got %v", rpctypes.ErrAuthFailed, err)

	cfg.Username, cfg.Password = "root", "123"
	authed, err := integration2.NewClient(t, cfg)
	require.NoError(t, err)
	defer authed.Close()

	_, err = authed.UserList(t.Context())
	require.NoError(t, err)
}

func authSetupRoot(t *testing.T, auth clientv3.Auth) {
	_, err := auth.UserAdd(t.Context(), "root", "123")
	require.NoError(t, err)
	_, err = auth.RoleAdd(t.Context(), "root")
	require.NoError(t, err)
	_, err = auth.UserGrantRole(t.Context(), "root", "root")
	require.NoError(t, err)
	_, err = auth.AuthEnable(t.Context())
	require.NoError(t, err)
}

// TestGetTokenWithoutAuth is when Client can connect to etcd even if they
// supply credentials and the server is in AuthDisable mode.
func TestGetTokenWithoutAuth(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 2})
	defer clus.Terminate(t)

	authapi := clus.RandClient()

	var err error
	var client *clientv3.Client

	// make sure "auth" was disabled
	_, err = authapi.AuthDisable(t.Context())
	require.NoError(t, err)

	// "Username" and "Password" must be used
	cfg := clientv3.Config{
		Endpoints:   authapi.Endpoints(),
		DialTimeout: 5 * time.Second,
		Username:    "root",
		Password:    "123",
	}

	client, err = integration2.NewClient(t, cfg)
	if err == nil {
		defer client.Close()
	}

	switch {
	case err == nil:
		t.Log("passes as expected")
	case errors.Is(err, context.DeadlineExceeded):
		t.Errorf("not expected result:%v with endpoint:%s", err, authapi.Endpoints())
	default:
		t.Errorf("other errors:%v", err)
	}
}
