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
	"context"
	"fmt"
	"os"
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

type bucketKey struct {
	bucket backend.Bucket
	key    []byte
}

func TestEtctlutlMigrateSuccess(t *testing.T) {
	lastReleaseBinary := e2e.BinPath.EtcdLastRelease
	beforeLastReleaseBinary := e2e.BinPath.EtcdBeforeLastRelease

	tcs := []struct {
		name                 string
		targetVersion        string
		clusterVersion       e2e.ClusterVersion
		targetClusterVersion e2e.ClusterVersion
		force                bool
		cleanWal             bool

		expectLogsSubString  string
		expectStorageVersion *semver.Version

		expectNonFoundKeys []bucketKey
		expectStartError   string
	}{
		{
			name:                 "Upgrade v3.4 to v3.5 should work",
			clusterVersion:       e2e.BeforeLastVersion,
			targetVersion:        "3.5",
			targetClusterVersion: e2e.LastVersion,
		},

		{
			name:                 "Downgrade v3.5 to v3.4 should work",
			clusterVersion:       e2e.LastVersion,
			targetVersion:        "3.4",
			targetClusterVersion: e2e.BeforeLastVersion,
			cleanWal:             true,
			expectLogsSubString:  "updated storage version\t" + `{"new-storage-version": "3.4.0"}`,
			expectNonFoundKeys: []bucketKey{
				{bucket: schema.Meta, key: schema.MetaTermKeyName},
				{bucket: schema.Meta, key: schema.MetaConfStateName},
				{bucket: schema.Cluster, key: schema.ClusterDowngradeKeyName},
			},
		},
		{
			name:                 "Migrate v3.5 to v3.5 is no-op",
			clusterVersion:       e2e.LastVersion,
			targetVersion:        "3.5",
			targetClusterVersion: e2e.LastVersion,
			expectLogsSubString:  "storage version up-to-date\t" + `{"storage-version": "3.5"}`,
		},
		{
			name:                 "Upgrade v3.5 to v3.6 should work",
			clusterVersion:       e2e.LastVersion,
			targetVersion:        "3.6",
			targetClusterVersion: e2e.CurrentVersion,
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Migrate v3.6 to v3.6 is no-op",
			clusterVersion:       e2e.CurrentVersion,
			targetVersion:        "3.6",
			targetClusterVersion: e2e.CurrentVersion,
			expectLogsSubString:  "storage version up-to-date\t" + `{"storage-version": "3.6"}`,
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Downgrade v3.6 to v3.5 should fail until it's implemented",
			clusterVersion:       e2e.CurrentVersion,
			targetVersion:        "3.5",
			targetClusterVersion: e2e.CurrentVersion,
			expectLogsSubString:  "cannot downgrade storage, WAL contains newer entries",
			expectStorageVersion: &version.V3_6,
		},
		{
			name:                 "Downgrade v3.6 to v3.5 should work when wal file doesn't have version entry",
			clusterVersion:       e2e.CurrentVersion,
			targetVersion:        "3.5",
			targetClusterVersion: e2e.LastVersion,
			cleanWal:             true,
			expectLogsSubString:  "updated storage version\t" + `{"new-storage-version": "3.5.0"}`,
			expectNonFoundKeys: []bucketKey{
				{bucket: schema.Meta, key: schema.MetaStorageVersionName},
			},
		},
		{
			name:                 "Upgrade v3.6 to v3.7 with force should work",
			clusterVersion:       e2e.CurrentVersion,
			targetVersion:        "3.7",
			force:                true,
			expectLogsSubString:  "forcefully set storage version\t" + `{"storage-version": "3.7"}`,
			expectStorageVersion: &semver.Version{Major: 3, Minor: 7},
			// we don't have 3.7 binary, try to start with existing and verify error
			targetClusterVersion: e2e.CurrentVersion,
			expectStartError:     "version \\\"3.7.0\\\" is not supported",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)
			lg := zaptest.NewLogger(t)
			// some workflow targets don't run 'release' and last 2 releases won't be downloaded
			// skip test cases when this happens
			if (tc.clusterVersion == e2e.LastVersion || tc.targetClusterVersion == e2e.LastVersion) && !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
				t.Skipf("%q does not exist", lastReleaseBinary)
			}
			if (tc.clusterVersion == e2e.BeforeLastVersion || tc.targetClusterVersion == e2e.BeforeLastVersion) && !fileutil.Exist(e2e.BinPath.EtcdBeforeLastRelease) {
				t.Skipf("%q does not exist", beforeLastReleaseBinary)
			}
			dataDirPath := t.TempDir()
			defer os.RemoveAll(dataDirPath)

			epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
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

			dialTimeout := 10 * time.Second
			prefixArgs := []string{e2e.BinPath.Etcdctl, "--endpoints", strings.Join(epc.EndpointsGRPC(), ","), "--dial-timeout", dialTimeout.String()}

			t.Log("Write keys to ensure wal snapshot is created and all fields are set...")
			for i := 0; i < 10; i++ {
				if err = e2e.SpawnWithExpect(append(prefixArgs, "put", fmt.Sprintf("%d", i), "value"), expect.ExpectedResponse{Value: "OK"}); err != nil {
					t.Fatal(err)
				}
			}

			memberDataDir := epc.Procs[0].Config().DataDirPath
			if tc.cleanWal {
				// migrate command doesn't handle version entries in wal file
				// we'll get an error when starting etcd: "etcdserver/membership: cluster cannot be downgraded (current version: X is lower than determined cluster version: X)"
				// snapshot restore will generate a clean wal file that doesn't have version entries

				t.Log("Taking snapshot...")
				spath := filepath.Join(dataDirPath, "snapshot")
				cmdArgs := append(prefixArgs, "snapshot", "save", spath)
				err = e2e.SpawnWithExpect(cmdArgs, expect.ExpectedResponse{Value: fmt.Sprintf("Snapshot saved at %s", spath)})
				if err != nil {
					t.Fatal(err)
				}

				t.Log("Restoring snapshot...")
				dataDirPath = filepath.Join(dataDirPath, "restored")
				memberDataDir = filepath.Join(dataDirPath, "member-0")
				t.Log("etcdctl restoring the snapshot...")
				err = e2e.SpawnWithExpect([]string{
					e2e.BinPath.Etcdutl, "snapshot", "restore", spath,
					"--name", epc.Procs[0].Config().Name,
					"--initial-cluster", epc.Procs[0].Config().InitialCluster,
					"--initial-cluster-token", epc.Procs[0].Config().InitialToken,
					"--initial-advertise-peer-urls", epc.Procs[0].Config().PeerURL.String(),
					"--data-dir", memberDataDir},
					expect.ExpectedResponse{Value: "added member"})
				if err != nil {
					t.Fatal(err)
				}
			}

			t.Log("Stopping the server...")
			if errC := epc.Close(); errC != nil {
				t.Fatalf("error closing etcd processes (%v)", errC)
			}

			t.Log("etcdutl migrate...")
			args := []string{e2e.BinPath.Etcdutl, "migrate", "--data-dir", memberDataDir, "--target-version", tc.targetVersion}
			if tc.force {
				args = append(args, "--force")
			}
			err = e2e.SpawnWithExpect(args, expect.ExpectedResponse{Value: tc.expectLogsSubString})
			if err != nil {
				if tc.expectLogsSubString != "" {
					require.ErrorContains(t, err, tc.expectLogsSubString)
				} else {
					t.Fatal(err)
				}
			}

			t.Log("verify backend...")
			be := backend.NewDefaultBackend(lg, filepath.Join(memberDataDir, "member/snap/db"))

			ver := schema.ReadStorageVersion(be.ReadTx())
			assert.Equal(t, tc.expectStorageVersion, ver)

			for _, bk := range tc.expectNonFoundKeys {
				_, vs := be.ReadTx().UnsafeRange(bk.bucket, bk.key, nil, 0)
				assert.Zero(t, len(vs))
			}

			be.Close()

			t.Log("Starting target version server...")
			epct, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
				e2e.WithVersion(tc.targetClusterVersion),
				e2e.WithDataDirPath(dataDirPath),
				e2e.WithClusterSize(1),
				e2e.WithKeepDataDir(true),
				// Set low SnapshotCount to ensure wal snapshot is done
				e2e.WithSnapshotCount(1),
			)

			if err != nil {
				if tc.expectStartError != "" && strings.Contains(err.Error(), tc.expectStartError) {
					t.Logf("Found expected start error: %s", tc.expectStartError)
					return
				}
				t.Fatalf("could not start etcd process cluster (%v)", err)
			}

			defer func() {
				if errC := epct.Close(); errC != nil {
					t.Fatalf("error closing etcd processes (%v)", errC)
				}
			}()

			t.Log("Read keys ...")
			for i := 0; i < 10; i++ {
				if err = e2e.SpawnWithExpect(append(prefixArgs, "get", fmt.Sprintf("%d", i)), expect.ExpectedResponse{Value: "value"}); err != nil {
					t.Fatal(err)
				}
			}

		})
	}
}

func TestEtctlutlMigrateError(t *testing.T) {
	tcs := []struct {
		name           string
		targetVersion  string
		clusterVersion e2e.ClusterVersion
		force          bool

		expectLogsSubString  string
		expectStorageVersion *semver.Version
	}{
		{
			name:                "Invalid target version string",
			targetVersion:       "abc",
			expectLogsSubString: `Error: wrong target version format, expected "X.Y", got "abc"`,
		},
		{
			name:                "Invalid target version",
			targetVersion:       "3.a",
			expectLogsSubString: `Error: failed to parse target version: strconv.ParseInt: parsing "a": invalid syntax`,
		},
		{
			name:                "Target with only major version is invalid",
			targetVersion:       "3",
			expectLogsSubString: `Error: wrong target version format, expected "X.Y", got "3"`,
		},
		{
			name:                "Target with patch version is invalid",
			targetVersion:       "3.6.0",
			expectLogsSubString: `Error: wrong target version format, expected "X.Y", got "3.6.0"`,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)

			t.Log("etcdutl migrate...")
			args := []string{e2e.BinPath.Etcdutl, "migrate", "--data-dir", t.TempDir(), "--target-version", tc.targetVersion}
			err := e2e.SpawnWithExpect(args, expect.ExpectedResponse{Value: tc.expectLogsSubString})
			if err != nil {
				if tc.expectLogsSubString != "" {
					require.ErrorContains(t, err, tc.expectLogsSubString)
				} else {
					t.Fatal(err)
				}
			}
		})
	}
}
