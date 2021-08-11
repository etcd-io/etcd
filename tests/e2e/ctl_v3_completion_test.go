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
)

func TestCtlV3CompletionBash(t *testing.T) { testShellCompletion(t, ctlBinPath, "bash") }

func TestUtlV3CompletionBash(t *testing.T) { testShellCompletion(t, utlBinPath, "bash") }

func testShellCompletion(t *testing.T, binPath, shellName string) {
	BeforeTest(t)

	stdout := new(bytes.Buffer)
	completionCmd := exec.Command(binPath, "completion", shellName)
	completionCmd.Stdout = stdout
	completionCmd.Stderr = os.Stderr
	require.NoError(t, completionCmd.Run())

	filename := fmt.Sprintf("etcdctl-%s.completion", shellName)
	require.NoError(t, os.WriteFile(filename, stdout.Bytes(), 0644))

	shellCmd := exec.Command(shellName, "-c", "source "+filename)
	require.NoError(t, shellCmd.Run())
}
