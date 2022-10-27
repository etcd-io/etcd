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

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3AuthMemberAdd(t *testing.T)    { testCtl(t, authTestMemberAdd) }
func TestCtlV3AuthMemberUpdate(t *testing.T) { testCtl(t, authTestMemberUpdate) }

func TestCtlV3AuthFromKeyPerm(t *testing.T) { testCtl(t, authTestFromKeyPerm) }

func TestCtlV3AuthSnapshot(t *testing.T) { testCtl(t, authTestSnapshot) }
func TestCtlV3AuthSnapshotJWT(t *testing.T) {
	testCtl(t, authTestSnapshot, withCfg(*e2e.NewConfigJWT()))
}

func TestCtlV3AuthTestCacheReload(t *testing.T) { testCtl(t, authTestCacheReload) }

func authEnable(cx ctlCtx) error {
	// create root user with root role
	if err := ctlV3User(cx, []string{"add", "root", "--interactive=false"}, "User root created", []string{"root"}); err != nil {
		return fmt.Errorf("failed to create root user %v", err)
	}
	if err := ctlV3User(cx, []string{"grant-role", "root", "root"}, "Role root is granted to user root", nil); err != nil {
		return fmt.Errorf("failed to grant root user root role %v", err)
	}
	if err := ctlV3AuthEnable(cx); err != nil {
		return fmt.Errorf("authEnableTest ctlV3AuthEnable error (%v)", err)
	}
	return nil
}

func ctlV3AuthEnable(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "auth", "enable")
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, "Authentication Enabled")
}

func ctlV3PutFailPerm(cx ctlCtx, key, val string) error {
	return e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "put", key, val), cx.envMap, "permission denied")
}

func authSetupTestUser(cx ctlCtx) {
	if err := ctlV3User(cx, []string{"add", "test-user", "--interactive=false"}, "User test-user created", []string{"pass"}); err != nil {
		cx.t.Fatal(err)
	}
	if err := e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "role", "add", "test-role"), cx.envMap, "Role test-role created"); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3User(cx, []string{"grant-role", "test-user", "test-role"}, "Role test-role is granted to user test-user", nil); err != nil {
		cx.t.Fatal(err)
	}
	cmd := append(cx.PrefixArgs(), "role", "grant-permission", "test-role", "readwrite", "foo")
	if err := e2e.SpawnWithExpectWithEnv(cmd, cx.envMap, "Role test-role updated"); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestMemberAdd(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}
	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	// ordinary user cannot add a new member
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3MemberAdd(cx, peerURL, false); err == nil {
		cx.t.Fatalf("ordinary user must not be allowed to add a member")
	}

	// root can add a new member
	cx.user, cx.pass = "root", "root"
	if err := ctlV3MemberAdd(cx, peerURL, false); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestMemberUpdate(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	mr, err := getMemberList(cx)
	if err != nil {
		cx.t.Fatal(err)
	}

	// ordinary user cannot update a member
	cx.user, cx.pass = "test-user", "pass"
	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	memberID := fmt.Sprintf("%x", mr.Members[0].ID)
	if err = ctlV3MemberUpdate(cx, memberID, peerURL); err == nil {
		cx.t.Fatalf("ordinary user must not be allowed to update a member")
	}

	// root can update a member
	cx.user, cx.pass = "root", "root"
	if err = ctlV3MemberUpdate(cx, memberID, peerURL); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestCertCN(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	if err := ctlV3User(cx, []string{"add", "example.com", "--interactive=false"}, "User example.com created", []string{""}); err != nil {
		cx.t.Fatal(err)
	}
	if err := e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "role", "add", "test-role"), cx.envMap, "Role test-role created"); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3User(cx, []string{"grant-role", "example.com", "test-role"}, "Role test-role is granted to user example.com", nil); err != nil {
		cx.t.Fatal(err)
	}

	// grant a new key
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "hoo", "", false}); err != nil {
		cx.t.Fatal(err)
	}

	// try a granted key
	cx.user, cx.pass = "", ""
	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Error(err)
	}

	// try a non-granted key
	cx.user, cx.pass = "", ""
	err := ctlV3PutFailPerm(cx, "baz", "bar")
	require.ErrorContains(cx.t, err, "permission denied")
}

func authTestFromKeyPerm(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// grant keys after z to test-user
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "z", "\x00", false}); err != nil {
		cx.t.Fatal(err)
	}

	// try the granted open ended permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		if err := ctlV3Put(cx, key, "val", ""); err != nil {
			cx.t.Fatal(err)
		}
	}
	largeKey := ""
	for i := 0; i < 10; i++ {
		largeKey += "\xff"
		if err := ctlV3Put(cx, largeKey, "val", ""); err != nil {
			cx.t.Fatal(err)
		}
	}

	// try a non granted key
	err := ctlV3PutFailPerm(cx, "x", "baz")
	require.ErrorContains(cx.t, err, "permission denied")

	// revoke the open ended permission
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleRevokePermission(cx, "test-role", "z", "", true); err != nil {
		cx.t.Fatal(err)
	}

	// try the revoked open ended permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		err := ctlV3PutFailPerm(cx, key, "val")
		require.ErrorContains(cx.t, err, "permission denied")
	}

	// grant the entire keys
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "", "\x00", false}); err != nil {
		cx.t.Fatal(err)
	}

	// try keys, of course it must be allowed because test-role has a permission of the entire keys
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		if err := ctlV3Put(cx, key, "val", ""); err != nil {
			cx.t.Fatal(err)
		}
	}

	// revoke the entire keys
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleRevokePermission(cx, "test-role", "", "", true); err != nil {
		cx.t.Fatal(err)
	}

	// try the revoked entire key permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		err := ctlV3PutFailPerm(cx, key, "val")
		require.ErrorContains(cx.t, err, "permission denied")
	}
}

func authTestSnapshot(cx ctlCtx) {
	maintenanceInitKeys(cx)

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	fpath := "test-auth.snapshot"
	defer os.RemoveAll(fpath)

	// ordinary user cannot save a snapshot
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3SnapshotSave(cx, fpath); err == nil {
		cx.t.Fatal("ordinary user should not be able to save a snapshot")
	}

	// root can save a snapshot
	cx.user, cx.pass = "root", "root"
	if err := ctlV3SnapshotSave(cx, fpath); err != nil {
		cx.t.Fatalf("snapshotTest ctlV3SnapshotSave error (%v)", err)
	}

	st, err := getSnapshotStatus(cx, fpath)
	if err != nil {
		cx.t.Fatalf("snapshotTest getSnapshotStatus error (%v)", err)
	}
	if st.Revision != 4 {
		cx.t.Fatalf("expected 4, got %d", st.Revision)
	}
	if st.TotalKey < 3 {
		cx.t.Fatalf("expected at least 3, got %d", st.TotalKey)
	}
}

func certCNAndUsername(cx ctlCtx, noPassword bool) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	if noPassword {
		if err := ctlV3User(cx, []string{"add", "example.com", "--no-password"}, "User example.com created", []string{""}); err != nil {
			cx.t.Fatal(err)
		}
	} else {
		if err := ctlV3User(cx, []string{"add", "example.com", "--interactive=false"}, "User example.com created", []string{""}); err != nil {
			cx.t.Fatal(err)
		}
	}
	if err := e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "role", "add", "test-role-cn"), cx.envMap, "Role test-role-cn created"); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3User(cx, []string{"grant-role", "example.com", "test-role-cn"}, "Role test-role-cn is granted to user example.com", nil); err != nil {
		cx.t.Fatal(err)
	}

	// grant a new key for CN based user
	if err := ctlV3RoleGrantPermission(cx, "test-role-cn", grantingPerm{true, true, "hoo", "", false}); err != nil {
		cx.t.Fatal(err)
	}

	// grant a new key for username based user
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "bar", "", false}); err != nil {
		cx.t.Fatal(err)
	}

	// try a granted key for CN based user
	cx.user, cx.pass = "", ""
	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Error(err)
	}

	// try a granted key for username based user
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "bar", "bar", ""); err != nil {
		cx.t.Error(err)
	}

	// try a non-granted key for both of them
	cx.user, cx.pass = "", ""
	err := ctlV3PutFailPerm(cx, "baz", "bar")
	require.ErrorContains(cx.t, err, "permission denied")

	cx.user, cx.pass = "test-user", "pass"
	err = ctlV3PutFailPerm(cx, "baz", "bar")
	require.ErrorContains(cx.t, err, "permission denied")
}

func authTestCertCNAndUsername(cx ctlCtx) {
	certCNAndUsername(cx, false)
}

func authTestCertCNAndUsernameNoPassword(cx ctlCtx) {
	certCNAndUsername(cx, true)
}

func ctlV3User(cx ctlCtx, args []string, expStr string, stdIn []string) error {
	cmdArgs := append(cx.PrefixArgs(), "user")
	cmdArgs = append(cmdArgs, args...)

	proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		return err
	}
	defer proc.Close()

	// Send 'stdIn' strings as input.
	for _, s := range stdIn {
		if err = proc.Send(s + "\r"); err != nil {
			return err
		}
	}

	_, err = proc.Expect(expStr)
	return err
}

// authTestCacheReload tests the permissions when a member restarts
func authTestCacheReload(cx ctlCtx) {
	authData := []struct {
		user string
		role string
		pass string
	}{
		{
			user: "root",
			role: "root",
			pass: "123",
		},
		{
			user: "user0",
			role: "role0",
			pass: "123",
		},
	}

	node0 := cx.epc.Procs[0]
	endpoint := node0.EndpointsV3()[0]

	// create a client
	c, err := clientv3.New(clientv3.Config{Endpoints: []string{endpoint}, DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer c.Close()

	for _, authObj := range authData {
		// add role
		if _, err = c.RoleAdd(context.TODO(), authObj.role); err != nil {
			cx.t.Fatal(err)
		}

		// add user
		if _, err = c.UserAdd(context.TODO(), authObj.user, authObj.pass); err != nil {
			cx.t.Fatal(err)
		}

		// grant role to user
		if _, err = c.UserGrantRole(context.TODO(), authObj.user, authObj.role); err != nil {
			cx.t.Fatal(err)
		}
	}

	// role grant permission to role0
	if _, err = c.RoleGrantPermission(context.TODO(), authData[1].role, "foo", "", clientv3.PermissionType(clientv3.PermReadWrite)); err != nil {
		cx.t.Fatal(err)
	}

	// enable auth
	if _, err = c.AuthEnable(context.TODO()); err != nil {
		cx.t.Fatal(err)
	}

	// create another client with ID:Password
	c2, err := clientv3.New(clientv3.Config{Endpoints: []string{endpoint}, Username: authData[1].user, Password: authData[1].pass, DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer c2.Close()

	// create foo since that is within the permission set
	// expectation is to succeed
	if _, err = c2.Put(context.TODO(), "foo", "bar"); err != nil {
		cx.t.Fatal(err)
	}

	// restart the node
	if err := node0.Restart(context.TODO()); err != nil {
		cx.t.Fatal(err)
	}

	// nothing has changed, but it fails without refreshing cache after restart
	if _, err = c2.Put(context.TODO(), "foo", "bar2"); err != nil {
		cx.t.Fatal(err)
	}
}
