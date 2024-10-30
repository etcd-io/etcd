// Copyright 2024 The etcd Authors
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
	"time"

	"github.com/stretchr/testify/require"
)

func TestNoErrorLogsDuringNormalOperations(t *testing.T) {
	tests := []struct {
		name        string
		clusterSize int
	}{
		{
			name:        "single node cluster",
			clusterSize: 1,
		},
		{
			name:        "three node cluster",
			clusterSize: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			epc, err := newEtcdProcessCluster(t,
				&etcdProcessClusterConfig{
					debug:       true,
					clusterSize: tc.clusterSize,
				},
			)
			require.NoError(t, err)
			defer epc.Close()

			require.Lenf(t, epc.procs, tc.clusterSize, "embedded etcd cluster process count is not as expected")

			// Collect the handle of logs before closing the processes.
			var logHandles []logsExpect
			for i := range tc.clusterSize {
				logHandles = append(logHandles, epc.procs[i].Logs())
			}

			time.Sleep(time.Second)
			err = epc.Close()
			require.NoErrorf(t, err, "closing etcd processes")

			// Now that the processes are closed we can collect all log lines. This must happen after closing, else we
			// might not get all log lines.
			var lines []string
			for _, h := range logHandles {
				lines = append(lines, h.Lines()...)
			}
			require.NotEmptyf(t, lines, "expected at least one log line")

			for _, line := range lines {
				require.NotContainsf(t, line, "ERROR:", "error level log line found: %s", line)
			}
		})
	}
}
