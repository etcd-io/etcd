// Copyright 2022 The etcd Authors
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
//go:build e2e
// +build e2e

package common

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCompletion(t *testing.T) {
	tcs := []struct {
		name  string
		bin   string
		shell string
	}{
		{
			name:  "ETCDCTL",
			bin:   e2e.CtlBinPath,
			shell: "bash",
		},
		{
			name:  "ETCDUTL",
			bin:   e2e.UtlBinPath,
			shell: "bash",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)

			stdout := new(bytes.Buffer)
			completionCmd := exec.Command(tc.bin, "completion", tc.shell)
			completionCmd.Stdout = stdout
			completionCmd.Stderr = os.Stderr
			require.NoError(t, completionCmd.Run())

			filename := fmt.Sprintf("etcdctl-%s.completion", tc.shell)
			require.NoError(t, os.WriteFile(filename, stdout.Bytes(), 0644))

			shellCmd := exec.Command(tc.shell, "-c", "source "+filename)
			require.NoError(t, shellCmd.Run())
		})
	}
}
