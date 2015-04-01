// Copyright 2015 CoreOS, Inc.
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

package security

import (
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/crypto/bcrypt"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/store"
)

func TestMergeUser(t *testing.T) {
	tbl := []struct {
		input  User
		merge  User
		expect User
		iserr  bool
	}{
		{
			User{User: "foo"},
			User{User: "bar"},
			User{},
			true,
		},
		{
			User{User: "foo"},
			User{User: "foo"},
			User{User: "foo", Roles: []string{}},
			false,
		},
		{
			User{User: "foo"},
			User{User: "foo", Grant: []string{"role1"}},
			User{User: "foo", Roles: []string{"role1"}},
			false,
		},
		{
			User{User: "foo", Roles: []string{"role1"}},
			User{User: "foo", Grant: []string{"role1"}},
			User{},
			true,
		},
		{
			User{User: "foo", Roles: []string{"role1"}},
			User{User: "foo", Revoke: []string{"role2"}},
			User{},
			true,
		},
		{
			User{User: "foo", Roles: []string{"role1"}},
			User{User: "foo", Grant: []string{"role2"}},
			User{User: "foo", Roles: []string{"role1", "role2"}},
			false,
		},
		{
			User{User: "foo"},
			User{User: "foo", Password: "bar"},
			User{User: "foo", Roles: []string{}, Password: "$2a$10$aUPOdbOGNawaVSusg3g2wuC3AH6XxIr9/Ms4VgDvzrAVOJPYzZILa"},
			false,
		},
	}

	for i, tt := range tbl {
		out, err := tt.input.Merge(tt.merge)
		if err != nil && !tt.iserr {
			t.Fatalf("Got unexpected error on item %d", i)
		}
		if !tt.iserr {
			err := bcrypt.CompareHashAndPassword([]byte(out.Password), []byte(tt.merge.Password))
			if err == nil {
				tt.expect.Password = out.Password
			}
			if !reflect.DeepEqual(out, tt.expect) {
				t.Errorf("Unequal merge expectation on item %d: got: %#v, expect: %#v", i, out, tt.expect)
			}
		}
	}
}

func TestMergeRole(t *testing.T) {
	tbl := []struct {
		input  Role
		merge  Role
		expect Role
		iserr  bool
	}{
		{
			Role{Role: "foo"},
			Role{Role: "bar"},
			Role{},
			true,
		},
		{
			Role{Role: "foo"},
			Role{Role: "foo", Grant: &Permissions{KV: rwPermission{Read: []string{"/foodir"}, Write: []string{"/foodir"}}}},
			Role{Role: "foo", Permissions: Permissions{KV: rwPermission{Read: []string{"/foodir"}, Write: []string{"/foodir"}}}},
			false,
		},
		{
			Role{Role: "foo", Permissions: Permissions{KV: rwPermission{Read: []string{"/foodir"}, Write: []string{"/foodir"}}}},
			Role{Role: "foo", Revoke: &Permissions{KV: rwPermission{Read: []string{"/foodir"}, Write: []string{"/foodir"}}}},
			Role{Role: "foo", Permissions: Permissions{KV: rwPermission{Read: []string{}, Write: []string{}}}},
			false,
		},
		{
			Role{Role: "foo", Permissions: Permissions{KV: rwPermission{Read: []string{"/bardir"}}}},
			Role{Role: "foo", Revoke: &Permissions{KV: rwPermission{Read: []string{"/foodir"}}}},
			Role{},
			true,
		},
	}
	for i, tt := range tbl {
		out, err := tt.input.Merge(tt.merge)
		if err != nil && !tt.iserr {
			t.Fatalf("Got unexpected error on item %d", i)
		}
		if !tt.iserr {
			if !reflect.DeepEqual(out, tt.expect) {
				t.Errorf("Unequal merge expectation on item %d: got: %#v, expect: %#v", i, out, tt.expect)
			}
		}
	}
}

type testDoer struct {
	get   []etcdserver.Response
	index int
}

func (td *testDoer) Do(_ context.Context, req etcdserverpb.Request) (etcdserver.Response, error) {
	if req.Method == "GET" {
		res := td.get[td.index]
		td.index++
		return res, nil
	}
	return etcdserver.Response{}, nil
}

func TestAllUsers(t *testing.T) {
	d := &testDoer{
		get: []etcdserver.Response{
			{
				Event: &store.Event{
					Action: store.Get,
					Node: &store.NodeExtern{
						Nodes: store.NodeExterns{
							&store.NodeExtern{
								Key: StorePermsPrefix + "/users/cat",
							},
							&store.NodeExtern{
								Key: StorePermsPrefix + "/users/dog",
							},
						},
					},
				},
			},
		},
	}
	expected := []string{"cat", "dog"}

	s := Store{d, time.Second, false}
	users, err := s.AllUsers()
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if !reflect.DeepEqual(users, expected) {
		t.Error("AllUsers doesn't match given store. Got", users, "expected", expected)
	}
}

func TestGetAndDeleteUser(t *testing.T) {
	data := `{"user": "cat", "roles" : ["animal"]}`
	d := &testDoer{
		get: []etcdserver.Response{
			{
				Event: &store.Event{
					Action: store.Get,
					Node: &store.NodeExtern{
						Key:   StorePermsPrefix + "/users/cat",
						Value: &data,
					},
				},
			},
		},
	}
	expected := User{User: "cat", Roles: []string{"animal"}}

	s := Store{d, time.Second, false}
	out, err := s.GetUser("cat")
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if !reflect.DeepEqual(out, expected) {
		t.Error("GetUser doesn't match given store. Got", out, "expected", expected)
	}
	err = s.DeleteUser("cat")
	if err != nil {
		t.Error("Unexpected error", err)
	}
}

func TestAllRoles(t *testing.T) {
	d := &testDoer{
		get: []etcdserver.Response{
			{
				Event: &store.Event{
					Action: store.Get,
					Node: &store.NodeExtern{
						Nodes: store.NodeExterns{
							&store.NodeExtern{
								Key: StorePermsPrefix + "/roles/animal",
							},
							&store.NodeExtern{
								Key: StorePermsPrefix + "/roles/human",
							},
						},
					},
				},
			},
		},
	}
	expected := []string{"animal", "human", "root"}

	s := Store{d, time.Second, false}
	out, err := s.AllRoles()
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if !reflect.DeepEqual(out, expected) {
		t.Error("AllRoles doesn't match given store. Got", out, "expected", expected)
	}
}

func TestGetAndDeleteRole(t *testing.T) {
	data := `{"role": "animal"}`
	d := &testDoer{
		get: []etcdserver.Response{
			{
				Event: &store.Event{
					Action: store.Get,
					Node: &store.NodeExtern{
						Key:   StorePermsPrefix + "/roles/animal",
						Value: &data,
					},
				},
			},
		},
	}
	expected := Role{Role: "animal"}

	s := Store{d, time.Second, false}
	out, err := s.GetRole("animal")
	if err != nil {
		t.Error("Unexpected error", err)
	}
	if !reflect.DeepEqual(out, expected) {
		t.Error("GetRole doesn't match given store. Got", out, "expected", expected)
	}
	err = s.DeleteRole("animal")
	if err != nil {
		t.Error("Unexpected error", err)
	}
}

func TestEnsure(t *testing.T) {
	d := &testDoer{
		get: []etcdserver.Response{
			{
				Event: &store.Event{
					Action: store.Set,
					Node: &store.NodeExtern{
						Key: StorePermsPrefix,
						Dir: true,
					},
				},
			},
			{
				Event: &store.Event{
					Action: store.Set,
					Node: &store.NodeExtern{
						Key: StorePermsPrefix + "/users/",
						Dir: true,
					},
				},
			},
			{
				Event: &store.Event{
					Action: store.Set,
					Node: &store.NodeExtern{
						Key: StorePermsPrefix + "/roles/",
						Dir: true,
					},
				},
			},
		},
	}

	s := Store{d, time.Second, false}
	err := s.ensureSecurityDirectories()
	if err != nil {
		t.Error("Unexpected error", err)
	}
}

func TestSimpleMatch(t *testing.T) {
	role := Role{Role: "foo", Permissions: Permissions{KV: rwPermission{Read: []string{"/foodir/*", "/fookey"}, Write: []string{"/bardir/*", "/barkey"}}}}
	if !role.HasKeyAccess("/foodir/foo/bar", false) {
		t.Fatal("role lacks expected access")
	}
	if !role.HasKeyAccess("/fookey", false) {
		t.Fatal("role lacks expected access")
	}
	if role.HasKeyAccess("/bardir/bar/foo", false) {
		t.Fatal("role has unexpected access")
	}
	if role.HasKeyAccess("/barkey", false) {
		t.Fatal("role has unexpected access")
	}
	if role.HasKeyAccess("/foodir/foo/bar", true) {
		t.Fatal("role has unexpected access")
	}
	if role.HasKeyAccess("/fookey", true) {
		t.Fatal("role has unexpected access")
	}
	if !role.HasKeyAccess("/bardir/bar/foo", true) {
		t.Fatal("role lacks expected access")
	}
	if !role.HasKeyAccess("/barkey", true) {
		t.Fatal("role lacks expected access")
	}
}
