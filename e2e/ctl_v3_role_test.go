// Copyright 2016 CoreOS, Inc.
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

import "testing"

func TestCtlV3RoleAdd(t *testing.T)          { testCtl(t, roleAddTest) }
func TestCtlV3RoleAddNoTLS(t *testing.T)     { testCtl(t, roleAddTest, withCfg(configNoTLS)) }
func TestCtlV3RoleAddClientTLS(t *testing.T) { testCtl(t, roleAddTest, withCfg(configClientTLS)) }
func TestCtlV3RoleAddPeerTLS(t *testing.T)   { testCtl(t, roleAddTest, withCfg(configPeerTLS)) }
func TestCtlV3RoleAddTimeout(t *testing.T)   { testCtl(t, roleAddTest, withDialTimeout(0)) }

func roleAddTest(cx ctlCtx) {
	cmdSet := []struct {
		args        []string
		expectedStr string
	}{
		{
			args:        []string{"add", "root"},
			expectedStr: "Role root created",
		},
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

	return spawnWithExpect(cmdArgs, expStr)
}
