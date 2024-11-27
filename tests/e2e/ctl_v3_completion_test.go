// Copyright 2021 The etcd Authors
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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCtlV3CompletionBash(t *testing.T) {
	testShellCompletion(t, e2e.BinPath.Etcdctl, "bash")
}

func TestUtlV3CompletionBash(t *testing.T) {
	testShellCompletion(t, e2e.BinPath.Etcdutl, "bash")
}

// testShellCompletion can only run in non-coverage mode. The etcdctl and etcdutl
// built with `-tags cov` mode will show go-test result after each execution, like
//
//	PASS
//	coverage: 0.0% of statements in ./...
//
// Since the PASS is not real command, the `source completion" fails with
// command-not-found error.
func testShellCompletion(t *testing.T, binPath, shellName string) {
	e2e.BeforeTest(t)

	stdout := new(bytes.Buffer)
	completionCmd := exec.Command(binPath, "completion", shellName)
	completionCmd.Stdout = stdout
	completionCmd.Stderr = os.Stderr
	require.NoError(t, completionCmd.Run())

	filename := fmt.Sprintf("etcdctl-%s.completion", shellName)
	require.NoError(t, os.WriteFile(filename, stdout.Bytes(), 0o644))

	shellCmd := exec.Command(shellName, "-c", "source "+filename)
	require.NoError(t, shellCmd.Run())
}
