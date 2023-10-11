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

	"github.com/google/go-cmp/cmp"
)

func TestEtcdServerProcessConfig(t *testing.T) {
	tcs := []struct {
		name       string
		config     *EtcdProcessClusterConfig
		expectArgs []string
	}{
		{
			name:   "Default",
			config: NewConfig(),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigDefault-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"10000",
			},
		},
		{
			name:   "SnapshotCount",
			config: NewConfig(WithSnapshotCount(42)),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigSnapshotCount-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"42",
			},
		},
		{
			name:   "SnapshotCatchUpEntries",
			config: NewConfig(WithSnapshotCatchUpEntries(12)),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigSnapshotCatchUpEntries-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"10000",
				"--experimental-snapshot-catchup-entries",
				"12",
			},
		},
		{
			name:   "QuotaBackendBytes",
			config: NewConfig(WithQuotaBackendBytes(123)),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigQuotaBackendBytes-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"10000",
				"--quota-backend-bytes",
				"123",
			},
		},
		{
			name:   "CorruptCheck",
			config: NewConfig(WithInitialCorruptCheck(true)),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigCorruptCheck-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"10000",
				"--experimental-initial-corrupt-check",
			},
		},
		{
			name:   "StrictReconfigCheck",
			config: NewConfig(WithStrictReconfigCheck(false)),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigStrictReconfigCheck-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"10000",
				"--strict-reconfig-check=false",
			},
		},
		{
			name:   "CatchUpEntries",
			config: NewConfig(WithSnapshotCatchUpEntries(100)),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigCatchUpEntries-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"10000",
				"--experimental-snapshot-catchup-entries",
				"100",
			},
		},
		{
			name:   "CatchUpEntriesLastVersion",
			config: NewConfig(WithSnapshotCatchUpEntries(100), WithVersion(LastVersion)),
			expectArgs: []string{
				"--name",
				"TestEtcdServerProcessConfigCatchUpEntriesLastVersion-test-0",
				"--listen-client-urls",
				"http://localhost:0",
				"--advertise-client-urls",
				"http://localhost:0",
				"--listen-peer-urls",
				"http://localhost:1",
				"--initial-advertise-peer-urls",
				"http://localhost:1",
				"--initial-cluster-token",
				"new",
				"--data-dir",
				"/tmp/fake/member-0",
				"--snapshot-count",
				"10000",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.BaseDataDirPath = "/tmp/fake"
			if diff := cmp.Diff(tc.config.EtcdServerProcessConfig(t, 0).Args, tc.expectArgs); diff != "" {
				t.Errorf("diff: %s", diff)
			}
		})
	}
}
