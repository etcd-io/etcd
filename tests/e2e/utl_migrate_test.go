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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.uber.org/zap/zaptest"
)

func TestEtctlutlMigrate(t *testing.T) {
	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"

	tcs := []struct {
		name          string
		targetVersion string
		binary        string
		force         bool

		expectLogsSubString  string
		expectStorageVersion *semver.Version
	}{
		{
			name:                 "Invalid target version string",
			targetVersion:        "abc",
			expectLogsSubString:  `Error: wrong target version format, expected "X.Y", got "abc"`,
			expectStorageVersion: &schema.V3_6,
		},
		{
			name:                 "Invalid target version",
			targetVersion:        "3.a",
			expectLogsSubString:  `Error: failed to parse target version: strconv.ParseInt: parsing "a": invalid syntax`,
			expectStorageVersion: &schema.V3_6,
		},
		{
			name:                 "Target with only major version is invalid",
			targetVersion:        "3",
			expectLogsSubString:  `Error: wrong target version format, expected "X.Y", got "3"`,
			expectStorageVersion: &schema.V3_6,
		},
		{
			name:                 "Target with patch version is invalid",
			targetVersion:        "3.6.0",
			expectLogsSubString:  `Error: wrong target version format, expected "X.Y", got "3.6.0"`,
			expectStorageVersion: &schema.V3_6,
		},
		{
			name:                "Migrate v3.5 to v3.5 is no-op",
			binary:              lastReleaseBinary,
			targetVersion:       "3.5",
			expectLogsSubString: "storage version up-to-date\t" + `{"storage-version": "3.5"}`,
		},
		{
			name:                 "Upgrade v3.5 to v3.6 should work",
			binary:               lastReleaseBinary,
			targetVersion:        "3.6",
			expectStorageVersion: &schema.V3_6,
		},
		{
			name:                 "Migrate v3.6 to v3.6 is no-op",
			targetVersion:        "3.6",
			expectLogsSubString:  "storage version up-to-date\t" + `{"storage-version": "3.6"}`,
			expectStorageVersion: &schema.V3_6,
		},
		{
			name:                 "Downgrade v3.6 to v3.5 should fail until it's implemented",
			targetVersion:        "3.5",
			expectLogsSubString:  "cannot downgrade storage, WAL contains newer entries",
			expectStorageVersion: &schema.V3_6,
		},
		{
			name:                "Downgrade v3.6 to v3.5 with force should work",
			targetVersion:       "3.5",
			force:               true,
			expectLogsSubString: "forcefully cleared storage version",
		},
		{
			name:                 "Upgrade v3.6 to v3.7 with force should work",
			targetVersion:        "3.7",
			force:                true,
			expectLogsSubString:  "forcefully set storage version\t" + `{"storage-version": "3.7"}`,
			expectStorageVersion: &semver.Version{Major: 3, Minor: 7},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)
			lg := zaptest.NewLogger(t)
			if tc.binary != "" && !fileutil.Exist(tc.binary) {
				t.Skipf("%q does not exist", lastReleaseBinary)
			}
			dataDirPath := t.TempDir()

			epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
				ExecPath:     tc.binary,
				DataDirPath:  dataDirPath,
				ClusterSize:  1,
				InitialToken: "new",
				KeepDataDir:  true,
				// Set low SnapshotCount to ensure wal snapshot is done
				SnapshotCount: 1,
			})
			if err != nil {
				t.Fatalf("could not start etcd process cluster (%v)", err)
			}
			defer func() {
				if errC := epc.Close(); errC != nil {
					t.Fatalf("error closing etcd processes (%v)", errC)
				}
			}()

			dialTimeout := 10 * time.Second
			prefixArgs := []string{e2e.CtlBinPath, "--endpoints", strings.Join(epc.EndpointsV3(), ","), "--dial-timeout", dialTimeout.String()}

			t.Log("Write keys to ensure wal snapshot is created and all v3.5 fields are set...")
			for i := 0; i < 10; i++ {
				if err = e2e.SpawnWithExpect(append(prefixArgs, "put", fmt.Sprintf("%d", i), "value"), "OK"); err != nil {
					t.Fatal(err)
				}
			}

			t.Log("Stopping the server...")
			if err = epc.Procs[0].Stop(); err != nil {
				t.Fatal(err)
			}

			t.Log("etcdutl migrate...")
			args := []string{e2e.UtlBinPath, "migrate", "--data-dir", dataDirPath, "--target-version", tc.targetVersion}
			if tc.force {
				args = append(args, "--force")
			}
			err = e2e.SpawnWithExpect(args, tc.expectLogsSubString)
			if err != nil {
				t.Fatal(err)
			}

			t.Log("etcdutl migrate...")
			be := backend.NewDefaultBackend(lg, dataDirPath+"/member/snap/db")
			defer be.Close()

			ver := schema.ReadStorageVersion(be.ReadTx())
			assert.Equal(t, tc.expectStorageVersion, ver)
		})
	}
}
