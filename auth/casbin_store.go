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
	"errors"
	"strings"

	"github.com/casbin/casbin"
	"github.com/coreos/etcd/auth/authpb"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/backend"
	"golang.org/x/net/context"
)

var (
	casbinAuthBucketName = []byte("casbinAuth")

	ErrPermissionAlreadyExist = errors.New("auth: permission already exists")
)

type casbinAuthStore struct {
	s *authStore

	enforcer *casbin.Enforcer
}

func (as *casbinAuthStore) AuthEnable() error {
	as.s.enabledMu.Lock()
	defer as.s.enabledMu.Unlock()
	if as.s.enabled {
		plog.Noticef("Authentication already enabled")
		return nil
	}
	b := as.s.be
	tx := b.BatchTx()
	tx.Lock()
	defer func() {
		tx.Unlock()
		b.ForceCommit()
	}()

	u := getUser(tx, rootUser)
	if u == nil {
		return ErrRootUserNotExist
	}

	tx.UnsafePut(authBucketName, enableFlagKey, authEnabled)

	as.s.enabled = true
	as.s.tokenProvider.enable()

	as.s.rangePermCache = make(map[string]*unifiedRangePermissions)

	as.s.setRevision(getRevision(tx))

	plog.Noticef("Authentication enabled")

	return nil
}

func (as *casbinAuthStore) AuthDisable() {
	as.s.AuthDisable()
}

func (as *casbinAuthStore) Close() error {
	return as.s.Close()
}

func (as *casbinAuthStore) Authenticate(ctx context.Context, username, password string) (*pb.AuthenticateResponse, error) {
	return as.s.Authenticate(ctx, username, password)
}

func (as *casbinAuthStore) CheckPassword(username, password string) (uint64, error) {
	return as.s.CheckPassword(username, password)
}

func (as *casbinAuthStore) Recover(be backend.Backend) {
	as.s.Recover(be)
}

func (as *casbinAuthStore) UserList(r *pb.AuthUserListRequest) (*pb.AuthUserListResponse, error) {
	return as.s.UserList(r)
}

func (as *casbinAuthStore) UserAdd(r *pb.AuthUserAddRequest) (*pb.AuthUserAddResponse, error) {
	return as.s.UserAdd(r)
}

func (as *casbinAuthStore) UserDelete(r *pb.AuthUserDeleteRequest) (*pb.AuthUserDeleteResponse, error) {
	response, err := as.s.UserDelete(r)
	if err == nil {
		as.enforcer.DeleteUser(r.Name)
	}
	return response, err
}

func (as *casbinAuthStore) UserChangePassword(r *pb.AuthUserChangePasswordRequest) (*pb.AuthUserChangePasswordResponse, error) {
	return as.s.UserChangePassword(r)
}

func (as *casbinAuthStore) RoleList(r *pb.AuthRoleListRequest) (*pb.AuthRoleListResponse, error) {
	return as.s.RoleList(r)
}

func (as *casbinAuthStore) RoleAdd(r *pb.AuthRoleAddRequest) (*pb.AuthRoleAddResponse, error) {
	return as.s.RoleAdd(r)
}

func (as *casbinAuthStore) RoleDelete(r *pb.AuthRoleDeleteRequest) (*pb.AuthRoleDeleteResponse, error) {
	response, err := as.s.RoleDelete(r)
	if err == nil {
		as.enforcer.DeleteRole(r.Role)
	}
	return response, err
}

func (as *casbinAuthStore) GenTokenPrefix() (string, error) {
	return as.s.GenTokenPrefix()
}

func (as *casbinAuthStore) Revision() uint64 {
	return as.s.Revision()
}

func (as *casbinAuthStore) AuthInfoFromCtx(ctx context.Context) (*AuthInfo, error) {
	return as.s.AuthInfoFromCtx(ctx)
}

func (as *casbinAuthStore) AuthInfoFromTLS(ctx context.Context) *AuthInfo {
	return as.s.AuthInfoFromTLS(ctx)
}

func (as *casbinAuthStore) WithRoot(ctx context.Context) context.Context {
	return as.s.WithRoot(ctx)
}

func (as *casbinAuthStore) HasRole(user, role string) bool {
	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	u := getUser(tx, user)
	if u == nil {
		plog.Warningf("tried to check user %s has role %s, but user %s doesn't exist", user, role, user)
		return false
	}

	return as.enforcer.HasRoleForUser(user, role)
}

func (as *casbinAuthStore) UserGrantRole(r *pb.AuthUserGrantRoleRequest) (*pb.AuthUserGrantRoleResponse, error) {
	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	as.enforcer.AddRoleForUser(r.User, r.Role)
	as.enforcer.SavePolicy()

	plog.Noticef("granted role %s to user %s", r.Role, r.User)
	return &pb.AuthUserGrantRoleResponse{}, nil
}

func (as *casbinAuthStore) UserGet(r *pb.AuthUserGetRequest) (*pb.AuthUserGetResponse, error) {
	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	var resp pb.AuthUserGetResponse

	user := getUser(tx, r.Name)
	if user == nil {
		return nil, ErrUserNotFound
	}

	resp.Roles = as.enforcer.GetRolesForUser(r.Name)
	return &resp, nil
}

func (as *casbinAuthStore) UserRevokeRole(r *pb.AuthUserRevokeRoleRequest) (*pb.AuthUserRevokeRoleResponse, error) {
	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	user := getUser(tx, r.Name)
	if user == nil {
		return nil, ErrUserNotFound
	}

	if !as.enforcer.HasRoleForUser(r.Name, r.Role) {
		return nil, ErrRoleNotGranted
	}

	as.enforcer.DeleteRoleForUser(r.Name, r.Role)
	as.enforcer.SavePolicy()

	plog.Noticef("revoked role %s from user %s", r.Role, r.Name)
	return &pb.AuthUserRevokeRoleResponse{}, nil
}

func (as *casbinAuthStore) RoleGet(r *pb.AuthRoleGetRequest) (*pb.AuthRoleGetResponse, error) {
	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	var resp pb.AuthRoleGetResponse

	role := getRole(tx, r.Role)
	if role == nil {
		return nil, ErrRoleNotFound
	}

	permissions := as.enforcer.GetPermissionsForUser(r.Role)

	for _, permission := range permissions {
		key := permission[1]
		rangeEnd := permission[2]
		permType := permission[3]
		resp.Perm = append(resp.Perm, &authpb.Permission{authpb.Permission_Type(authpb.Permission_Type_value[strings.ToUpper(permType)]), []byte(key), []byte(rangeEnd)})
	}

	return &resp, nil
}

func (as *casbinAuthStore) RoleRevokePermission(r *pb.AuthRoleRevokePermissionRequest) (*pb.AuthRoleRevokePermissionResponse, error) {
	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	role := getRole(tx, r.Role)
	if role == nil {
		return nil, ErrRoleNotFound
	}

	if !as.enforcer.HasPermissionForUser(r.Role, r.Key, r.RangeEnd) {
		return nil, ErrPermissionNotGranted
	}

	as.enforcer.DeletePermissionForUser(r.Role, r.Key, r.RangeEnd)
	as.enforcer.SavePolicy()

	plog.Noticef("revoked key %s from role %s", r.Key, r.Role)
	return &pb.AuthRoleRevokePermissionResponse{}, nil
}

func (as *casbinAuthStore) RoleGrantPermission(r *pb.AuthRoleGrantPermissionRequest) (*pb.AuthRoleGrantPermissionResponse, error) {
	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	role := getRole(tx, r.Name)
	if role == nil {
		return nil, ErrRoleNotFound
	}

	if as.enforcer.HasPermissionForUser(r.Name, string(r.Perm.Key), string(r.Perm.RangeEnd), authpb.Permission_Type_name[int32(r.Perm.PermType)]) {
		return nil, ErrPermissionAlreadyExist
	}

	as.enforcer.AddPermissionForUser(r.Name, string(r.Perm.Key), string(r.Perm.RangeEnd), authpb.Permission_Type_name[int32(r.Perm.PermType)])
	as.enforcer.SavePolicy()

	plog.Noticef("role %s's permission of key %s is updated as %s", r.Name, r.Perm.Key, authpb.Permission_Type_name[int32(r.Perm.PermType)])
	return &pb.AuthRoleGrantPermissionResponse{}, nil
}

func (as *casbinAuthStore) isOpPermitted(userName string, revision uint64, key, rangeEnd []byte, permTyp authpb.Permission_Type) error {
	if !as.s.isAuthEnabled() {
		return nil
	}

	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	user := getUser(tx, userName)
	if user == nil {
		plog.Errorf("invalid user name %s for permission checking", userName)
		return ErrPermissionDenied
	}

	if as.enforcer.Enforce(userName, string(key), string(rangeEnd), authpb.Permission_Type_name[int32(permTyp)]) {
		return nil
	}

	return ErrPermissionDenied
}

func (as *casbinAuthStore) IsPutPermitted(authInfo *AuthInfo, key []byte) error {
	return as.isOpPermitted(authInfo.Username, authInfo.Revision, key, nil, authpb.WRITE)
}

func (as *casbinAuthStore) IsRangePermitted(authInfo *AuthInfo, key, rangeEnd []byte) error {
	return as.isOpPermitted(authInfo.Username, authInfo.Revision, key, rangeEnd, authpb.READ)
}

func (as *casbinAuthStore) IsDeleteRangePermitted(authInfo *AuthInfo, key, rangeEnd []byte) error {
	return as.isOpPermitted(authInfo.Username, authInfo.Revision, key, rangeEnd, authpb.WRITE)
}

func (as *casbinAuthStore) IsAdminPermitted(authInfo *AuthInfo) error {
	if !as.s.isAuthEnabled() {
		return nil
	}
	if authInfo == nil {
		return ErrUserEmpty
	}

	tx := as.s.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	u := getUser(tx, authInfo.Username)
	if u == nil {
		return ErrUserNotFound
	}

	if !as.hasRootRole(u) {
		return ErrPermissionDenied
	}

	return nil
}

func NewCasbinAuthStore(be backend.Backend, tp TokenProvider) *casbinAuthStore {
	tx := be.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(casbinAuthBucketName)

	m := casbin.NewModel()
	m.AddDef("r", "r", "user, key, rangeEnd, permType")
	m.AddDef("p", "p", "user, key, rangeEnd, permType")
	m.AddDef("g", "g", "_, _")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "g(r.user, \"root\") || (g(r.user, p.user) && keyMatch(r.key, p.key) && (r.permType == p.permType || p.permType == \"*\"))")

	a := NewCasbinBackend(casbinAuthBucketName, tx)

	e := casbin.NewEnforcer(m, a, false)

	e.SavePolicy()

	tx.Unlock()
	be.ForceCommit()

	s := NewAuthStore(be, tp)

	as := &casbinAuthStore{
		enforcer: e,
		s:        s,
	}

	return as
}

func (as *casbinAuthStore) hasRootRole(u *authpb.User) bool {
	return as.enforcer.HasRoleForUser(string(u.Name), "root")
}
