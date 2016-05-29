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
	"bytes"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/coreos/etcd/auth/authpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/backend"
	"github.com/coreos/pkg/capnslog"
	"golang.org/x/crypto/bcrypt"
)

var (
	enableFlagKey = []byte("authEnabled")

	authBucketName      = []byte("auth")
	authUsersBucketName = []byte("authUsers")
	authRolesBucketName = []byte("authRoles")

	plog = capnslog.NewPackageLogger("github.com/coreos/etcd", "auth")

	ErrUserAlreadyExist = errors.New("auth: user already exists")
	ErrUserNotFound     = errors.New("auth: user not found")
	ErrRoleAlreadyExist = errors.New("auth: role already exists")
	ErrRoleNotFound     = errors.New("auth: role not found")
	ErrAuthFailed       = errors.New("auth: authentication failed, invalid user ID or password")
)

type AuthStore interface {
	// AuthEnable turns on the authentication feature
	AuthEnable()

	// AuthDisable turns off the authentication feature
	AuthDisable()

	// Authenticate does authentication based on given user name and password,
	// and returns a token for successful case.
	// Note that the generated token is valid only for the member the client
	// connected to within fixed time duration. Reauth is required after the duration.
	Authenticate(name string, password string) (*pb.AuthenticateResponse, error)

	// Recover recovers the state of auth store from the given backend
	Recover(b backend.Backend)

	// UserAdd adds a new user
	UserAdd(r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error)

	// UserDelete deletes a user
	UserDelete(r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error)

	// UserChangePassword changes a password of a user
	UserChangePassword(r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error)

	// UserGrant grants a role to the user
	UserGrant(r *pb.AuthUserGrantRequest) (*pb.AuthUserGrantResponse, error)

	// RoleAdd adds a new role
	RoleAdd(r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error)

	// RoleGrant grants a permission to a role
	RoleGrant(r *pb.AuthRoleGrantRequest) (*pb.AuthRoleGrantResponse, error)

	// UsernameFromToken gets a username from the given Token
	UsernameFromToken(token string) (string, bool)

	// IsPutPermitted checks put permission of the user
	IsPutPermitted(header *pb.RequestHeader, key string) bool

	// IsRangePermitted checks range permission of the user
	IsRangePermitted(header *pb.RequestHeader, key string) bool
}

type authStore struct {
	be        backend.Backend
	enabled   bool
	enabledMu sync.RWMutex
}

func (as *authStore) AuthEnable() {
	value := []byte{1}

	b := as.be
	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafePut(authBucketName, enableFlagKey, value)
	tx.Unlock()
	b.ForceCommit()

	as.enabledMu.Lock()
	as.enabled = true
	as.enabledMu.Unlock()

	plog.Noticef("Authentication enabled")
}

func (as *authStore) AuthDisable() {
	value := []byte{0}

	b := as.be
	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafePut(authBucketName, enableFlagKey, value)
	tx.Unlock()
	b.ForceCommit()

	as.enabledMu.Lock()
	as.enabled = false
	as.enabledMu.Unlock()

	plog.Noticef("Authentication disabled")
}

func (as *authStore) Authenticate(name string, password string) (*pb.AuthenticateResponse, error) {
	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authUsersBucketName, []byte(name), nil, 0)
	if len(vs) != 1 {
		plog.Noticef("authentication failed, user %s doesn't exist", name)
		return &pb.AuthenticateResponse{}, ErrAuthFailed
	}

	user := &authpb.User{}
	err := user.Unmarshal(vs[0])
	if err != nil {
		return nil, err
	}

	if bcrypt.CompareHashAndPassword(user.Password, []byte(password)) != nil {
		plog.Noticef("authentication failed, invalid password for user %s", name)
		return &pb.AuthenticateResponse{}, ErrAuthFailed
	}

	token, err := genSimpleTokenForUser(name)
	if err != nil {
		plog.Errorf("failed to generate simple token: %s", err)
		return nil, err
	}

	plog.Infof("authorized %s, token is %s", name, token)
	return &pb.AuthenticateResponse{Token: token}, nil
}

func (as *authStore) Recover(be backend.Backend) {
	as.be = be
	// TODO(mitake): recovery process
}

func (as *authStore) UserAdd(r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(r.Password), bcrypt.DefaultCost)
	if err != nil {
		plog.Errorf("failed to hash password: %s", err)
		return nil, err
	}

	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authUsersBucketName, []byte(r.Name), nil, 0)
	if len(vs) != 0 {
		return &pb.AuthUserAddResponse{}, ErrUserAlreadyExist
	}

	newUser := authpb.User{
		Name:     []byte(r.Name),
		Password: hashed,
	}

	marshaledUser, merr := newUser.Marshal()
	if merr != nil {
		plog.Errorf("failed to marshal a new user data: %s", merr)
		return nil, merr
	}

	tx.UnsafePut(authUsersBucketName, []byte(r.Name), marshaledUser)

	plog.Noticef("added a new user: %s", r.Name)

	return &pb.AuthUserAddResponse{}, nil
}

func (as *authStore) UserDelete(r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error) {
	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authUsersBucketName, []byte(r.Name), nil, 0)
	if len(vs) != 1 {
		return &pb.AuthUserDeleteResponse{}, ErrUserNotFound
	}

	tx.UnsafeDelete(authUsersBucketName, []byte(r.Name))

	plog.Noticef("deleted a user: %s", r.Name)

	return &pb.AuthUserDeleteResponse{}, nil
}

func (as *authStore) UserChangePassword(r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error) {
	// TODO(mitake): measure the cost of bcrypt.GenerateFromPassword()
	// If the cost is too high, we should move the encryption to outside of the raft
	hashed, err := bcrypt.GenerateFromPassword([]byte(r.Password), bcrypt.DefaultCost)
	if err != nil {
		plog.Errorf("failed to hash password: %s", err)
		return nil, err
	}

	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authUsersBucketName, []byte(r.Name), nil, 0)
	if len(vs) != 1 {
		return &pb.AuthUserChangePasswordResponse{}, ErrUserNotFound
	}

	updatedUser := authpb.User{
		Name:     []byte(r.Name),
		Password: hashed,
	}

	marshaledUser, merr := updatedUser.Marshal()
	if merr != nil {
		plog.Errorf("failed to marshal a new user data: %s", merr)
		return nil, merr
	}

	tx.UnsafePut(authUsersBucketName, []byte(r.Name), marshaledUser)

	plog.Noticef("changed a password of a user: %s", r.Name)

	return &pb.AuthUserChangePasswordResponse{}, nil
}

func (as *authStore) UserGrant(r *pb.AuthUserGrantRequest) (*pb.AuthUserGrantResponse, error) {
	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authUsersBucketName, []byte(r.User), nil, 0)
	if len(vs) != 1 {
		return nil, ErrUserNotFound
	}

	user := &authpb.User{}
	err := user.Unmarshal(vs[0])
	if err != nil {
		return nil, err
	}

	_, vs = tx.UnsafeRange(authRolesBucketName, []byte(r.Role), nil, 0)
	if len(vs) != 1 {
		return nil, ErrRoleNotFound
	}

	idx := sort.SearchStrings(user.Roles, r.Role)
	if idx < len(user.Roles) && strings.Compare(user.Roles[idx], r.Role) == 0 {
		plog.Warningf("user %s is already granted role %s", r.User, r.Role)
		return &pb.AuthUserGrantResponse{}, nil
	}

	user.Roles = append(user.Roles, r.Role)
	sort.Sort(sort.StringSlice(user.Roles))

	marshaledUser, merr := user.Marshal()
	if merr != nil {
		return nil, merr
	}

	tx.UnsafePut(authUsersBucketName, user.Name, marshaledUser)

	plog.Noticef("granted role %s to user %s", r.Role, r.User)
	return &pb.AuthUserGrantResponse{}, nil
}

func (as *authStore) RoleAdd(r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error) {
	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authRolesBucketName, []byte(r.Name), nil, 0)
	if len(vs) != 0 {
		return nil, ErrRoleAlreadyExist
	}

	newRole := &authpb.Role{
		Name: []byte(r.Name),
	}

	marshaledRole, err := newRole.Marshal()
	if err != nil {
		return nil, err
	}

	tx.UnsafePut(authRolesBucketName, []byte(r.Name), marshaledRole)

	plog.Noticef("Role %s is created", r.Name)

	return &pb.AuthRoleAddResponse{}, nil
}

func (as *authStore) UsernameFromToken(token string) (string, bool) {
	simpleTokensMu.RLock()
	defer simpleTokensMu.RUnlock()
	t, ok := simpleTokens[token]
	return t, ok
}

type permSlice []*authpb.Permission

func (perms permSlice) Len() int {
	return len(perms)
}

func (perms permSlice) Less(i, j int) bool {
	return bytes.Compare(perms[i].Key, perms[j].Key) < 0
}

func (perms permSlice) Swap(i, j int) {
	perms[i], perms[j] = perms[j], perms[i]
}

func (as *authStore) RoleGrant(r *pb.AuthRoleGrantRequest) (*pb.AuthRoleGrantResponse, error) {
	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authRolesBucketName, []byte(r.Name), nil, 0)
	if len(vs) != 1 {
		return nil, ErrRoleNotFound
	}

	role := &authpb.Role{}
	err := role.Unmarshal(vs[0])
	if err != nil {
		plog.Errorf("failed to unmarshal a role %s: %s", r.Name, err)
		return nil, err
	}

	idx := sort.Search(len(role.KeyPermission), func(i int) bool {
		return bytes.Compare(role.KeyPermission[i].Key, []byte(r.Perm.Key)) >= 0
	})

	if idx < len(role.KeyPermission) && bytes.Equal(role.KeyPermission[idx].Key, r.Perm.Key) {
		// update existing permission
		role.KeyPermission[idx].PermType = r.Perm.PermType
	} else {
		// append new permission to the role
		newPerm := &authpb.Permission{
			Key:      []byte(r.Perm.Key),
			PermType: r.Perm.PermType,
		}

		role.KeyPermission = append(role.KeyPermission, newPerm)
		sort.Sort(permSlice(role.KeyPermission))
	}

	marshaledRole, merr := role.Marshal()
	if merr != nil {
		plog.Errorf("failed to marshal updated role %s: %s", r.Name, merr)
		return nil, merr
	}

	tx.UnsafePut(authRolesBucketName, []byte(r.Name), marshaledRole)

	plog.Noticef("role %s's permission of key %s is updated as %s", r.Name, r.Perm.Key, authpb.Permission_Type_name[int32(r.Perm.PermType)])

	return &pb.AuthRoleGrantResponse{}, nil
}

func (as *authStore) isOpPermitted(userName string, key string, write bool, read bool) bool {
	// TODO(mitake): this function would be costly so we need a caching mechanism
	if !as.isAuthEnabled() {
		return true
	}

	tx := as.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	_, vs := tx.UnsafeRange(authUsersBucketName, []byte(userName), nil, 0)
	if len(vs) != 1 {
		plog.Errorf("invalid user name %s for permission checking", userName)
		return false
	}

	user := &authpb.User{}
	err := user.Unmarshal(vs[0])
	if err != nil {
		plog.Errorf("failed to unmarshal user struct (name: %s): %s", userName, err)
		return false
	}

	for _, roleName := range user.Roles {
		_, vs := tx.UnsafeRange(authRolesBucketName, []byte(roleName), nil, 0)
		if len(vs) != 1 {
			plog.Errorf("invalid role name %s for permission checking", roleName)
			return false
		}

		role := &authpb.Role{}
		err := role.Unmarshal(vs[0])
		if err != nil {
			plog.Errorf("failed to unmarshal a role %s: %s", roleName, err)
			return false
		}

		for _, perm := range role.KeyPermission {
			if bytes.Equal(perm.Key, []byte(key)) {
				if perm.PermType == authpb.READWRITE {
					return true
				}

				if write && !read && perm.PermType == authpb.WRITE {
					return true
				}

				if read && !write && perm.PermType == authpb.READ {
					return true
				}
			}
		}
	}

	return false
}

func (as *authStore) IsPutPermitted(header *pb.RequestHeader, key string) bool {
	return as.isOpPermitted(header.Username, key, true, false)
}

func (as *authStore) IsRangePermitted(header *pb.RequestHeader, key string) bool {
	return as.isOpPermitted(header.Username, key, false, true)
}

func (as *authStore) isAuthEnabled() bool {
	as.enabledMu.RLock()
	defer as.enabledMu.RUnlock()
	return as.enabled
}

func NewAuthStore(be backend.Backend) *authStore {
	tx := be.BatchTx()
	tx.Lock()

	tx.UnsafeCreateBucket(authBucketName)
	tx.UnsafeCreateBucket(authUsersBucketName)
	tx.UnsafeCreateBucket(authRolesBucketName)

	tx.Unlock()
	be.ForceCommit()

	return &authStore{
		be: be,
	}
}
