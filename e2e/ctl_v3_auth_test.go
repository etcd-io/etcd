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
	"testing"
)

func TestCtlV3AuthEnable(t *testing.T)   { testCtl(t, authEnableTest) }
func TestCtlV3AuthDisable(t *testing.T)  { testCtl(t, authDisableTest) }
func TestCtlV3AuthWriteKey(t *testing.T) { testCtl(t, authCredWriteKeyTest) }

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

func ctlV3AuthEnable(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "auth", "enable")
	return spawnWithExpect(cmdArgs, "Authentication Enabled")
}

func authDisableTest(cx ctlCtx) {
	if err := ctlV3AuthDisable(cx); err != nil {
		cx.t.Fatalf("authDisableTest ctlV3AuthDisable error (%v)", err)
	}
}

func ctlV3AuthDisable(cx ctlCtx) error {
	cmdArgs := append(cx.PrefixArgs(), "auth", "disable")
	return spawnWithExpect(cmdArgs, "Authentication Disabled")
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

	// confirm root role doesn't grant access to all keys
	if err := ctlV3PutFailPerm(cx, "foo", "bar"); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3GetFailPerm(cx, "foo"); err != nil {
		cx.t.Fatal(err)
	}

	// try invalid user
	cx.user, cx.pass = "a", "b"
	if err := ctlV3PutFailAuth(cx, "foo", "bar"); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put failed
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "a"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// try good user
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Put(cx, "foo", "bar", ""); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put succeeded
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}

	// try bad password
	cx.user, cx.pass = "test-user", "badpass"
	if err := ctlV3PutFailAuth(cx, "foo", "baz"); err != nil {
		cx.t.Fatal(err)
	}
	// confirm put failed
	cx.user, cx.pass = "test-user", "pass"
	if err := ctlV3Get(cx, []string{"foo"}, []kv{{"foo", "bar"}}...); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3PutFailAuth(cx ctlCtx, key, val string) error {
	return spawnWithExpect(append(cx.PrefixArgs(), "put", key, val), "authentication failed")
}

func ctlV3PutFailPerm(cx ctlCtx, key, val string) error {
	return spawnWithExpect(append(cx.PrefixArgs(), "put", key, val), "permission denied")
}

func ctlV3GetFailPerm(cx ctlCtx, key string) error {
	return spawnWithExpect(append(cx.PrefixArgs(), "get", key), "permission denied")
}

func authSetupTestUser(cx ctlCtx) {
	if err := ctlV3User(cx, []string{"add", "test-user", "--interactive=false"}, "User test-user created", []string{"pass"}); err != nil {
		cx.t.Fatal(err)
	}
	if err := spawnWithExpect(append(cx.PrefixArgs(), "role", "add", "test-role"), "Role test-role created"); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3User(cx, []string{"grant-role", "test-user", "test-role"}, "Role test-role is granted to user test-user", nil); err != nil {
		cx.t.Fatal(err)
	}
	cmd := append(cx.PrefixArgs(), "role", "grant-permission", "test-role", "readwrite", "foo")
	if err := spawnWithExpect(cmd, "Role test-role updated"); err != nil {
		cx.t.Fatal(err)
	}
}
