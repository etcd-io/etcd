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

package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/crypto/bcrypt"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	etcderr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/types"
)

const (
	// StorePermsPrefix is the internal prefix of the storage layer dedicated to storing user data.
	StorePermsPrefix = "/2"

	// RootRoleName is the name of the ROOT role, with privileges to manage the cluster.
	RootRoleName = "root"

	// GuestRoleName is the name of the role that defines the privileges of an unauthenticated user.
	GuestRoleName = "guest"
)

var (
	plog = capnslog.NewPackageLogger("github.com/coreos/etcd/etcdserver", "auth")
)

var rootRole = Role{
	Role: RootRoleName,
	Permissions: Permissions{
		KV: rwPermission{
			Read:  []string{"*"},
			Write: []string{"*"},
		},
	},
}

var guestRole = Role{
	Role: GuestRoleName,
	Permissions: Permissions{
		KV: rwPermission{
			Read:  []string{"*"},
			Write: []string{"*"},
		},
	},
}

type doer interface {
	Do(context.Context, etcdserverpb.Request) (etcdserver.Response, error)
}

type Store struct {
	server      doer
	timeout     time.Duration
	ensuredOnce bool
}

type User struct {
	User     string   `json:"user"`
	Password string   `json:"password,omitempty"`
	Roles    []string `json:"roles"`
	Grant    []string `json:"grant,omitempty"`
	Revoke   []string `json:"revoke,omitempty"`
}

type Role struct {
	Role        string       `json:"role"`
	Permissions Permissions  `json:"permissions"`
	Grant       *Permissions `json:"grant,omitempty"`
	Revoke      *Permissions `json:"revoke,omitempty"`
}

type Permissions struct {
	KV rwPermission `json:"kv"`
}

type rwPermission struct {
	Read  []string `json:"read"`
	Write []string `json:"write"`
}

type Error struct {
	httpStatus int
	errmsg     string
}

func (ae Error) Error() string   { return ae.errmsg }
func (ae Error) HTTPStatus() int { return ae.httpStatus }

func authErr(hs int, s string, v ...interface{}) Error {
	return Error{httpStatus: hs, errmsg: fmt.Sprintf("auth: "+s, v...)}
}

func NewStore(server doer, timeout time.Duration) *Store {
	s := &Store{
		server:  server,
		timeout: timeout,
	}
	return s
}

func (s *Store) AllUsers() ([]string, error) {
	resp, err := s.requestResource("/users/", false)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return []string{}, nil
			}
		}
		return nil, err
	}
	var nodes []string
	for _, n := range resp.Event.Node.Nodes {
		_, user := path.Split(n.Key)
		nodes = append(nodes, user)
	}
	sort.Strings(nodes)
	return nodes, nil
}

func (s *Store) GetUser(name string) (User, error) {
	resp, err := s.requestResource("/users/"+name, false)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return User{}, authErr(http.StatusNotFound, "User %s does not exist.", name)
			}
		}
		return User{}, err
	}
	var u User
	err = json.Unmarshal([]byte(*resp.Event.Node.Value), &u)
	if err != nil {
		return u, err
	}
	// Require that root always has a root role.
	if u.User == "root" {
		inRoles := false
		for _, r := range u.Roles {
			if r == RootRoleName {
				inRoles = true
				break
			}
		}
		if !inRoles {
			u.Roles = append(u.Roles, RootRoleName)
		}
	}

	return u, nil
}

func (s *Store) CreateOrUpdateUser(user User) (out User, created bool, err error) {
	_, err = s.GetUser(user.User)
	if err == nil {
		// Remove the update-user roles from updating downstream.
		// Roles are granted or revoked, not changed directly.
		user.Roles = nil
		out, err := s.UpdateUser(user)
		return out, false, err
	}
	u, err := s.CreateUser(user)
	return u, true, err
}

func (s *Store) CreateUser(user User) (User, error) {
	u, err := s.createUserInternal(user)
	if err == nil {
		plog.Noticef("created user %s", user.User)
	}
	return u, err
}

func (s *Store) createUserInternal(user User) (User, error) {
	if user.Password == "" {
		return user, authErr(http.StatusBadRequest, "Cannot create user %s with an empty password", user.User)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return user, err
	}
	user.Password = string(hash)

	_, err = s.createResource("/users/"+user.User, user)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeNodeExist {
				return user, authErr(http.StatusConflict, "User %s already exists.", user.User)
			}
		}
	}
	return user, err
}

func (s *Store) DeleteUser(name string) error {
	if s.AuthEnabled() && name == "root" {
		return authErr(http.StatusForbidden, "Cannot delete root user while auth is enabled.")
	}
	_, err := s.deleteResource("/users/" + name)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return authErr(http.StatusNotFound, "User %s does not exist", name)
			}
		}
		return err
	}
	plog.Noticef("deleted user %s", name)
	return nil
}

func (s *Store) UpdateUser(user User) (User, error) {
	old, err := s.GetUser(user.User)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return user, authErr(http.StatusNotFound, "User %s doesn't exist.", user.User)
			}
		}
		return old, err
	}
	newUser, err := old.Merge(user)
	if err != nil {
		return old, err
	}
	if reflect.DeepEqual(old, newUser) {
		if user.Revoke != nil || user.Grant != nil {
			return old, authErr(http.StatusConflict, "User not updated. Grant/Revoke lists didn't match any current roles.")
		}
		return old, authErr(http.StatusBadRequest, "User not updated. Use Grant/Revoke/Password to update the user.")
	}
	_, err = s.updateResource("/users/"+user.User, newUser)
	if err == nil {
		plog.Noticef("updated user %s", user.User)
	}
	return newUser, err
}

func (s *Store) AllRoles() ([]string, error) {
	nodes := []string{RootRoleName}
	resp, err := s.requestResource("/roles/", false)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return nodes, nil
			}
		}
		return nil, err
	}
	for _, n := range resp.Event.Node.Nodes {
		_, role := path.Split(n.Key)
		nodes = append(nodes, role)
	}
	sort.Strings(nodes)
	return nodes, nil
}

func (s *Store) GetRole(name string) (Role, error) {
	if name == RootRoleName {
		return rootRole, nil
	}
	resp, err := s.requestResource("/roles/"+name, false)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return Role{}, authErr(http.StatusNotFound, "Role %s does not exist.", name)
			}
		}
		return Role{}, err
	}
	var r Role
	err = json.Unmarshal([]byte(*resp.Event.Node.Value), &r)
	if err != nil {
		return r, err
	}

	return r, nil
}

func (s *Store) CreateOrUpdateRole(r Role) (role Role, created bool, err error) {
	_, err = s.GetRole(r.Role)
	if err == nil {
		role, err = s.UpdateRole(r)
		created = false
		return
	}
	return r, true, s.CreateRole(r)
}

func (s *Store) CreateRole(role Role) error {
	if role.Role == RootRoleName {
		return authErr(http.StatusForbidden, "Cannot modify role %s: is root role.", role.Role)
	}
	_, err := s.createResource("/roles/"+role.Role, role)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeNodeExist {
				return authErr(http.StatusConflict, "Role %s already exists.", role.Role)
			}
		}
	}
	if err == nil {
		plog.Noticef("created new role %s", role.Role)
	}
	return err
}

func (s *Store) DeleteRole(name string) error {
	if name == RootRoleName {
		return authErr(http.StatusForbidden, "Cannot modify role %s: is root role.", name)
	}
	_, err := s.deleteResource("/roles/" + name)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return authErr(http.StatusNotFound, "Role %s doesn't exist.", name)
			}
		}
	}
	if err == nil {
		plog.Noticef("deleted role %s", name)
	}
	return err
}

func (s *Store) UpdateRole(role Role) (Role, error) {
	old, err := s.GetRole(role.Role)
	if err != nil {
		if e, ok := err.(*etcderr.Error); ok {
			if e.ErrorCode == etcderr.EcodeKeyNotFound {
				return role, authErr(http.StatusNotFound, "Role %s doesn't exist.", role.Role)
			}
		}
		return old, err
	}
	newRole, err := old.Merge(role)
	if err != nil {
		return old, err
	}
	if reflect.DeepEqual(old, newRole) {
		if role.Revoke != nil || role.Grant != nil {
			return old, authErr(http.StatusConflict, "Role not updated. Grant/Revoke lists didn't match any current permissions.")
		}
		return old, authErr(http.StatusBadRequest, "Role not updated. Use Grant/Revoke to update the role.")
	}
	_, err = s.updateResource("/roles/"+role.Role, newRole)
	if err == nil {
		plog.Noticef("updated role %s", role.Role)
	}
	return newRole, err
}

func (s *Store) AuthEnabled() bool {
	return s.detectAuth()
}

func (s *Store) EnableAuth() error {
	if s.AuthEnabled() {
		return authErr(http.StatusConflict, "already enabled")
	}
	_, err := s.GetUser("root")
	if err != nil {
		return authErr(http.StatusConflict, "No root user available, please create one")
	}
	_, err = s.GetRole(GuestRoleName)
	if err != nil {
		plog.Printf("no guest role access found, creating default")
		err := s.CreateRole(guestRole)
		if err != nil {
			plog.Errorf("error creating guest role. aborting auth enable.")
			return err
		}
	}
	err = s.enableAuth()
	if err == nil {
		plog.Noticef("auth: enabled auth")
	} else {
		plog.Errorf("error enabling auth (%v)", err)
	}
	return err
}

func (s *Store) DisableAuth() error {
	if !s.AuthEnabled() {
		return authErr(http.StatusConflict, "already disabled")
	}
	err := s.disableAuth()
	if err == nil {
		plog.Noticef("auth: disabled auth")
	} else {
		plog.Errorf("error disabling auth (%v)", err)
	}
	return err
}

// Merge applies the properties of the passed-in User to the User on which it
// is called and returns a new User with these modifications applied. Think of
// all Users as immutable sets of data. Merge allows you to perform the set
// operations (desired grants and revokes) atomically
func (u User) Merge(n User) (User, error) {
	var out User
	if u.User != n.User {
		return out, authErr(http.StatusConflict, "Merging user data with conflicting usernames: %s %s", u.User, n.User)
	}
	out.User = u.User
	if n.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(n.Password), bcrypt.DefaultCost)
		if err != nil {
			return out, err
		}
		out.Password = string(hash)
	} else {
		out.Password = u.Password
	}
	currentRoles := types.NewUnsafeSet(u.Roles...)
	for _, g := range n.Grant {
		if currentRoles.Contains(g) {
			plog.Noticef("granting duplicate role %s for user %s", g, n.User)
			continue
		}
		currentRoles.Add(g)
	}
	for _, r := range n.Revoke {
		if !currentRoles.Contains(r) {
			plog.Noticef("revoking ungranted role %s for user %s", r, n.User)
			continue
		}
		currentRoles.Remove(r)
	}
	out.Roles = currentRoles.Values()
	return out, nil
}

func (u User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// Merge for a role works the same as User above -- atomic Role application to
// each of the substructures.
func (r Role) Merge(n Role) (Role, error) {
	var out Role
	var err error
	if r.Role != n.Role {
		return out, authErr(http.StatusConflict, "Merging role with conflicting names: %s %s", r.Role, n.Role)
	}
	out.Role = r.Role
	out.Permissions, err = r.Permissions.Grant(n.Grant)
	if err != nil {
		return out, err
	}
	out.Permissions, err = out.Permissions.Revoke(n.Revoke)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (r Role) HasKeyAccess(key string, write bool) bool {
	if r.Role == RootRoleName {
		return true
	}
	return r.Permissions.KV.HasAccess(key, write)
}

func (r Role) HasRecursiveAccess(key string, write bool) bool {
	if r.Role == RootRoleName {
		return true
	}
	return r.Permissions.KV.HasRecursiveAccess(key, write)
}

// Grant adds a set of permissions to the permission object on which it is called,
// returning a new permission object.
func (p Permissions) Grant(n *Permissions) (Permissions, error) {
	var out Permissions
	var err error
	if n == nil {
		return p, nil
	}
	out.KV, err = p.KV.Grant(n.KV)
	return out, err
}

// Revoke removes a set of permissions to the permission object on which it is called,
// returning a new permission object.
func (p Permissions) Revoke(n *Permissions) (Permissions, error) {
	var out Permissions
	var err error
	if n == nil {
		return p, nil
	}
	out.KV, err = p.KV.Revoke(n.KV)
	return out, err
}

// Grant adds a set of permissions to the permission object on which it is called,
// returning a new permission object.
func (rw rwPermission) Grant(n rwPermission) (rwPermission, error) {
	var out rwPermission
	currentRead := types.NewUnsafeSet(rw.Read...)
	for _, r := range n.Read {
		if currentRead.Contains(r) {
			return out, authErr(http.StatusConflict, "Granting duplicate read permission %s", r)
		}
		currentRead.Add(r)
	}
	currentWrite := types.NewUnsafeSet(rw.Write...)
	for _, w := range n.Write {
		if currentWrite.Contains(w) {
			return out, authErr(http.StatusConflict, "Granting duplicate write permission %s", w)
		}
		currentWrite.Add(w)
	}
	out.Read = currentRead.Values()
	out.Write = currentWrite.Values()
	return out, nil
}

// Revoke removes a set of permissions to the permission object on which it is called,
// returning a new permission object.
func (rw rwPermission) Revoke(n rwPermission) (rwPermission, error) {
	var out rwPermission
	currentRead := types.NewUnsafeSet(rw.Read...)
	for _, r := range n.Read {
		if !currentRead.Contains(r) {
			plog.Noticef("revoking ungranted read permission %s", r)
			continue
		}
		currentRead.Remove(r)
	}
	currentWrite := types.NewUnsafeSet(rw.Write...)
	for _, w := range n.Write {
		if !currentWrite.Contains(w) {
			plog.Noticef("revoking ungranted write permission %s", w)
			continue
		}
		currentWrite.Remove(w)
	}
	out.Read = currentRead.Values()
	out.Write = currentWrite.Values()
	return out, nil
}

func (rw rwPermission) HasAccess(key string, write bool) bool {
	var list []string
	if write {
		list = rw.Write
	} else {
		list = rw.Read
	}
	for _, pat := range list {
		match, err := simpleMatch(pat, key)
		if err == nil && match {
			return true
		}
	}
	return false
}

func (rw rwPermission) HasRecursiveAccess(key string, write bool) bool {
	list := rw.Read
	if write {
		list = rw.Write
	}
	for _, pat := range list {
		match, err := prefixMatch(pat, key)
		if err == nil && match {
			return true
		}
	}
	return false
}

func simpleMatch(pattern string, key string) (match bool, err error) {
	if pattern[len(pattern)-1] == '*' {
		return strings.HasPrefix(key, pattern[:len(pattern)-1]), nil
	}
	return key == pattern, nil
}

func prefixMatch(pattern string, key string) (match bool, err error) {
	if pattern[len(pattern)-1] != '*' {
		return false, nil
	}
	return strings.HasPrefix(key, pattern[:len(pattern)-1]), nil
}
