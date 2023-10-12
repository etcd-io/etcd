// Copyright 2023 The etcd Authors
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

	"github.com/stretchr/testify/assert"
)

func TestEtcdServerProcessConfig(t *testing.T) {
	tcs := []struct {
		name                 string
		config               *EtcdProcessClusterConfig
		expectArgsNotContain []string
		expectArgsContain    []string
	}{
		{
			name:   "Default",
			config: NewConfig(),
			expectArgsContain: []string{
				"--listen-client-urls=http://localhost:0",
				"--advertise-client-urls=http://localhost:0",
				"--listen-peer-urls=http://localhost:1",
				"--initial-advertise-peer-urls=http://localhost:1",
				"--initial-cluster-token=new",
				"--snapshot-count=10000",
			},
		},
		{
			name:   "SnapshotCount",
			config: NewConfig(WithSnapshotCount(42)),
			expectArgsContain: []string{
				"--snapshot-count=42",
			},
		},
		{
			name:   "QuotaBackendBytes",
			config: NewConfig(WithQuotaBackendBytes(123)),
			expectArgsContain: []string{
				"--quota-backend-bytes=123",
			},
		},
		{
			name:   "CorruptCheck",
			config: NewConfig(WithInitialCorruptCheck(true)),
			expectArgsContain: []string{
				"--experimental-initial-corrupt-check=true",
			},
		},
		{
			name:   "StrictReconfigCheck",
			config: NewConfig(WithStrictReconfigCheck(false)),
			expectArgsContain: []string{
				"--strict-reconfig-check=false",
			},
		},
		{
			name:   "CatchUpEntries",
			config: NewConfig(WithSnapshotCatchUpEntries(100)),
			expectArgsContain: []string{
				"--experimental-snapshot-catchup-entries=100",
			},
		},
		{
			name:   "CatchUpEntriesLastVersion",
			config: NewConfig(WithSnapshotCatchUpEntries(100), WithVersion(LastVersion)),
			expectArgsNotContain: []string{
				"--experimental-snapshot-catchup-entries=100",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.config.EtcdServerProcessConfig(t, 0).Args
			if len(tc.expectArgsContain) != 0 {
				assert.Subset(t, args, tc.expectArgsContain)
			}
			if len(tc.expectArgsNotContain) != 0 {
				assert.NotSubset(t, args, tc.expectArgsNotContain)
			}
		})
	}
}
