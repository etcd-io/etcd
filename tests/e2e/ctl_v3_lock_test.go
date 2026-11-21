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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3LockWithCmd(t *testing.T) {
	testCtl(t, testLockWithCmd)
}

func testLockWithCmd(cx ctlCtx) {
	// exec command with zero exit code
	echoCmd := []string{"echo"}
	require.NoError(cx.t, ctlV3LockWithCmd(cx, echoCmd, expect.ExpectedResponse{Value: ""}))

	// exec command with non-zero exit code
	code := 3
	awkCmd := []string{"awk", fmt.Sprintf("BEGIN{exit %d}", code)}
	expect := expect.ExpectedResponse{Value: fmt.Sprintf("Error: exit status %d", code)}
	require.ErrorContains(cx.t, ctlV3LockWithCmd(cx, awkCmd, expect), expect.Value)
}

// ctlV3LockWithCmd creates a lock process to exec command.
func ctlV3LockWithCmd(cx ctlCtx, execCmd []string, as ...expect.ExpectedResponse) error {
	// use command as lock name
	cmdArgs := append(cx.PrefixArgs(), "lock", execCmd[0])
	cmdArgs = append(cmdArgs, execCmd...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e2e.SpawnWithExpectsContext(ctx, cmdArgs, cx.envMap, as...)
}