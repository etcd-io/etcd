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
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3AuthEnable(t *testing.T) {
	testCtl(t, authEnableTest)
}
func TestCtlV3AuthDisable(t *testing.T)             { testCtl(t, authDisableTest) }
func TestCtlV3AuthGracefulDisable(t *testing.T)     { testCtl(t, authGracefulDisableTest) }
func TestCtlV3AuthStatus(t *testing.T)              { testCtl(t, authStatusTest) }
func TestCtlV3AuthWriteKey(t *testing.T)            { testCtl(t, authCredWriteKeyTest) }
func TestCtlV3AuthRoleUpdate(t *testing.T)          { testCtl(t, authRoleUpdateTest) }
func TestCtlV3AuthUserDeleteDuringOps(t *testing.T) { testCtl(t, authUserDeleteDuringOpsTest) }
func TestCtlV3AuthRoleRevokeDuringOps(t *testing.T) { testCtl(t, authRoleRevokeDuringOpsTest) }
func TestCtlV3AuthTxn(t *testing.T)                 { testCtl(t, authTestTxn) }
func TestCtlV3AuthTxnJWT(t *testing.T)              { testCtl(t, authTestTxn, withCfg(*e2e.NewConfigJWT())) }
func TestCtlV3AuthPrefixPerm(t *testing.T)          { testCtl(t, authTestPrefixPerm) }
func TestCtlV3AuthMemberAdd(t *testing.T)           { testCtl(t, authTestMemberAdd) }
func TestCtlV3AuthMemberRemove(t *testing.T) {
	testCtl(t, authTestMemberRemove, withQuorum(), withNoStrictReconfig())
}
func TestCtlV3AuthMemberUpdate(t *testing.T)     { testCtl(t, authTestMemberUpdate) }
func TestCtlV3AuthRevokeWithDelete(t *testing.T) { testCtl(t, authTestRevokeWithDelete) }
func TestCtlV3AuthInvalidMgmt(t *testing.T)      { testCtl(t, authTestInvalidMgmt) }
func TestCtlV3AuthFromKeyPerm(t *testing.T)      { testCtl(t, authTestFromKeyPerm) }
func TestCtlV3AuthAndWatch(t *testing.T)         { testCtl(t, authTestWatch) }
func TestCtlV3AuthAndWatchJWT(t *testing.T)      { testCtl(t, authTestWatch, withCfg(*e2e.NewConfigJWT())) }

func TestCtlV3AuthLeaseTestKeepAlive(t *testing.T) { testCtl(t, authLeaseTestKeepAlive) }
func TestCtlV3AuthLeaseTestTimeToLiveExpired(t *testing.T) {
	testCtl(t, authLeaseTestTimeToLiveExpired)
}
func TestCtlV3AuthLeaseGrantLeases(t *testing.T) { testCtl(t, authLeaseTestLeaseGrantLeases) }
func TestCtlV3AuthLeaseGrantLeasesJWT(t *testing.T) {
	testCtl(t, authLeaseTestLeaseGrantLeases, withCfg(*e2e.NewConfigJWT()))
}
func TestCtlV3AuthLeaseRevoke(t *testing.T) { testCtl(t, authLeaseTestLeaseRevoke) }

func TestCtlV3AuthRoleGet(t *testing.T)  { testCtl(t, authTestRoleGet) }
func TestCtlV3AuthUserGet(t *testing.T)  { testCtl(t, authTestUserGet) }
func TestCtlV3AuthRoleList(t *testing.T) { testCtl(t, authTestRoleList) }

func TestCtlV3AuthDefrag(t *testing.T) { testCtl(t, authTestDefrag) }
func TestCtlV3AuthEndpointHealth(t *testing.T) {
	testCtl(t, authTestEndpointHealth, withQuorum())
}
func TestCtlV3AuthSnapshot(t *testing.T) { testCtl(t, authTestSnapshot) }
func TestCtlV3AuthSnapshotJWT(t *testing.T) {
	testCtl(t, authTestSnapshot, withCfg(*e2e.NewConfigJWT()))
}
func TestCtlV3AuthJWTExpire(t *testing.T) {
	testCtl(t, authTestJWTExpire, withCfg(*e2e.NewConfigJWT()))
}
func TestCtlV3AuthRevisionConsistency(t *testing.T) { testCtl(t, authTestRevisionConsistency) }
func TestCtlV3AuthTestCacheReload(t *testing.T)     { testCtl(t, authTestCacheReload) }
func TestCtlV3AuthLeaseTimeToLive(t *testing.T)     { testCtl(t, authTestLeaseTimeToLive) }

func TestCtlV3AuthRecoverFromSnapshot(t *testing.T) {
	testCtl(t, authTestRecoverSnapshot, withCfg(*e2e.NewConfigNoTLS()), withQuorum(), withSnapshotCount(5))
}

func authEnableTest(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}
}

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

func applyTLSWithRootCommonName() func() {
	var (
		oldCertPath       = e2e.CertPath
		oldPrivateKeyPath = e2e.PrivateKeyPath
		oldCaPath         = e2e.CaPath

		newCertPath       = filepath.Join(e2e.FixturesDir, "CommonName-root.crt")
		newPrivateKeyPath = filepath.Join(e2e.FixturesDir, "CommonName-root.key")
		newCaPath         = filepath.Join(e2e.FixturesDir, "CommonName-root.crt")
	)

	e2e.CertPath = newCertPath
	e2e.PrivateKeyPath = newPrivateKeyPath
	e2e.CaPath = newCaPath

	return func() {
		e2e.CertPath = oldCertPath
		e2e.PrivateKeyPath = oldPrivateKeyPath
		e2e.CaPath = oldCaPath
	}
}

func ctlV3AuthEnable(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "auth", "enable")
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, "Authentication Enabled")
}

func authDisableTest(cx ctlCtx) {
	// a key that isn't granted to test-user
	if err := ctlV3Put(cx, "hoo", "a", ""); err != nil {
		cx.t.Fatal(err)
	}

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// test-user doesn't have the permission, it must fail
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3PutFailPerm(cx, "hoo", "bar"); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	if err := ctlV3AuthDisable(cx); err != nil {
		cx.t.Fatalf("authDisableTest ctlV3AuthDisable error (%v)", err)
	}

	// now ErrAuthNotEnabled of Authenticate() is simply ignored
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}

	// now the key can be accessed
	cx.user, cx.pass = "", ""
	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"hoo"}, []kv{{"hoo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}
}

func authGracefulDisableTest(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"

	donec := make(chan struct{})

	go func() {
		defer close(donec)

		// sleep a bit to let the watcher connects while auth is still enabled
		time.Sleep(1000 * time.Millisecond)

		// now disable auth...
		if err := ctlV3AuthDisable(cx); err != nil {
			cx.t.Fatalf("authGracefulDisableTest ctlV3AuthDisable error (%v)", err)
		}

		// ...and restart the node
		node0 := cx.epc.Procs[0]
		node0.WithStopSignal(syscall.SIGINT)
		if rerr := node0.Restart(); rerr != nil {
			cx.t.Fatal(rerr)
		}

		// the watcher should still work after reconnecting
		if perr := ctlV3Put(cx, "key", "value", ""); perr != nil {
			cx.t.Errorf("authGracefulDisableTest ctlV3Put error (%v)", perr)
		}
	}()

	err := ctlV3Watch(cx, []string{"key"}, kvExec{key: "key", val: "value"})

	if err != nil {
		if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
			cx.t.Errorf("authGracefulDisableTest ctlV3Watch error (%v)", err)
		}
	}

	<-donec
}

func ctlV3AuthDisable(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "auth", "disable")
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, "Authentication Disabled")
}

func authStatusTest(cx ctlCtx) {
	cmdArgs := append(cx.PrefixArgs(), "auth", "status")
	if err := e2e.SpawnWithExpects(cmdArgs, cx.envMap, "Authentication Status: false", "AuthRevision:"); err != nil {
		cx.t.Fatal(err)
	}

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	cmdArgs = append(cx.PrefixArgs(), "auth", "status")

	if err := e2e.SpawnWithExpects(cmdArgs, cx.envMap, "Authentication Status: true", "AuthRevision:"); err != nil {
		cx.t.Fatal(err)
	}

	cmdArgs = append(cx.PrefixArgs(), "auth", "status", "--write-out", "json")
	if err := e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, "enabled"); err != nil {
		cx.t.Fatal(err)
	}
	if err := e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, "authRevision"); err != nil {
		cx.t.Fatal(err)
	}
}

func authCredWriteKeyTest(cx ctlCtx) {
	// baseline key to check for failed puts
	if err := ctlV3Put(cx, "foo", "a", ""); err != nil {
		cx.t.Fatal(err)
	}

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// confirm root role can access to all keys
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// try invalid user
	cx.user, cx.pass = "a", "b"
	if err := ctlV3PutFailAuth(cx, "foo", "bar"); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put failed
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// try good user
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "foo", "bar2", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar2"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// try bad password
	cx.user, cx.pass = "test-user", "badpass"
	if err := ctlV3PutFailAuth(cx, "foo", "baz"); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put failed
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar2"}}...); err != nil {
		cx.t.Fatal(err)
	}
}

func authRoleUpdateTest(cx ctlCtx) {
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// try put to not granted key
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3PutFailPerm(cx, "hoo", "bar"); err != nil {
		cx.t.Fatal(err)
	}

	// grant a new key
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "hoo", "", false}); err != nil {
		cx.t.Fatal(err)
	}

	// try a newly granted key
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"hoo"}, []kv{{"hoo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// revoke the newly granted key
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleRevokePermission(cx, "test-role", "hoo", "", false); err != nil {
		cx.t.Fatal(err)
	}

	// try put to the revoked key
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3PutFailPerm(cx, "hoo", "bar"); err != nil {
		cx.t.Fatal(err)
	}

	// confirm a key still granted can be accessed
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}
}

func authUserDeleteDuringOpsTest(cx ctlCtx) {
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// create a key
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// delete the user
	cx.user, cx.pass = "root", "root"
	err := ctlV3User(cx, []string{"delete", "test-user"}, "User test-user deleted", []string{})
	if err != nil {
		cx.t.Fatal(err)
	}

	// check the user is deleted
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3PutFailAuth(cx, "foo", "baz"); err != nil {
		cx.t.Fatal(err)
	}
}

func authRoleRevokeDuringOpsTest(cx ctlCtx) {
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// create a key
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// create a new role
	cx.user, cx.pass = "root", "root"
	if err := ctlV3Role(cx, []string{"add", "test-role2"}, "Role test-role2 created"); err != nil {
		cx.t.Fatal(err)
	}
	// grant a new key to the new role
	if err := ctlV3RoleGrantPermission(cx, "test-role2", grantingPerm{true, true, "hoo", "", false}); err != nil {
		cx.t.Fatal(err)
	}
	// grant the new role to the user
	if err := ctlV3User(cx, []string{"grant-role", "test-user", "test-role2"}, "Role test-role2 is granted to user test-user", nil); err != nil {
		cx.t.Fatal(err)
	}

	// try a newly granted key
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"hoo"}, []kv{{"hoo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// revoke a role from the user
	cx.user, cx.pass = "root", "root"
	err := ctlV3User(cx, []string{"revoke-role", "test-user", "test-role"}, "Role test-role is revoked from user test-user", []string{})
	if err != nil {
		cx.t.Fatal(err)
	}

	// check the role is revoked and permission is lost from the user
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3PutFailPerm(cx, "foo", "baz"); err != nil {
		cx.t.Fatal(err)
	}

	// try a key that can be accessed from the remaining role
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "hoo", "bar2", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"hoo"}, []kv{{"hoo", "bar2"}}...); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3PutFailAuth(cx ctlCtx, key, val string) error {
	return e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "put", key, val), cx.envMap, "authentication failed")
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

func authTestTxn(cx ctlCtx) {
	// keys with 1 suffix aren't granted to test-user
	// keys with 2 suffix are granted to test-user

	keys := []string{"c1", "s1", "f1"}
	grantedKeys := []string{"c2", "s2", "f2"}
	for _, key := range keys {
		if err := ctlV3Put(cx, key, "v", ""); err != nil {
			cx.t.Fatal(err)
		}
	}

	for _, key := range grantedKeys {
		if err := ctlV3Put(cx, key, "v", ""); err != nil {
			cx.t.Fatal(err)
		}
	}

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// grant keys to test-user
	cx.user, cx.pass = "root", "root"
	for _, key := range grantedKeys {
		if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, key, "", false}); err != nil {
			cx.t.Fatal(err)
		}
	}

	// now test txn
	cx.interactive = true
	cx.user, cx.pass = "test-user", "pass"

	rqs := txnRequests{
		compare:  []string{`version("c2") = "1"`},
		ifSucess: []string{"get s2"},
		ifFail:   []string{"get f2"},
		results:  []string{"SUCCESS", "s2", "v"},
	}
	if err := ctlV3Txn(cx, rqs); err != nil {
		cx.t.Fatal(err)
	}

	// a key of compare case isn't granted
	rqs = txnRequests{
		compare:  []string{`version("c1") = "1"`},
		ifSucess: []string{"get s2"},
		ifFail:   []string{"get f2"},
		results:  []string{"Error: etcdserver: permission denied"},
	}
	if err := ctlV3Txn(cx, rqs); err != nil {
		cx.t.Fatal(err)
	}

	// a key of success case isn't granted
	rqs = txnRequests{
		compare:  []string{`version("c2") = "1"`},
		ifSucess: []string{"get s1"},
		ifFail:   []string{"get f2"},
		results:  []string{"Error: etcdserver: permission denied"},
	}
	if err := ctlV3Txn(cx, rqs); err != nil {
		cx.t.Fatal(err)
	}

	// a key of failure case isn't granted
	rqs = txnRequests{
		compare:  []string{`version("c2") = "1"`},
		ifSucess: []string{"get s2"},
		ifFail:   []string{"get f1"},
		results:  []string{"Error: etcdserver: permission denied"},
	}
	if err := ctlV3Txn(cx, rqs); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestPrefixPerm(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	prefix := "/prefix/" // directory like prefix
	// grant keys to test-user
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, prefix, "", true}); err != nil {
		cx.t.Fatal(err)
	}

	// try a prefix granted permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		if err := ctlV3Put(cx, key, "val", ""); err != nil {
			cx.t.Fatal(err)
		}
	}

	if err := ctlV3PutFailPerm(cx, clientv3.GetPrefixRangeEnd(prefix), "baz"); err != nil {
		cx.t.Fatal(err)
	}

	// grant the entire keys to test-user
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "", "", true}); err != nil {
		cx.t.Fatal(err)
	}

	prefix2 := "/prefix2/"
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("%s%d", prefix2, i)
		if err := ctlV3Put(cx, key, "val", ""); err != nil {
			cx.t.Fatal(err)
		}
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

func authTestMemberRemove(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	ep, memIDToRemove, clusterID := cx.memberToRemove()

	// ordinary user cannot remove a member
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3MemberRemove(cx, ep, memIDToRemove, clusterID); err == nil {
		cx.t.Fatalf("ordinary user must not be allowed to remove a member")
	}

	// root can remove a member
	cx.user, cx.pass = "root", "root"
	if err := ctlV3MemberRemove(cx, ep, memIDToRemove, clusterID); err != nil {
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

	// try a non granted key
	cx.user, cx.pass = "", ""
	if err := ctlV3PutFailPerm(cx, "baz", "bar"); err != nil {
		cx.t.Error(err)
	}
}

func authTestRevokeWithDelete(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// create a new role
	cx.user, cx.pass = "root", "root"
	if err := ctlV3Role(cx, []string{"add", "test-role2"}, "Role test-role2 created"); err != nil {
		cx.t.Fatal(err)
	}

	// grant the new role to the user
	if err := ctlV3User(cx, []string{"grant-role", "test-user", "test-role2"}, "Role test-role2 is granted to user test-user", nil); err != nil {
		cx.t.Fatal(err)
	}

	// check the result
	if err := ctlV3User(cx, []string{"get", "test-user"}, "Roles: test-role test-role2", nil); err != nil {
		cx.t.Fatal(err)
	}

	// delete the role, test-role2 must be revoked from test-user
	if err := ctlV3Role(cx, []string{"delete", "test-role2"}, "Role test-role2 deleted"); err != nil {
		cx.t.Fatal(err)
	}

	// check the result
	if err := ctlV3User(cx, []string{"get", "test-user"}, "Roles: test-role", nil); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestInvalidMgmt(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	if err := ctlV3Role(cx, []string{"delete", "root"}, "Error: etcdserver: invalid auth management"); err == nil {
		cx.t.Fatal("deleting the role root must not be allowed")
	}

	if err := ctlV3User(cx, []string{"revoke-role", "root", "root"}, "Error: etcdserver: invalid auth management", []string{}); err == nil {
		cx.t.Fatal("revoking the role root from the user root must not be allowed")
	}
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
	if err := ctlV3PutFailPerm(cx, "x", "baz"); err != nil {
		cx.t.Fatal(err)
	}

	// revoke the open ended permission
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleRevokePermission(cx, "test-role", "z", "", true); err != nil {
		cx.t.Fatal(err)
	}

	// try the revoked open ended permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		if err := ctlV3PutFailPerm(cx, key, "val"); err != nil {
			cx.t.Fatal(err)
		}
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
		if err := ctlV3PutFailPerm(cx, key, "val"); err != nil {
			cx.t.Fatal(err)
		}
	}
}

func authLeaseTestKeepAlive(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)
	// put with TTL 10 seconds and keep-alive
	leaseID, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3LeaseGrant error (%v)", err)
	}
	if err := ctlV3Put(cx, "key", "val", leaseID); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3Put error (%v)", err)
	}
	if err := ctlV3LeaseKeepAlive(cx, leaseID); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3LeaseKeepAlive error (%v)", err)
	}
	if err := ctlV3Get(cx, []string{"key"}, kv{"key", "val"}); err != nil {
		cx.t.Fatalf("leaseTestKeepAlive: ctlV3Get error (%v)", err)
	}
}

func authLeaseTestTimeToLiveExpired(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	ttl := 3
	if err := leaseTestTimeToLiveExpire(cx, ttl); err != nil {
		cx.t.Fatalf("leaseTestTimeToLiveExpire: error (%v)", err)
	}
}

func authLeaseTestLeaseGrantLeases(cx ctlCtx) {
	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	if err := leaseTestGrantLeasesList(cx); err != nil {
		cx.t.Fatalf("authLeaseTestLeaseGrantLeases: error (%v)", err)
	}
}

func authLeaseTestLeaseRevoke(cx ctlCtx) {
	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// put with TTL 10 seconds and revoke
	leaseID, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		cx.t.Fatalf("ctlV3LeaseGrant error (%v)", err)
	}
	if err := ctlV3Put(cx, "key", "val", leaseID); err != nil {
		cx.t.Fatalf("ctlV3Put error (%v)", err)
	}
	if err := ctlV3LeaseRevoke(cx, leaseID); err != nil {
		cx.t.Fatalf("ctlV3LeaseRevoke error (%v)", err)
	}
	if err := ctlV3GetWithErr(cx, []string{"key"}, []string{"retrying of unary invoker failed"}); err != nil { // expect errors
		cx.t.Fatalf("ctlV3GetWithErr error (%v)", err)
	}
}

func authTestWatch(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// grant a key range
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "key", "key4", false}); err != nil {
		cx.t.Fatal(err)
	}

	tests := []struct {
		puts []kv
		args []string

		wkv  []kvExec
		want bool
	}{
		{ // watch 1 key, should be successful
			[]kv{{"key", "value"}},
			[]string{"key", "--rev", "1"},
			[]kvExec{{key: "key", val: "value"}},
			true,
		},
		{ // watch 3 keys by range, should be successful
			[]kv{{"key1", "val1"}, {"key3", "val3"}, {"key2", "val2"}},
			[]string{"key", "key3", "--rev", "1"},
			[]kvExec{{key: "key1", val: "val1"}, {key: "key2", val: "val2"}},
			true,
		},

		{ // watch 1 key, should not be successful
			[]kv{},
			[]string{"key5", "--rev", "1"},
			[]kvExec{},
			false,
		},
		{ // watch 3 keys by range, should not be successful
			[]kv{},
			[]string{"key", "key6", "--rev", "1"},
			[]kvExec{},
			false,
		},
	}

	cx.user, cx.pass = "test-user", "pass"
	for i, tt := range tests {
		donec := make(chan struct{})
		go func(i int, puts []kv) {
			defer close(donec)
			for j := range puts {
				if err := ctlV3Put(cx, puts[j].key, puts[j].val, ""); err != nil {
					cx.t.Errorf("watchTest #%d-%d: ctlV3Put error (%v)", i, j, err)
				}
			}
		}(i, tt.puts)

		var err error
		if tt.want {
			err = ctlV3Watch(cx, tt.args, tt.wkv...)
		} else {
			err = ctlV3WatchFailPerm(cx, tt.args)
		}

		if err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Errorf("watchTest #%d: ctlV3Watch error (%v)", i, err)
			}
		}

		<-donec
	}

}

func authTestRoleGet(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}
	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	expected := []string{
		"Role test-role",
		"KV Read:", "foo",
		"KV Write:", "foo",
	}
	if err := e2e.SpawnWithExpects(append(cx.PrefixArgs(), "role", "get", "test-role"), cx.envMap, expected...); err != nil {
		cx.t.Fatal(err)
	}

	// test-user can get the information of test-role because it belongs to the role
	cx.user, cx.pass = "test-user", "pass"
	if err := e2e.SpawnWithExpects(append(cx.PrefixArgs(), "role", "get", "test-role"), cx.envMap, expected...); err != nil {
		cx.t.Fatal(err)
	}

	// test-user cannot get the information of root because it doesn't belong to the role
	expected = []string{
		"Error: etcdserver: permission denied",
	}
	if err := e2e.SpawnWithExpects(append(cx.PrefixArgs(), "role", "get", "root"), cx.envMap, expected...); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestUserGet(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}
	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	expected := []string{
		"User: test-user",
		"Roles: test-role",
	}

	if err := e2e.SpawnWithExpects(append(cx.PrefixArgs(), "user", "get", "test-user"), cx.envMap, expected...); err != nil {
		cx.t.Fatal(err)
	}

	// test-user can get the information of test-user itself
	cx.user, cx.pass = "test-user", "pass"
	if err := e2e.SpawnWithExpects(append(cx.PrefixArgs(), "user", "get", "test-user"), cx.envMap, expected...); err != nil {
		cx.t.Fatal(err)
	}

	// test-user cannot get the information of root
	expected = []string{
		"Error: etcdserver: permission denied",
	}
	if err := e2e.SpawnWithExpects(append(cx.PrefixArgs(), "user", "get", "root"), cx.envMap, expected...); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestRoleList(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}
	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)
	if err := e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "role", "list"), cx.envMap, "test-role"); err != nil {
		cx.t.Fatal(err)
	}
}

func authTestDefrag(cx ctlCtx) {
	maintenanceInitKeys(cx)

	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// ordinary user cannot defrag
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3OnlineDefrag(cx); err == nil {
		cx.t.Fatal("ordinary user should not be able to issue a defrag request")
	}

	// root can defrag
	cx.user, cx.pass = "root", "root"
	if err := ctlV3OnlineDefrag(cx); err != nil {
		cx.t.Fatal(err)
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

func authTestEndpointHealth(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	if err := ctlV3EndpointHealth(cx); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointHealth error (%v)", err)
	}

	// health checking with an ordinary user "succeeds" since permission denial goes through consensus
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3EndpointHealth(cx); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointHealth error (%v)", err)
	}

	// succeed if permissions granted for ordinary user
	cx.user, cx.pass = "root", "root"
	if err := ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "health", "", false}); err != nil {
		cx.t.Fatal(err)
	}
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3EndpointHealth(cx); err != nil {
		cx.t.Fatalf("endpointStatusTest ctlV3EndpointHealth error (%v)", err)
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

	// try a non granted key for both of them
	cx.user, cx.pass = "", ""
	if err := ctlV3PutFailPerm(cx, "baz", "bar"); err != nil {
		cx.t.Error(err)
	}

	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3PutFailPerm(cx, "baz", "bar"); err != nil {
		cx.t.Error(err)
	}
}

func authTestCertCNAndUsername(cx ctlCtx) {
	certCNAndUsername(cx, false)
}

func authTestCertCNAndUsernameNoPassword(cx ctlCtx) {
	certCNAndUsername(cx, true)
}

func authTestJWTExpire(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// try a granted key
	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Error(err)
	}

	// wait an expiration of my JWT token
	<-time.After(3 * time.Second)

	if err := ctlV3Put(cx, "hoo", "bar", ""); err != nil {
		cx.t.Error(err)
	}
}

func authTestRevisionConsistency(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}
	cx.user, cx.pass = "root", "root"

	// add user
	if err := ctlV3User(cx, []string{"add", "test-user", "--interactive=false"}, "User test-user created", []string{"pass"}); err != nil {
		cx.t.Fatal(err)
	}
	// delete the same user
	if err := ctlV3User(cx, []string{"delete", "test-user"}, "User test-user deleted", []string{}); err != nil {
		cx.t.Fatal(err)
	}

	// get node0 auth revision
	node0 := cx.epc.Procs[0]
	endpoint := node0.EndpointsV3()[0]
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{endpoint}, Username: cx.user, Password: cx.pass, DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer cli.Close()

	sresp, err := cli.AuthStatus(context.TODO())
	if err != nil {
		cx.t.Fatal(err)
	}
	oldAuthRevision := sresp.AuthRevision

	// restart the node
	node0.WithStopSignal(syscall.SIGINT)
	if err := node0.Restart(); err != nil {
		cx.t.Fatal(err)
	}

	// get node0 auth revision again
	sresp, err = cli.AuthStatus(context.TODO())
	if err != nil {
		cx.t.Fatal(err)
	}
	newAuthRevision := sresp.AuthRevision

	// assert AuthRevision equal
	if newAuthRevision != oldAuthRevision {
		cx.t.Fatalf("auth revison shouldn't change when restarting etcd, expected: %d, got: %d", oldAuthRevision, newAuthRevision)
	}
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
	node0.WithStopSignal(syscall.SIGINT)
	if err := node0.Restart(); err != nil {
		cx.t.Fatal(err)
	}

	// nothing has changed, but it fails without refreshing cache after restart
	if _, err = c2.Put(context.TODO(), "foo", "bar2"); err != nil {
		cx.t.Fatal(err)
	}
}

// Verify that etcd works after recovering from a snapshot.
// Refer to https://github.com/etcd-io/etcd/issues/14571.
func authTestRecoverSnapshot(cx ctlCtx) {
	roles := []authRole{
		{
			role:       "role0",
			permission: clientv3.PermissionType(clientv3.PermReadWrite),
			key:        "foo",
		},
	}

	users := []authUser{
		{
			user: "root",
			pass: "rootPass",
			role: "root",
		},
		{
			user: "user0",
			pass: "user0Pass",
			role: "role0",
		},
	}

	cx.t.Log("setup and enable auth")
	setupAuth(cx, roles, users)

	// create a client with root user
	cx.t.Log("create a client with root user")
	cliRoot, err := clientv3.New(clientv3.Config{Endpoints: cx.epc.EndpointsV3(), Username: "root", Password: "rootPass", DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer cliRoot.Close()

	// write more than SnapshotCount keys, so that at least one snapshot is created
	cx.t.Log("Write enough key/value to trigger a snapshot")
	for i := 0; i <= 6; i++ {
		if _, err := cliRoot.Put(context.TODO(), fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i)); err != nil {
			cx.t.Fatalf("failed to Put (%v)", err)
		}
	}

	// add a new member into the cluster
	// Refer to https://github.com/etcd-io/etcd/blob/17cb291f1515d0a4d712acdf396a1f2874f172bf/tests/e2e/cluster_test.go#L238
	var (
		idx            = 3
		name           = fmt.Sprintf("test-%d", idx)
		port           = cx.cfg.BasePort + 5*idx
		curlHost       = fmt.Sprintf("localhost:%d", port)
		nodeClientURL  = url.URL{Scheme: cx.cfg.ClientScheme(), Host: curlHost}
		nodePeerURL    = url.URL{Scheme: cx.cfg.PeerScheme(), Host: fmt.Sprintf("localhost:%d", port+1)}
		initialCluster = cx.epc.Procs[0].Config().InitialCluster + "," + fmt.Sprintf("%s=%s", name, nodePeerURL.String())
	)
	cx.t.Logf("Adding a new member: %s", nodePeerURL.String())
	// Must wait at least 5 seconds, otherwise it will always get an
	// "etcdserver: unhealthy cluster" response, please refer to link below,
	// https://github.com/etcd-io/etcd/blob/17cb291f1515d0a4d712acdf396a1f2874f172bf/server/etcdserver/server.go#L1611
	assert.Eventually(cx.t, func() bool {
		if _, err := cliRoot.MemberAdd(context.TODO(), []string{nodePeerURL.String()}); err != nil {
			cx.t.Logf("Failed to add member, peelURL: %s, error: %v", nodePeerURL.String(), err)
			return false
		}
		return true
	}, 8*time.Second, 2*time.Second)

	cx.t.Logf("Starting the new member: %s", nodePeerURL.String())
	newProc, err := runEtcdNode(name, cx.t.TempDir(), nodeClientURL.String(), nodePeerURL.String(), "existing", initialCluster)
	require.NoError(cx.t, err)
	defer newProc.Stop()

	// create a client with user "user0", and connects to the new member
	cx.t.Log("create a client with user 'user0'")
	cliUser, err := clientv3.New(clientv3.Config{Endpoints: []string{nodeClientURL.String()}, Username: "user0", Password: "user0Pass", DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer cliUser.Close()

	// write data using the cliUser, expect no error
	cx.t.Log("Write a key/value using user 'user0'")
	_, err = cliUser.Put(context.TODO(), "foo", "bar")
	require.NoError(cx.t, err)

	//verify all nodes have the same revision and hash
	var endpoints []string
	for _, proc := range cx.epc.Procs {
		endpoints = append(endpoints, proc.Config().Acurl)
	}
	endpoints = append(endpoints, nodeClientURL.String())
	cx.t.Log("Verify all members have the same revision and hash")
	assert.Eventually(cx.t, func() bool {
		hashKvs, err := hashKVs(endpoints, cliRoot)
		if err != nil {
			cx.t.Logf("failed to get HashKV: %v", err)
			return false
		}

		if len(hashKvs) != 4 {
			cx.t.Logf("expected 4 hashkv responses, but got: %d", len(hashKvs))
			return false
		}

		if !(hashKvs[0].Header.Revision == hashKvs[1].Header.Revision &&
			hashKvs[0].Header.Revision == hashKvs[2].Header.Revision &&
			hashKvs[0].Header.Revision == hashKvs[3].Header.Revision) {
			cx.t.Logf("Got different revisions, [%d, %d, %d, %d]",
				hashKvs[0].Header.Revision,
				hashKvs[1].Header.Revision,
				hashKvs[2].Header.Revision,
				hashKvs[3].Header.Revision)
			return false
		}

		assert.Equal(cx.t, hashKvs[0].Hash, hashKvs[1].Hash)
		assert.Equal(cx.t, hashKvs[0].Hash, hashKvs[2].Hash)
		assert.Equal(cx.t, hashKvs[0].Hash, hashKvs[3].Hash)

		return true
	}, 5*time.Second, 100*time.Millisecond)
}

type authRole struct {
	role       string
	permission clientv3.PermissionType
	key        string
	keyEnd     string
}

type authUser struct {
	user string
	pass string
	role string
}

func setupAuth(cx ctlCtx, roles []authRole, users []authUser) {
	endpoint := cx.epc.Procs[0].EndpointsV3()[0]

	// create a client
	c, err := clientv3.New(clientv3.Config{Endpoints: []string{endpoint}, DialTimeout: 3 * time.Second})
	if err != nil {
		cx.t.Fatal(err)
	}
	defer c.Close()

	// create roles
	for _, r := range roles {
		// add role
		if _, err = c.RoleAdd(context.TODO(), r.role); err != nil {
			cx.t.Fatal(err)
		}

		// grant permission to role
		if _, err = c.RoleGrantPermission(context.TODO(), r.role, r.key, r.keyEnd, r.permission); err != nil {
			cx.t.Fatal(err)
		}
	}

	// create users
	for _, u := range users {
		// add user
		if _, err = c.UserAdd(context.TODO(), u.user, u.pass); err != nil {
			cx.t.Fatal(err)
		}

		// grant role to user
		if _, err = c.UserGrantRole(context.TODO(), u.user, u.role); err != nil {
			cx.t.Fatal(err)
		}
	}

	// enable auth
	if _, err = c.AuthEnable(context.TODO()); err != nil {
		cx.t.Fatal(err)
	}
}

func hashKVs(endpoints []string, cli *clientv3.Client) ([]*clientv3.HashKVResponse, error) {
	var retHashKVs []*clientv3.HashKVResponse
	for _, ep := range endpoints {
		resp, err := cli.HashKV(context.TODO(), ep, 0)
		if err != nil {
			return nil, err
		}
		retHashKVs = append(retHashKVs, resp)
	}
	return retHashKVs, nil
}

func authTestLeaseTimeToLive(cx ctlCtx) {
	if err := authEnable(cx); err != nil {
		cx.t.Fatal(err)
	}
	cx.user, cx.pass = "root", "root"

	authSetupTestUser(cx)

	cx.user = "test-user"
	cx.pass = "pass"

	leaseID, err := ctlV3LeaseGrant(cx, 10)
	if err != nil {
		cx.t.Fatal(err)
	}

	err = ctlV3Put(cx, "foo", "val", leaseID)
	if err != nil {
		cx.t.Fatal(err)
	}

	err = ctlV3LeaseTimeToLive(cx, leaseID, true)
	if err != nil {
		cx.t.Fatal(err)
	}

	cx.user = "root"
	cx.pass = "root"
	err = ctlV3Put(cx, "bar", "val", leaseID)
	if err != nil {
		cx.t.Fatal(err)
	}

	cx.user = "test-user"
	cx.pass = "pass"
	// the lease is attached to bar, which test-user cannot access
	err = ctlV3LeaseTimeToLive(cx, leaseID, true)
	if err == nil {
		cx.t.Fatal("test-user must not be able to access to the lease, because it's attached to the key bar")
	}

	// without --keys, access should be allowed
	err = ctlV3LeaseTimeToLive(cx, leaseID, false)
	if err != nil {
		cx.t.Fatal(err)
	}
}
