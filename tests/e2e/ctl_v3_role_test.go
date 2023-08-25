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

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestCtlV3RoleAddTimeout tests add role with 0 grpc dial timeout while it tolerates dial timeout error.
// This is unique in e2e test
func TestCtlV3RoleAddTimeout(t *testing.T) { testCtl(t, roleAddTest, withDialTimeout(0)) }

func roleAddTest(cx ctlCtx) {
	cmdSet := []struct {
		args        []string
		expectedStr string
	}{
		// Add a role.
		{
			args:        []string{"add", "root"},
			expectedStr: "Role root created",
		},
		// Try adding the same role.
		{
			args:        []string{"add", "root"},
			expectedStr: "role name already exists",
		},
	}

	for i, cmd := range cmdSet {
		if err := ctlV3Role(cx, cmd.args, cmd.expectedStr); err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Fatalf("roleAddTest #%d: ctlV3Role error (%v)", i, err)
			}
		}
	}
}

func ctlV3Role(cx ctlCtx, args []string, expStr string) error {
	cmdArgs := append(cx.PrefixArgs(), "role")
	cmdArgs = append(cmdArgs, args...)

	return e2e.SpawnWithExpectWithEnv(cmdArgs, cx.envMap, expect.ExpectedResponse{Value: expStr})
}

func ctlV3RoleGrantPermission(cx ctlCtx, rolename string, perm grantingPerm) error {
	cmdArgs := append(cx.PrefixArgs(), "role", "grant-permission")
	if perm.prefix {
		cmdArgs = append(cmdArgs, "--prefix")
	} else if len(perm.rangeEnd) == 1 && perm.rangeEnd[0] == '\x00' {
		cmdArgs = append(cmdArgs, "--from-key")
	}

	cmdArgs = append(cmdArgs, rolename)
	cmdArgs = append(cmdArgs, grantingPermToArgs(perm)...)

	proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		return err
	}
	defer proc.Close()

	expStr := fmt.Sprintf("Role %s updated", rolename)
	_, err = proc.Expect(expStr)
	return err
}

func ctlV3RoleRevokePermission(cx ctlCtx, rolename string, key, rangeEnd string, fromKey bool) error {
	cmdArgs := append(cx.PrefixArgs(), "role", "revoke-permission")
	cmdArgs = append(cmdArgs, rolename)
	cmdArgs = append(cmdArgs, key)
	var expStr string
	if len(rangeEnd) != 0 {
		cmdArgs = append(cmdArgs, rangeEnd)
		expStr = fmt.Sprintf("Permission of range [%s, %s) is revoked from role %s", key, rangeEnd, rolename)
	} else if fromKey {
		cmdArgs = append(cmdArgs, "--from-key")
		expStr = fmt.Sprintf("Permission of range [%s, <open ended> is revoked from role %s", key, rolename)
	} else {
		expStr = fmt.Sprintf("Permission of key %s is revoked from role %s", key, rolename)
	}

	proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		return err
	}
	defer proc.Close()
	_, err = proc.Expect(expStr)
	return err
}

type grantingPerm struct {
	read     bool
	write    bool
	key      string
	rangeEnd string
	prefix   bool
}

func grantingPermToArgs(perm grantingPerm) []string {
	permstr := ""

	if perm.read {
		permstr += "read"
	}

	if perm.write {
		permstr += "write"
	}

	if len(permstr) == 0 {
		panic("invalid granting permission")
	}

	if len(perm.rangeEnd) == 0 {
		return []string{permstr, perm.key}
	}

	if len(perm.rangeEnd) == 1 && perm.rangeEnd[0] == '\x00' {
		return []string{permstr, perm.key}
	}

	return []string{permstr, perm.key, perm.rangeEnd}
}
