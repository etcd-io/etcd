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

package auth

import (
	"os"
	"testing"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/backend"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
)

func init() { BcryptCost = bcrypt.MinCost }

func dummyIndexWaiter(index uint64) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		ch <- struct{}{}
	}()
	return ch
}

func setupAuthStore(t *testing.T) (store *authStore, teardownfunc func(t *testing.T)) {
	b, tPath := backend.NewDefaultTmpBackend()

	as := NewAuthStore(b, dummyIndexWaiter)
	err := enableAuthAndCreateRoot(as)
	if err != nil {
		t.Fatal(err)
	}

	// adds a new role
	_, err = as.RoleAdd(&pb.AuthRoleAddRequest{Name: "role-test"})
	if err != nil {
		t.Fatal(err)
	}

	ua := &pb.AuthUserAddRequest{Name: "foo", Password: "bar"}
	_, err = as.UserAdd(ua) // add a non-existing user
	if err != nil {
		t.Fatal(err)
	}

	tearDown := func(t *testing.T) {
		b.Close()
		os.Remove(tPath)
		as.Close()
	}
	return as, tearDown
}

func enableAuthAndCreateRoot(as *authStore) error {
	_, err := as.UserAdd(&pb.AuthUserAddRequest{Name: "root", Password: "root"})
	if err != nil {
		return err
	}

	_, err = as.RoleAdd(&pb.AuthRoleAddRequest{Name: "root"})
	if err != nil {
		return err
	}

	_, err = as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "root", Role: "root"})
	if err != nil {
		return err
	}

	return as.AuthEnable()
}

func TestUserAdd(t *testing.T) {
	as, tearDown := setupAuthStore(t)
	defer tearDown(t)

	ua := &pb.AuthUserAddRequest{Name: "foo"}
	_, err := as.UserAdd(ua) // add an existing user
	if err == nil {
		t.Fatalf("expected %v, got %v", ErrUserAlreadyExist, err)
	}
	if err != ErrUserAlreadyExist {
		t.Fatalf("expected %v, got %v", ErrUserAlreadyExist, err)
	}

	ua = &pb.AuthUserAddRequest{Name: ""}
	_, err = as.UserAdd(ua) // add a user with empty name
	if err != ErrUserEmpty {
		t.Fatal(err)
	}
}

func TestCheckPassword(t *testing.T) {
	as, tearDown := setupAuthStore(t)
	defer tearDown(t)

	// auth a non-existing user
	_, err := as.CheckPassword("foo-test", "bar")
	if err == nil {
		t.Fatalf("expected %v, got %v", ErrAuthFailed, err)
	}
	if err != ErrAuthFailed {
		t.Fatalf("expected %v, got %v", ErrAuthFailed, err)
	}

	// auth an existing user with correct password
	_, err = as.CheckPassword("foo", "bar")
	if err != nil {
		t.Fatal(err)
	}

	// auth an existing user but with wrong password
	_, err = as.CheckPassword("foo", "")
	if err == nil {
		t.Fatalf("expected %v, got %v", ErrAuthFailed, err)
	}
	if err != ErrAuthFailed {
		t.Fatalf("expected %v, got %v", ErrAuthFailed, err)
	}
}

func TestUserDelete(t *testing.T) {
	as, tearDown := setupAuthStore(t)
	defer tearDown(t)

	// delete an existing user
	ud := &pb.AuthUserDeleteRequest{Name: "foo"}
	_, err := as.UserDelete(ud)
	if err != nil {
		t.Fatal(err)
	}

	// delete a non-existing user
	_, err = as.UserDelete(ud)
	if err == nil {
		t.Fatalf("expected %v, got %v", ErrUserNotFound, err)
	}
	if err != ErrUserNotFound {
		t.Fatalf("expected %v, got %v", ErrUserNotFound, err)
	}
}

func TestUserChangePassword(t *testing.T) {
	as, tearDown := setupAuthStore(t)
	defer tearDown(t)

	ctx1 := context.WithValue(context.WithValue(context.TODO(), "index", uint64(1)), "simpleToken", "dummy")
	_, err := as.Authenticate(ctx1, "foo", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = as.UserChangePassword(&pb.AuthUserChangePasswordRequest{Name: "foo", Password: "bar"})
	if err != nil {
		t.Fatal(err)
	}

	ctx2 := context.WithValue(context.WithValue(context.TODO(), "index", uint64(2)), "simpleToken", "dummy")
	_, err = as.Authenticate(ctx2, "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}

	// change a non-existing user
	_, err = as.UserChangePassword(&pb.AuthUserChangePasswordRequest{Name: "foo-test", Password: "bar"})
	if err == nil {
		t.Fatalf("expected %v, got %v", ErrUserNotFound, err)
	}
	if err != ErrUserNotFound {
		t.Fatalf("expected %v, got %v", ErrUserNotFound, err)
	}
}

func TestRoleAdd(t *testing.T) {
	as, tearDown := setupAuthStore(t)
	defer tearDown(t)

	// adds a new role
	_, err := as.RoleAdd(&pb.AuthRoleAddRequest{Name: "role-test-1"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUserGrant(t *testing.T) {
	as, tearDown := setupAuthStore(t)
	defer tearDown(t)

	// grants a role to the user
	_, err := as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "foo", Role: "role-test"})
	if err != nil {
		t.Fatal(err)
	}

	// grants a role to a non-existing user
	_, err = as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "foo-test", Role: "role-test"})
	if err == nil {
		t.Fatalf("expected %v, got %v", ErrUserNotFound, err)
	}
	if err != ErrUserNotFound {
		t.Fatalf("expected %v, got %v", ErrUserNotFound, err)
	}
}
