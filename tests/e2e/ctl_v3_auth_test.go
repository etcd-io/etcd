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
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3AuthMemberUpdate(t *testing.T) { testCtl(t, authTestMemberUpdate) }
func TestCtlV3AuthFromKeyPerm(t *testing.T)  { testCtl(t, authTestFromKeyPerm) }

// TestCtlV3AuthAndWatch TODO https://github.com/etcd-io/etcd/issues/7988 is the blocker of migration to common/auth_test.go
func TestCtlV3AuthAndWatch(t *testing.T)    { testCtl(t, authTestWatch) }
func TestCtlV3AuthAndWatchJWT(t *testing.T) { testCtl(t, authTestWatch, withCfg(*e2e.NewConfigJWT())) }

// TestCtlV3AuthEndpointHealth https://github.com/etcd-io/etcd/pull/13774#discussion_r1189118815 is the blocker of migration to common/auth_test.go
func TestCtlV3AuthEndpointHealth(t *testing.T) {
	testCtl(t, authTestEndpointHealth, withQuorum())
}

// TestCtlV3AuthSnapshot TODO fill up common/maintenance_auth_test.go when Snapshot API is added in interfaces.Client
func TestCtlV3AuthSnapshot(t *testing.T) { testCtl(t, authTestSnapshot) }

func TestCtlV3AuthSnapshotJWT(t *testing.T) {
	testCtl(t, authTestSnapshot, withCfg(*e2e.NewConfigJWT()))
}

func authEnable(cx ctlCtx) error {
	// create root user with root role
	if err := ctlV3User(cx, []string{"add", "root", "--interactive=false"}, "User root created", []string{"root"}); err != nil {
		return fmt.Errorf("failed to create root user %w", err)
	}
	if err := ctlV3User(cx, []string{"grant-role", "root", "root"}, "Role root is granted to user root", nil); err != nil {
		return fmt.Errorf("failed to grant root user root role %w", err)
	}
	if err := ctlV3AuthEnable(cx); err != nil {
		return fmt.Errorf("authEnableTest ctlV3AuthEnable error (%w)", err)
	}
	return nil
}

func ctlV3AuthEnable(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "auth", "enable")
	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: "Authentication Enabled"})
}

func ctlV3PutFailPerm(cx ctlCtx, key, val string) error {
	return e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "put", key, val), cx.envMap, expect.ExpectedResponse{Value: "permission denied"})
}

func authSetupTestUser(cx ctlCtx) {
	err := ctlV3User(cx, []string{"add", "test-user", "--interactive=false"}, "User test-user created", []string{"pass"})
	require.NoError(cx.t, err)
	err = e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "role", "add", "test-role"), cx.envMap, expect.ExpectedResponse{Value: "Role test-role created"})
	require.NoError(cx.t, err)
	err = ctlV3User(cx, []string{"grant-role", "test-user", "test-role"}, "Role test-role is granted to user test-user", nil)
	require.NoError(cx.t, err)
	cmd := append(cx.PrefixArgs(), "role", "grant-permission", "test-role", "readwrite", "foo")
	err = e2e.SpawnWithExpectWithEnv(cmd, cx.envMap, expect.ExpectedResponse{Value: "Role test-role updated"})
	require.NoError(cx.t, err)
}

func authTestMemberUpdate(cx ctlCtx) {
	require.NoError(cx.t, authEnable(cx))

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	mr, err := getMemberList(cx, false)
	require.NoError(cx.t, err)

	// ordinary user cannot update a member
	cx.user, cx.pass = "test-user", "pass"
	peerURL := fmt.Sprintf("http://localhost:%d", e2e.EtcdProcessBasePort+11)
	memberID := fmt.Sprintf("%x", mr.Members[0].ID)
	require.Errorf(cx.t, ctlV3MemberUpdate(cx, memberID, peerURL), "ordinary user must not be allowed to update a member")

	// root can update a member
	cx.user, cx.pass = "root", "root"
	err = ctlV3MemberUpdate(cx, memberID, peerURL)
	require.NoError(cx.t, err)
}

func authTestCertCN(cx ctlCtx) {
	require.NoError(cx.t, authEnable(cx))

	cx.user, cx.pass = "root", "root"
	require.NoError(cx.t, ctlV3User(cx, []string{"add", "example.com", "--interactive=false"}, "User example.com created", []string{""}))
	require.NoError(cx.t, e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "role", "add", "test-role"), cx.envMap, expect.ExpectedResponse{Value: "Role test-role created"}))
	require.NoError(cx.t, ctlV3User(cx, []string{"grant-role", "example.com", "test-role"}, "Role test-role is granted to user example.com", nil))

	// grant a new key
	require.NoError(cx.t, ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "hoo", "", false}))

	// try a granted key
	cx.user, cx.pass = "", ""
	assert.NoError(cx.t, ctlV3Put(cx, "hoo", "bar", ""))

	// try a non-granted key
	cx.user, cx.pass = "", ""
	require.ErrorContains(cx.t, ctlV3PutFailPerm(cx, "baz", "bar"), "permission denied")
}

func authTestFromKeyPerm(cx ctlCtx) {
	require.NoError(cx.t, authEnable(cx))

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// grant keys after z to test-user
	cx.user, cx.pass = "root", "root"
	require.NoError(cx.t, ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "z", "\x00", false}))

	// try the granted open ended permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		require.NoError(cx.t, ctlV3Put(cx, key, "val", ""))
	}
	largeKey := ""
	for i := 0; i < 10; i++ {
		largeKey += "\xff"
		require.NoError(cx.t, ctlV3Put(cx, largeKey, "val", ""))
	}

	// try a non granted key
	require.ErrorContains(cx.t, ctlV3PutFailPerm(cx, "x", "baz"), "permission denied")

	// revoke the open ended permission
	cx.user, cx.pass = "root", "root"
	require.NoError(cx.t, ctlV3RoleRevokePermission(cx, "test-role", "z", "", true))

	// try the revoked open ended permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		require.ErrorContains(cx.t, ctlV3PutFailPerm(cx, key, "val"), "permission denied")
	}

	// grant the entire keys
	cx.user, cx.pass = "root", "root"
	require.NoError(cx.t, ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "", "\x00", false}))

	// try keys, of course it must be allowed because test-role has a permission of the entire keys
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		require.NoError(cx.t, ctlV3Put(cx, key, "val", ""))
	}

	// revoke the entire keys
	cx.user, cx.pass = "root", "root"
	require.NoError(cx.t, ctlV3RoleRevokePermission(cx, "test-role", "", "", true))

	// try the revoked entire key permission
	cx.user, cx.pass = "test-user", "pass"
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("z%d", i)
		err := ctlV3PutFailPerm(cx, key, "val")
		require.ErrorContains(cx.t, err, "permission denied")
	}
}

func authTestWatch(cx ctlCtx) {
	require.NoError(cx.t, authEnable(cx))

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	// grant a key range
	require.NoError(cx.t, ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "key", "key4", false}))

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
				assert.NoErrorf(cx.t, ctlV3Put(cx, puts[j].key, puts[j].val, ""), "watchTest #%d-%d: ctlV3Put error", i, j)
			}
		}(i, tt.puts)

		var err error
		if tt.want {
			err = ctlV3Watch(cx, tt.args, tt.wkv...)
			if err != nil && cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Errorf("watchTest #%d: ctlV3Watch error (%v)", i, err)
			}
		} else {
			err = ctlV3WatchFailPerm(cx, tt.args)
			// this will not have any meaningful error output, but the process fails due to the cancellation
			require.ErrorContains(cx.t, err, "unexpected exit code")
		}

		<-donec
	}
}

func authTestSnapshot(cx ctlCtx) {
	maintenanceInitKeys(cx)

	require.NoError(cx.t, authEnable(cx))

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	fpath := "test-auth.snapshot"
	defer os.RemoveAll(fpath)

	// ordinary user cannot save a snapshot
	cx.user, cx.pass = "test-user", "pass"
	require.Errorf(cx.t, ctlV3SnapshotSave(cx, fpath), "ordinary user should not be able to save a snapshot")

	// root can save a snapshot
	cx.user, cx.pass = "root", "root"
	require.NoErrorf(cx.t, ctlV3SnapshotSave(cx, fpath), "snapshotTest ctlV3SnapshotSave error")

	st, err := getSnapshotStatus(cx, fpath)
	require.NoErrorf(cx.t, err, "snapshotTest getSnapshotStatus error")
	require.Equalf(cx.t, int64(4), st.Revision, "expected 4, got %d", st.Revision)
	require.GreaterOrEqualf(cx.t, st.TotalKey, 1, "expected at least 1, got %d", st.TotalKey)
}

func authTestEndpointHealth(cx ctlCtx) {
	require.NoError(cx.t, authEnable(cx))

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	require.NoErrorf(cx.t, ctlV3EndpointHealth(cx), "endpointStatusTest ctlV3EndpointHealth error")

	// health checking with an ordinary user "succeeds" since permission denial goes through consensus
	cx.user, cx.pass = "test-user", "pass"
	require.NoErrorf(cx.t, ctlV3EndpointHealth(cx), "endpointStatusTest ctlV3EndpointHealth error")

	// succeed if permissions granted for ordinary user
	cx.user, cx.pass = "root", "root"
	require.NoError(cx.t, ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "health", "", false}))
	cx.user, cx.pass = "test-user", "pass"
	require.NoErrorf(cx.t, ctlV3EndpointHealth(cx), "endpointStatusTest ctlV3EndpointHealth error")
}

func certCNAndUsername(cx ctlCtx, noPassword bool) {
	require.NoError(cx.t, authEnable(cx))

	cx.user, cx.pass = "root", "root"
	authSetupTestUser(cx)

	if noPassword {
		require.NoError(cx.t, ctlV3User(cx, []string{"add", "example.com", "--no-password"}, "User example.com created", []string{""}))
	} else {
		require.NoError(cx.t, ctlV3User(cx, []string{"add", "example.com", "--interactive=false"}, "User example.com created", []string{""}))
	}
	require.NoError(cx.t, e2e.SpawnWithExpectWithEnv(append(cx.PrefixArgs(), "role", "add", "test-role-cn"), cx.envMap, expect.ExpectedResponse{Value: "Role test-role-cn created"}))
	require.NoError(cx.t, ctlV3User(cx, []string{"grant-role", "example.com", "test-role-cn"}, "Role test-role-cn is granted to user example.com", nil))

	// grant a new key for CN based user
	require.NoError(cx.t, ctlV3RoleGrantPermission(cx, "test-role-cn", grantingPerm{true, true, "hoo", "", false}))

	// grant a new key for username based user
	require.NoError(cx.t, ctlV3RoleGrantPermission(cx, "test-role", grantingPerm{true, true, "bar", "", false}))

	// try a granted key for CN based user
	cx.user, cx.pass = "", ""
	require.NoError(cx.t, ctlV3Put(cx, "hoo", "bar", ""))

	// try a granted key for username based user
	cx.user, cx.pass = "test-user", "pass"
	require.NoError(cx.t, ctlV3Put(cx, "bar", "bar", ""))

	// try a non-granted key for both of them
	cx.user, cx.pass = "", ""
	require.ErrorContains(cx.t, ctlV3PutFailPerm(cx, "baz", "bar"), "permission denied")

	cx.user, cx.pass = "test-user", "pass"
	require.ErrorContains(cx.t, ctlV3PutFailPerm(cx, "baz", "bar"), "permission denied")
}

func authTestCertCNAndUsername(cx ctlCtx) {
	certCNAndUsername(cx, false)
}

func authTestCertCNAndUsernameNoPassword(cx ctlCtx) {
	certCNAndUsername(cx, true)
}

func ctlV3EndpointHealth(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "endpoint", "health")
	lines := make([]expect.ExpectedResponse, cx.epc.Cfg.ClusterSize)
	for i := range lines {
		lines[i] = expect.ExpectedResponse{Value: "is healthy"}
	}
	return e2e.SpawnWithExpects(cmdArgs, cx.envMap, lines...)
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
