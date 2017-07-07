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
	"strings"
	"testing"

	"github.com/coreos/etcd/auth/authpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/backend"
)

func setupCasbinAuthStore(t *testing.T) (store *casbinAuthStore, teardownfunc func(t *testing.T)) {
	b, tPath := backend.NewDefaultTmpBackend()

	tp, err := NewTokenProvider("simple", dummyIndexWaiter)
	if err != nil {
		t.Fatal(err)
	}
	as := NewCasbinAuthStore(b, tp)
	err = enableCasbinAuthAndCreateUsers(as)
	if err != nil {
		t.Fatal(err)
	}

	tearDown := func(t *testing.T) {
		b.Close()
		os.Remove(tPath)
		as.s.Close()
	}
	return as, tearDown
}

func initRoot(as *casbinAuthStore) error {
	_, err := as.s.UserAdd(&pb.AuthUserAddRequest{Name: "root", Password: "root"})
	if err != nil {
		return err
	}

	_, err = as.s.RoleAdd(&pb.AuthRoleAddRequest{Name: "root"})
	if err != nil {
		return err
	}

	_, err = as.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "root", Role: "root"})
	if err != nil {
		return err
	}

	_, err = as.s.UserGrantRole(&pb.AuthUserGrantRoleRequest{User: "root", Role: "root"})
	return err
}

func initUsers(as *casbinAuthStore) error {
	_, err := as.s.UserAdd(&pb.AuthUserAddRequest{Name: "alice", Password: "test"})
	if err != nil {
		return err
	}

	_, err = as.s.UserAdd(&pb.AuthUserAddRequest{Name: "bob", Password: "test"})
	if err != nil {
		return err
	}

	_, err = as.s.UserAdd(&pb.AuthUserAddRequest{Name: "cathy", Password: "test"})
	if err != nil {
		return err
	}

	_, err = as.s.UserAdd(&pb.AuthUserAddRequest{Name: "david", Password: "test"})
	return err
}

func initPermissions(as *casbinAuthStore) {
	e := as.enforcer

	e.AddPermissionForUser("alice", "/dataset1/*", "*", "READ")
	e.AddPermissionForUser("alice", "/dataset1/resource1", "*", "WRITE")

	e.AddPermissionForUser("bob", "/dataset2/resource1", "*", "*")
	e.AddPermissionForUser("bob", "/dataset2/resource2", "*", "READ")
	e.AddPermissionForUser("bob", "/dataset2/folder1/*", "*", "WRITE")

	e.AddPermissionForUser("dataset1_admin", "/dataset1/*", "*", "*")
	e.AddPermissionForUser("dataset2_admin", "/dataset2/*", "*", "*")
	e.AddPermissionForUser("root", "/*", "*", "*")

	e.AddRoleForUser("dataset_admin", "dataset1_admin")
	e.AddRoleForUser("dataset_admin", "dataset2_admin")
	e.AddRoleForUser("cathy", "dataset_admin")
	e.AddRoleForUser("david", "root")

	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	e.SavePolicy()
	e.ClearPolicy()
	e.LoadPolicy()
}

func enableCasbinAuthAndCreateUsers(as *casbinAuthStore) error {
	err := initRoot(as)
	if err != nil {
		return err
	}

	err = initUsers(as)
	if err != nil {
		return err
	}

	initPermissions(as)

	return as.s.AuthEnable()
}

func testEnforce(t *testing.T, as *casbinAuthStore, user string, key string, rangeEnd string, permType string, res bool) {
	err := as.isOpPermitted(user, 0, []byte(key), []byte(rangeEnd), authpb.Permission_Type(authpb.Permission_Type_value[strings.ToUpper(permType)]))

	myRes := false
	if err == nil {
		myRes = true
	}

	if myRes != res {
		t.Errorf("%s, %s, %s, %s: %t, supposed to be %t", user, key, rangeEnd, permType, !res, res)
	}
}

func TestBasic(t *testing.T) {
	as, tearDown := setupCasbinAuthStore(t)
	defer tearDown(t)

	// Test basic usage for policy rules.

	// alice is allowed to:
	// read /dataset1/*
	// write /dataset1/resource1
	testEnforce(t, as, "alice", "/dataset1/resource1", "", "READ", true)
	testEnforce(t, as, "alice", "/dataset1/resource1", "", "WRITE", true)
	testEnforce(t, as, "alice", "/dataset1/resource2", "", "READ", true)
	testEnforce(t, as, "alice", "/dataset1/resource2", "", "WRITE", false)
}

func TestPathWildcard(t *testing.T) {
	as, tearDown := setupCasbinAuthStore(t)
	defer tearDown(t)

	// Test the wildcard for the key in policy rules.

	// bob is allowed to:
	// read/write /dataset2/resource1
	// read /dataset2/resource2
	// write /dataset2/folder1/*
	testEnforce(t, as, "bob", "/dataset2/resource1", "", "READ", true)
	testEnforce(t, as, "bob", "/dataset2/resource1", "", "WRITE", true)
	testEnforce(t, as, "bob", "/dataset2/resource2", "", "READ", true)
	testEnforce(t, as, "bob", "/dataset2/resource2", "", "WRITE", false)

	testEnforce(t, as, "bob", "/dataset2/folder1/item1", "", "READ", false)
	testEnforce(t, as, "bob", "/dataset2/folder1/item1", "", "WRITE", true)
	testEnforce(t, as, "bob", "/dataset2/folder1/item2", "", "READ", false)
	testEnforce(t, as, "bob", "/dataset2/folder1/item2", "", "WRITE", true)
}

func TestRBAC(t *testing.T) {
	as, tearDown := setupCasbinAuthStore(t)
	defer tearDown(t)

	// Test the RBAC usage with multiple level role inheritance.

	// cathy can access all /dataset1/* and /dataset2/* resources because it has the dataset_admin role.
	// And dataset_admin role has the dataset1_admin and dataset2_admin roles.

	// Current role inheritance tree:
	//       dataset1_admin    dataset2_admin
	//                     \  /
	//                 dataset_admin
	//                      |
	//                    cathy
	testEnforce(t, as, "cathy", "/dataset1/item", "", "READ", true)
	testEnforce(t, as, "cathy", "/dataset1/item", "", "WRITE", true)
	testEnforce(t, as, "cathy", "/dataset2/item", "", "READ", true)
	testEnforce(t, as, "cathy", "/dataset2/item", "", "WRITE", true)
	testEnforce(t, as, "cathy", "/dataset3/item", "", "READ", false)
	testEnforce(t, as, "cathy", "/dataset3/item", "", "WRITE", false)
}

func TestRoot(t *testing.T) {
	as, tearDown := setupCasbinAuthStore(t)
	defer tearDown(t)

	// Test the root role.

	// david can access all resources because it has the root role.
	// And root role can do anything, including accessing the /dataset3/* resources,
	// which are not explicitly granted in the policy.
	testEnforce(t, as, "david", "/dataset1/item", "", "READ", true)
	testEnforce(t, as, "david", "/dataset1/item", "", "WRITE", true)
	testEnforce(t, as, "david", "/dataset2/item", "", "READ", true)
	testEnforce(t, as, "david", "/dataset2/item", "", "WRITE", true)
	testEnforce(t, as, "david", "/dataset3/item", "", "READ", true)
	testEnforce(t, as, "david", "/dataset3/item", "", "WRITE", true)
}
