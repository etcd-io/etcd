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
	"fmt"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
)

func TestEtcdServerProcessConfig(t *testing.T) {
	v3_5_12 := semver.Version{Major: 3, Minor: 5, Patch: 12}
	v3_5_14 := semver.Version{Major: 3, Minor: 5, Patch: 14}
	tcs := []struct {
		name                 string
		config               *EtcdProcessClusterConfig
		expectArgsEquals     []string
		expectArgsNotContain []string
		expectArgsContain    []string
		mockBinaryVersion    *semver.Version
	}{
		{
			name:   "Default",
			config: NewConfig(WithDataDirPath("/tmp/a")),
			expectArgsEquals: []string{
				"--name=TestEtcdServerProcessConfigDefault-test-0",
				"--listen-client-urls=http://localhost:0",
				"--advertise-client-urls=http://localhost:0",
				"--listen-peer-urls=http://localhost:1",
				"--initial-advertise-peer-urls=http://localhost:1",
				"--initial-cluster-token=new",
				"--data-dir",
				"/tmp/a/member-0",
				"--snapshot-count=10000",
				"--initial-cluster-token=new",
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
			mockBinaryVersion: &v3_5_14,
		},
		{
			name:   "CatchUpEntriesNoVersion",
			config: NewConfig(WithSnapshotCatchUpEntries(100), WithVersion(LastVersion)),
			expectArgsNotContain: []string{
				"--snapshot-catchup-entries=100",
			},
		},
		{
			name:   "CatchUpEntriesOldVersion",
			config: NewConfig(WithSnapshotCatchUpEntries(100), WithVersion(LastVersion)),
			expectArgsNotContain: []string{
				"--snapshot-catchup-entries=100",
			},
			mockBinaryVersion: &v3_5_12,
		},
		{
			name:   "ClientHTTPSeparate",
			config: NewConfig(WithClientHTTPSeparate(true)),
			expectArgsContain: []string{
				"--listen-client-http-urls=http://localhost:4",
			},
		},
		{
			name:   "ForceNewCluster",
			config: NewConfig(WithForceNewCluster(true)),
			expectArgsContain: []string{
				"--force-new-cluster=true",
			},
		},
		{
			name:   "EnableV2",
			config: NewConfig(WithEnableV2(true)),
			expectArgsContain: []string{
				"--enable-v2=true",
			},
		},
		{
			name:   "MetricsURL",
			config: NewConfig(WithMetricsURLScheme("http")),
			expectArgsContain: []string{
				"--listen-metrics-urls=http://localhost:2",
			},
		},
		{
			name:   "Discovery",
			config: NewConfig(WithDiscovery("123")),
			expectArgsContain: []string{
				"--discovery=123",
			},
		},
		{
			name:   "ClientTLS",
			config: NewConfig(WithClientConnType(ClientTLS)),
			expectArgsContain: []string{
				"--cert-file",
				"--key-file",
				"--trusted-ca-file",
			},
			expectArgsNotContain: []string{
				"--auto-tls",
				"--client-cert-auth",
			},
		},
		{
			name:   "ClientTLSCA",
			config: NewConfig(WithClientConnType(ClientTLS), WithClientCertAuthority(true)),
			expectArgsContain: []string{
				"--cert-file",
				"--key-file",
				"--trusted-ca-file",
				"--client-cert-auth",
			},
			expectArgsNotContain: []string{
				"--auto-tls",
			},
		},
		{
			name:   "ClientAutoTLS",
			config: NewConfig(WithClientConnType(ClientTLS), WithClientAutoTLS(true)),
			expectArgsContain: []string{
				"--auto-tls",
			},
			expectArgsNotContain: []string{
				"--cert-file",
				"--key-file",
				"--trusted-ca-file",
				"--client-cert-auth",
			},
		},
		{
			name:   "PeerTLS",
			config: NewConfig(WithIsPeerTLS(true)),
			expectArgsContain: []string{
				"--peer-cert-file",
				"--peer-key-file",
				"--peer-trusted-ca-file",
			},
			expectArgsNotContain: []string{
				"--peer-auto-tls",
				"--peer-client-cert-auth",
			},
		},
		{
			name:   "PeerAutoTLS",
			config: NewConfig(WithIsPeerTLS(true), WithIsPeerAutoTLS(true)),
			expectArgsContain: []string{
				"--peer-auto-tls",
			},
			expectArgsNotContain: []string{
				"--peer-cert-file",
				"--peer-key-file",
				"--peer-trusted-ca-file",
				"--peer-client-cert-auth",
			},
		},
		{
			name:   "RevokeCerts",
			config: NewConfig(WithClientRevokeCerts(true)),
			expectArgsContain: []string{
				"--client-crl-file",
				"--client-cert-auth",
			},
		},
		{
			name:   "CipherSuites",
			config: NewConfig(WithCipherSuites([]string{"a", "b"})),
			expectArgsContain: []string{
				"--cipher-suites",
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var mockGetVersionFromBinary func(binaryPath string) (*semver.Version, error)
			if tc.mockBinaryVersion == nil {
				mockGetVersionFromBinary = func(binaryPath string) (*semver.Version, error) {
					return nil, fmt.Errorf("could not get binary version")
				}
			} else {
				mockGetVersionFromBinary = func(binaryPath string) (*semver.Version, error) {
					return tc.mockBinaryVersion, nil
				}
			}
			setGetVersionFromBinary(t, mockGetVersionFromBinary)
			args := tc.config.EtcdServerProcessConfig(t, 0).Args
			if len(tc.expectArgsEquals) != 0 {
				assert.Equal(t, args, tc.expectArgsEquals)
			}
			if len(tc.expectArgsContain) != 0 {
				assert.Subset(t, args, tc.expectArgsContain)
			}
			if len(tc.expectArgsNotContain) != 0 {
				assert.NotSubset(t, args, tc.expectArgsNotContain)
			}
		})
	}
}
