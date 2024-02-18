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
	"testing"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3UserAdd(t *testing.T)      { testCtl(t, userAddTest) }
func TestCtlV3UserAddNoTLS(t *testing.T) { testCtl(t, userAddTest, withCfg(*e2e.NewConfigNoTLS())) }
func TestCtlV3UserAddClientTLS(t *testing.T) {
	testCtl(t, userAddTest, withCfg(*e2e.NewConfigClientTLS()))
}
func TestCtlV3UserAddPeerTLS(t *testing.T) { testCtl(t, userAddTest, withCfg(*e2e.NewConfigPeerTLS())) }
func TestCtlV3UserAddTimeout(t *testing.T) { testCtl(t, userAddTest, withDialTimeout(0)) }
func TestCtlV3UserAddClientAutoTLS(t *testing.T) {
	testCtl(t, userAddTest, withCfg(*e2e.NewConfigClientAutoTLS()))
}
func TestCtlV3UserList(t *testing.T)      { testCtl(t, userListTest) }
func TestCtlV3UserListNoTLS(t *testing.T) { testCtl(t, userListTest, withCfg(*e2e.NewConfigNoTLS())) }
func TestCtlV3UserListClientTLS(t *testing.T) {
	testCtl(t, userListTest, withCfg(*e2e.NewConfigClientTLS()))
}
func TestCtlV3UserListPeerTLS(t *testing.T) {
	testCtl(t, userListTest, withCfg(*e2e.NewConfigPeerTLS()))
}
func TestCtlV3UserListClientAutoTLS(t *testing.T) {
	testCtl(t, userListTest, withCfg(*e2e.NewConfigClientAutoTLS()))
}
func TestCtlV3UserDelete(t *testing.T)      { testCtl(t, userDelTest) }
func TestCtlV3UserDeleteNoTLS(t *testing.T) { testCtl(t, userDelTest, withCfg(*e2e.NewConfigNoTLS())) }
func TestCtlV3UserDeleteClientTLS(t *testing.T) {
	testCtl(t, userDelTest, withCfg(*e2e.NewConfigClientTLS()))
}
func TestCtlV3UserDeletePeerTLS(t *testing.T) {
	testCtl(t, userDelTest, withCfg(*e2e.NewConfigPeerTLS()))
}
func TestCtlV3UserDeleteClientAutoTLS(t *testing.T) {
	testCtl(t, userDelTest, withCfg(*e2e.NewConfigClientAutoTLS()))
}
func TestCtlV3UserPasswd(t *testing.T) { testCtl(t, userPasswdTest) }
func TestCtlV3UserPasswdNoTLS(t *testing.T) {
	testCtl(t, userPasswdTest, withCfg(*e2e.NewConfigNoTLS()))
}
func TestCtlV3UserPasswdClientTLS(t *testing.T) {
	testCtl(t, userPasswdTest, withCfg(*e2e.NewConfigClientTLS()))
}
func TestCtlV3UserPasswdPeerTLS(t *testing.T) {
	testCtl(t, userPasswdTest, withCfg(*e2e.NewConfigPeerTLS()))
}
func TestCtlV3UserPasswdClientAutoTLS(t *testing.T) {
	testCtl(t, userPasswdTest, withCfg(*e2e.NewConfigClientAutoTLS()))
}

type userCmdDesc struct {
	args        []string
	expectedStr string
	stdIn       []string
}

func userAddTest(cx ctlCtx) {
	cmdSet := []userCmdDesc{
		// Adds a user name.
		{
			args:        []string{"add", "username", "--interactive=false"},
			expectedStr: "User username created",
			stdIn:       []string{"password"},
		},
		// Adds a user name using the usertest:password syntax.
		{
			args:        []string{"add", "usertest:password"},
			expectedStr: "User usertest created",
			stdIn:       []string{},
		},
		// Tries to add a user with empty username.
		{
			args:        []string{"add", ":password"},
			expectedStr: "empty user name is not allowed",
			stdIn:       []string{},
		},
		// Tries to add a user name that already exists.
		{
			args:        []string{"add", "username", "--interactive=false"},
			expectedStr: "user name already exists",
			stdIn:       []string{"password"},
		},
		// Adds a user without password.
		{
			args:        []string{"add", "userwopasswd", "--no-password"},
			expectedStr: "User userwopasswd created",
			stdIn:       []string{},
		},
	}

	for i, cmd := range cmdSet {
		if err := ctlV3User(cx, cmd.args, cmd.expectedStr, cmd.stdIn); err != nil {
			if cx.dialTimeout > 0 && !isGRPCTimedout(err) {
				cx.t.Fatalf("userAddTest #%d: ctlV3User error (%v)", i, err)
			}
		}
	}
}

func userListTest(cx ctlCtx) {
	cmdSet := []userCmdDesc{
		// Adds a user name.
		{
			args:        []string{"add", "username", "--interactive=false"},
			expectedStr: "User username created",
			stdIn:       []string{"password"},
		},
		// List user name
		{
			args:        []string{"list"},
			expectedStr: "username",
		},
	}

	for i, cmd := range cmdSet {
		if err := ctlV3User(cx, cmd.args, cmd.expectedStr, cmd.stdIn); err != nil {
			cx.t.Fatalf("userListTest #%d: ctlV3User error (%v)", i, err)
		}
	}
}

func userDelTest(cx ctlCtx) {
	cmdSet := []userCmdDesc{
		// Adds a user name.
		{
			args:        []string{"add", "username", "--interactive=false"},
			expectedStr: "User username created",
			stdIn:       []string{"password"},
		},
		// Deletes the user name just added.
		{
			args:        []string{"delete", "username"},
			expectedStr: "User username deleted",
		},
		// Deletes a user name that is not present.
		{
			args:        []string{"delete", "username"},
			expectedStr: "user name not found",
		},
	}

	for i, cmd := range cmdSet {
		if err := ctlV3User(cx, cmd.args, cmd.expectedStr, cmd.stdIn); err != nil {
			cx.t.Fatalf("userDelTest #%d: ctlV3User error (%v)", i, err)
		}
	}
}

func userPasswdTest(cx ctlCtx) {
	cmdSet := []userCmdDesc{
		// Adds a user name.
		{
			args:        []string{"add", "username", "--interactive=false"},
			expectedStr: "User username created",
			stdIn:       []string{"password"},
		},
		// Changes the password.
		{
			args:        []string{"passwd", "username", "--interactive=false"},
			expectedStr: "Password updated",
			stdIn:       []string{"password1"},
		},
	}

	for i, cmd := range cmdSet {
		if err := ctlV3User(cx, cmd.args, cmd.expectedStr, cmd.stdIn); err != nil {
			cx.t.Fatalf("userPasswdTest #%d: ctlV3User error (%v)", i, err)
		}
	}
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
