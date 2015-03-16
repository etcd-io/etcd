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
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/crypto/bcrypt"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/types"
)

const (
	// StorePermsPrefix is the internal prefix of the storage layer dedicated to storing user data.
	StorePermsPrefix = "/2"
)

type doer interface {
	Do(context.Context, etcdserverpb.Request) (etcdserver.Response, error)
}

type Store struct {
	server  doer
	timeout time.Duration
	enabled bool
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

type MergeError struct {
	errmsg string
}

func (m MergeError) Error() string { return m.errmsg }

func mergeErr(s string, v ...interface{}) MergeError {
	return MergeError{fmt.Sprintf(s, v...)}
}

func NewStore(server doer, timeout time.Duration) *Store {
	s := &Store{
		server:  server,
		timeout: timeout,
	}
	s.enabled = s.detectSecurity()
	return s
}

func (s *Store) AllUsers() ([]string, error) {
	resp, err := s.requestResource("/users/", false)
	if err != nil {
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
		return User{}, err
	}
	var u User
	err = json.Unmarshal([]byte(*resp.Event.Node.Value), &u)
	if err != nil {
		return u, err
	}

	return u, nil
}

func (s *Store) CreateOrUpdateUser(user User) (User, error) {
	_, err := s.GetUser(user.User)
	if err == nil {
		// Remove the update-user roles from updating downstream.
		// Roles are granted or revoked, not changed directly.
		user.Roles = nil
		return s.UpdateUser(user)
	}
	return user, s.CreateUser(user)
}

func (s *Store) CreateUser(user User) error {
	if user.User == "root" {
		return mergeErr("Cannot create root user; enable security to set root.")
	}
	return s.createUserInternal(user)
}

func (s *Store) createUserInternal(user User) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)

	_, err = s.createResource("/users/"+user.User, user)
	return err
}

func (s *Store) DeleteUser(name string) error {
	if name == "root" {
		return mergeErr("Can't delete root user; disable security instead.")
	}
	_, err := s.deleteResource("/users/" + name)
	return err
}

func (s *Store) UpdateUser(user User) (User, error) {
	old, err := s.GetUser(user.User)
	if err != nil {
		return old, err
	}
	newUser, err := old.Merge(user)
	if err != nil {
		return old, err
	}
	_, err = s.updateResource("/users/"+user.User, newUser)
	if err != nil {
		return newUser, err
	}
	return newUser, nil
}

func (s *Store) AllRoles() ([]string, error) {
	resp, err := s.requestResource("/roles/", false)
	if err != nil {
		return nil, err
	}
	var nodes []string
	for _, n := range resp.Event.Node.Nodes {
		_, role := path.Split(n.Key)
		nodes = append(nodes, role)
	}
	sort.Strings(nodes)
	return nodes, nil
}

func (s *Store) GetRole(name string) (Role, error) {
	// TODO(barakmich): Possibly add a cache
	resp, err := s.requestResource("/roles/"+name, false)
	if err != nil {
		return Role{}, err
	}
	var r Role
	err = json.Unmarshal([]byte(*resp.Event.Node.Value), &r)
	if err != nil {
		return r, err
	}

	return r, nil
}

func (s *Store) CreateOrUpdateRole(role Role) (Role, error) {
	_, err := s.GetRole(role.Role)
	if err == nil {
		return s.UpdateRole(role)
	}
	return role, s.CreateRole(role)
}

func (s *Store) CreateRole(role Role) error {
	_, err := s.createResource("/roles/"+role.Role, role)
	return err
}

func (s *Store) DeleteRole(name string) error {
	_, err := s.deleteResource("/roles/" + name)
	return err
}

func (s *Store) UpdateRole(role Role) (Role, error) {
	old, err := s.GetRole(role.Role)
	if err != nil {
		return old, err
	}
	newRole, err := old.Merge(role)
	if err != nil {
		return old, err
	}
	_, err = s.updateResource("/roles/"+role.Role, newRole)
	if err != nil {
		return newRole, err
	}
	return newRole, nil
}

func (s *Store) SecurityEnabled() bool {
	return s.enabled
}

func (s *Store) EnableSecurity(rootUser User) error {
	err := s.ensureSecurityDirectories()
	if err != nil {
		return err
	}
	if rootUser.User != "root" {
		mergeErr("Trying to create root user not named root")
	}
	err = s.createUserInternal(rootUser)
	if err == nil {
		s.enabled = true
	}
	return err
}

func (s *Store) DisableSecurity() error {
	err := s.DeleteUser("root")
	if err == nil {
		s.enabled = false
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
		return out, mergeErr("Merging user data with conflicting usernames: %s %s", u.User, n.User)
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
			return out, mergeErr("Granting duplicate role %s for user %s", g, n.User)
		}
		currentRoles.Add(g)
	}
	for _, r := range n.Revoke {
		if !currentRoles.Contains(r) {
			return out, mergeErr("Revoking ungranted role %s for user %s", r, n.User)
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
		return out, mergeErr("Merging role with conflicting names: %s %s", r.Role, n.Role)
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
	return r.Permissions.KV.HasAccess(key, write)
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
			return out, mergeErr("Granting duplicate read permission %s", r)
		}
		currentRead.Add(r)
	}
	currentWrite := types.NewUnsafeSet(rw.Write...)
	for _, w := range n.Write {
		if currentWrite.Contains(w) {
			return out, mergeErr("Granting duplicate write permission %s", w)
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
			return out, mergeErr("Revoking ungranted read permission %s", r)
		}
		currentRead.Remove(r)
	}
	currentWrite := types.NewUnsafeSet(rw.Write...)
	for _, w := range n.Write {
		if !currentWrite.Contains(w) {
			return out, mergeErr("Revoking ungranted write permission %s", w)
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
		match, err := path.Match(pat, key)
		if err == nil && match {
			return true
		}
	}
	return false
}
