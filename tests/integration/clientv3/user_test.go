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
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/integration"
	"google.golang.org/grpc"
)

func TestUserError(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authapi := clus.RandClient()

	_, err := authapi.UserAdd(context.TODO(), "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}

	_, err = authapi.UserAdd(context.TODO(), "foo", "bar")
	if err != rpctypes.ErrUserAlreadyExist {
		t.Fatalf("expected %v, got %v", rpctypes.ErrUserAlreadyExist, err)
	}

	_, err = authapi.UserDelete(context.TODO(), "not-exist-user")
	if err != rpctypes.ErrUserNotFound {
		t.Fatalf("expected %v, got %v", rpctypes.ErrUserNotFound, err)
	}

	_, err = authapi.UserGrantRole(context.TODO(), "foo", "test-role-does-not-exist")
	if err != rpctypes.ErrRoleNotFound {
		t.Fatalf("expected %v, got %v", rpctypes.ErrRoleNotFound, err)
	}
}

func TestUserErrorAuth(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	authapi := clus.RandClient()
	authSetupRoot(t, authapi.Auth)

	// unauthenticated client
	if _, err := authapi.UserAdd(context.TODO(), "foo", "bar"); err != rpctypes.ErrUserEmpty {
		t.Fatalf("expected %v, got %v", rpctypes.ErrUserEmpty, err)
	}

	// wrong id or password
	cfg := clientv3.Config{
		Endpoints:   authapi.Endpoints(),
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	cfg.Username, cfg.Password = "wrong-id", "123"
	if _, err := integration.NewClient(t, cfg); err != rpctypes.ErrAuthFailed {
		t.Fatalf("expected %v, got %v", rpctypes.ErrAuthFailed, err)
	}
	cfg.Username, cfg.Password = "root", "wrong-pass"
	if _, err := integration.NewClient(t, cfg); err != rpctypes.ErrAuthFailed {
		t.Fatalf("expected %v, got %v", rpctypes.ErrAuthFailed, err)
	}

	cfg.Username, cfg.Password = "root", "123"
	authed, err := integration.NewClient(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer authed.Close()

	if _, err := authed.UserList(context.TODO()); err != nil {
		t.Fatal(err)
	}
}

func authSetupRoot(t *testing.T, auth clientv3.Auth) {
	if _, err := auth.UserAdd(context.TODO(), "root", "123"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.RoleAdd(context.TODO(), "root"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.UserGrantRole(context.TODO(), "root", "root"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.AuthEnable(context.TODO()); err != nil {
		t.Fatal(err)
	}
}

// Client can connect to etcd even if they supply credentials and the server is in AuthDisable mode.
func TestGetTokenWithoutAuth(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 2})
	defer clus.Terminate(t)

	authapi := clus.RandClient()

	var err error
	var client *clientv3.Client

	// make sure "auth" was disabled
	if _, err = authapi.AuthDisable(context.TODO()); err != nil {
		t.Fatal(err)
	}

	// "Username" and "Password" must be used
	cfg := clientv3.Config{
		Endpoints:   authapi.Endpoints(),
		DialTimeout: 5 * time.Second,
		Username:    "root",
		Password:    "123",
	}

	client, err = integration.NewClient(t, cfg)
	if err == nil {
		defer client.Close()
	}

	switch err {
	case nil:
		t.Log("passes as expected")
	case context.DeadlineExceeded:
		t.Errorf("not expected result:%v with endpoint:%s", err, authapi.Endpoints())
	default:
		t.Errorf("other errors:%v", err)
	}
}
