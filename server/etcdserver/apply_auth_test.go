// Copyright 2023 The etcd Authors
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

package etcdserver

import (
	"encoding/base64"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	betesting "go.etcd.io/etcd/server/v3/mvcc/backend/testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/authpb"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/lease"
)

func TestCheckLeasePutsKeys(t *testing.T) {
	lg := zaptest.NewLogger(t)

	b, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, b)

	simpleTokenTTLDefault := 300 * time.Second
	tokenTypeSimple := "simple"
	dummyIndexWaiter := func(index uint64) <-chan struct{} {
		ch := make(chan struct{}, 1)
		go func() {
			ch <- struct{}{}
		}()
		return ch
	}

	tp, _ := auth.NewTokenProvider(zaptest.NewLogger(t), tokenTypeSimple, dummyIndexWaiter, simpleTokenTTLDefault)
	as := auth.NewAuthStore(lg, b, tp, bcrypt.MinCost)
	defer as.AuthDisable()

	aa := authApplierV3{as: as}
	assert.NoError(t, aa.checkLeasePutsKeys(lease.NewLease(lease.LeaseID(1), 3600)), "auth is disabled, should allow puts")
	assert.NoError(t, enableAuthAndCreateRoot(aa.as), "error while enabling auth")
	aa.authInfo = auth.AuthInfo{Username: "root"}
	assert.NoError(t, aa.checkLeasePutsKeys(lease.NewLease(lease.LeaseID(1), 3600)), "auth is enabled, should allow puts for root")

	l := lease.NewLease(lease.LeaseID(1), 3600)
	l.SetLeaseItem(lease.LeaseItem{Key: "a"})
	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: 0}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrUserEmpty, "auth is enabled, should not allow bob, non existing at rev 0")
	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: 1}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrAuthOldRevision, "auth is enabled, old revision")

	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: aa.as.Revision()}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrPermissionDenied, "auth is enabled, bob does not have permissions, bob does not exist")
	_, err := aa.as.UserAdd(&pb.AuthUserAddRequest{Name: "bob", Options: &authpb.UserAddOptions{NoPassword: true}})
	assert.NoError(t, err, "bob should be added without error")
	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: aa.as.Revision()}
	assert.ErrorIs(t, aa.checkLeasePutsKeys(l), auth.ErrPermissionDenied, "auth is enabled, bob exists yet does not have permissions")

	// allow bob to access "a"
	_, err = aa.as.RoleAdd(&pb.AuthRoleAddRequest{Name: "bobsrole"})
	assert.NoError(t, err, "bobsrole should be added without error")
	_, err = aa.as.RoleGrantPermission(&pb.AuthRoleGrantPermissionRequest{
		Name: "bobsrole",
		Perm: &authpb.Permission{
			PermType: authpb.READWRITE,
			Key:      []byte("a"),
			RangeEnd: nil,
		},
	})
	assert.NoError(t, err, "bobsrole should be granted permissions without error")
	_, err = aa.as.UserGrantRole(&pb.AuthUserGrantRoleRequest{
		User: "bob",
		Role: "bobsrole",
	})
	assert.NoError(t, err, "bob should be granted bobsrole without error")

	aa.authInfo = auth.AuthInfo{Username: "bob", Revision: aa.as.Revision()}
	assert.NoError(t, aa.checkLeasePutsKeys(l), "bob should be able to access key 'a'")

}

func enableAuthAndCreateRoot(as auth.AuthStore) error {
	_, err := as.UserAdd(&pb.AuthUserAddRequest{
		Name:           "root",
		HashedPassword: encodePassword("root"),
		Options:        &authpb.UserAddOptions{NoPassword: false}})
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

func encodePassword(s string) string {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(s), bcrypt.MinCost)
	return base64.StdEncoding.EncodeToString(hashedPassword)
}
