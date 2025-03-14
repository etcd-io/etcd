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

//go:build !cluster_proxy

package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestEtctlutlMigrate(t *testing.T) {
	lastReleaseBinary := e2e.BinPath.EtcdLastRelease

	tcs := []struct {
		name           string
		targetVersion  string
		clusterVersion e2e.ClusterVersion
		clusterSize    int
		force          bool

		expectLogsSubString  string
		expectStorageVersion *semver.Version
	}{
		{
			name:                 "Invalid target version string",
			targetVersion:        "abc",
			clusterSize:          1,
			expectLogsSubString:  `Error: wrong target version format, expected "X.Y", got "abc"`,
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Invalid target version",
			targetVersion:        "3.a",
			clusterSize:          1,
			expectLogsSubString:  `Error: failed to parse target version: strconv.ParseInt: parsing "a": invalid syntax`,
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Target with only major version is invalid",
			targetVersion:        "3",
			clusterSize:          1,
			expectLogsSubString:  `Error: wrong target version format, expected "X.Y", got "3"`,
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Target with patch version is invalid",
			targetVersion:        "3.6.0",
			clusterSize:          1,
			expectLogsSubString:  `Error: wrong target version format, expected "X.Y", got "3.6.0"`,
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                "Migrate v3.5 to v3.5 is no-op",
			clusterVersion:      e2e.LastVersion,
			clusterSize:         1,
			targetVersion:       "3.5",
			expectLogsSubString: "storage version up-to-date\t" + `{"storage-version": "3.5"}`,
		},
		{
			name:                 "Upgrade 1 member cluster from v3.5 to v3.6 should work",
			clusterVersion:       e2e.LastVersion,
			clusterSize:          1,
			targetVersion:        "3.6",
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Upgrade 3 member cluster from v3.5 to v3.6 should work",
			clusterVersion:       e2e.LastVersion,
			clusterSize:          3,
			targetVersion:        "3.6",
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Migrate v3.6 to v3.6 is no-op",
			targetVersion:        "3.6",
			clusterSize:          1,
			expectLogsSubString:  "storage version up-to-date\t" + `{"storage-version": "3.6"}`,
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Downgrade 1 member cluster from v3.6 to v3.5 should work",
			targetVersion:        "3.5",
			clusterSize:          1,
			expectLogsSubString:  "updated storage version",
			expectStorageVersion: nil, // 3.5 doesn't have the field `storageVersion`, so it returns nil.
		},
		{
			name:                 "Downgrade 3 member cluster from v3.6 to v3.5 should work",
			targetVersion:        "3.5",
			clusterSize:          3,
			expectLogsSubString:  "updated storage version",
			expectStorageVersion: nil, // 3.5 doesn't have the field `storageVersion`, so it returns nil.
		},
		{
			name:                 "Upgrade v3.6 to v3.7 with force should work",
			targetVersion:        "3.7",
			clusterSize:          1,
			force:                true,
			expectLogsSubString:  "forcefully set storage version\t" + `{"storage-version": "3.7"}`,
			expectStorageVersion: &semver.Version{Major: 3, Minor: 7},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)
			lg := zaptest.NewLogger(t)
			if tc.clusterVersion != e2e.CurrentVersion && !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
				t.Skipf("%q does not exist", lastReleaseBinary)
			}
			dataDirPath := t.TempDir()

			epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
				e2e.WithVersion(tc.clusterVersion),
				e2e.WithDataDirPath(dataDirPath),
				e2e.WithClusterSize(1),
				e2e.WithKeepDataDir(true),
				// Set low SnapshotCount to ensure wal snapshot is done
				e2e.WithSnapshotCount(1),
			)
			if err != nil {
				t.Fatalf("could not start etcd process cluster (%v)", err)
			}
			defer func() {
				if errC := epc.Close(); errC != nil {
					t.Fatalf("error closing etcd processes (%v)", errC)
				}
			}()

			dialTimeout := 10 * time.Second
			prefixArgs := []string{e2e.BinPath.Etcdctl, "--endpoints", strings.Join(epc.EndpointsGRPC(), ","), "--dial-timeout", dialTimeout.String()}

			t.Log("Write keys to ensure wal snapshot is created and all v3.5 fields are set...")
			for i := 0; i < 10; i++ {
				require.NoError(t, e2e.SpawnWithExpect(append(prefixArgs, "put", fmt.Sprintf("%d", i), "value"), expect.ExpectedResponse{Value: "OK"}))
			}

			t.Log("Stopping the the members")
			for i := 0; i < len(epc.Procs); i++ {
				t.Logf("Stopping server %d: %v", i, epc.Procs[i].EndpointsGRPC())
				err = epc.Procs[i].Stop()
				require.NoError(t, err)
			}

			t.Log("etcdutl migrate all members")
			for i := 0; i < len(epc.Procs); i++ {
				t.Logf("etcdutl migrate member %d: %v", i, epc.Procs[i].EndpointsGRPC())
				memberDataDir := epc.Procs[i].Config().DataDirPath
				args := []string{e2e.BinPath.Etcdutl, "migrate", "--data-dir", memberDataDir, "--target-version", tc.targetVersion}
				if tc.force {
					args = append(args, "--force")
				}
				err = e2e.SpawnWithExpect(args, expect.ExpectedResponse{Value: tc.expectLogsSubString})
				if err != nil && tc.expectLogsSubString != "" {
					require.ErrorContains(t, err, tc.expectLogsSubString)
				} else {
					require.NoError(t, err)
				}

				be := backend.NewDefaultBackend(lg, filepath.Join(memberDataDir, "member/snap/db"))
				ver := schema.ReadStorageVersion(be.ReadTx())
				assert.Equal(t, tc.expectStorageVersion, ver)
				be.Close()
			}
		})
	}
}
