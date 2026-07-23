// Copyright 2026 The etcd Authors
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

package etcdutl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/verify"
)

func TestInit(t *testing.T) {
	type testCase struct {
		name            string
		memberName      string
		cluster         string
		clusterToken    string
		peerURLs        string
		dataDir         string
		walDir          string
		setupFunc       func(t *testing.T, dataDir string)
		noVerify        bool
		expectedMembers []string
		expectError     bool
		errorRegexp     string
	}

	testCases := []testCase{
		{
			name:            "single member",
			memberName:      "etcd-0",
			cluster:         "etcd-0=http://etcd-0:2380",
			peerURLs:        "http://etcd-0:2380",
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:            "single member with custom cluster token",
			memberName:      "etcd-0",
			cluster:         "etcd-0=http://etcd-0:2380",
			clusterToken:    "custom-token",
			peerURLs:        "http://etcd-0:2380",
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:            "multi member",
			memberName:      "etcd-1",
			cluster:         "etcd-0=http://etcd-0:2380,etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380",
			peerURLs:        "http://etcd-1:2380",
			expectedMembers: []string{"etcd-0", "etcd-1", "etcd-2"},
			expectError:     false,
		},
		{
			name:            "member with multiple peer urls",
			memberName:      "etcd-0",
			cluster:         "etcd-0=http://etcd-0:2380,etcd-0=http://etcd-0:2381",
			peerURLs:        "http://etcd-0:2380,http://etcd-0:2381",
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:            "custom wal dir",
			memberName:      "etcd-0",
			cluster:         "etcd-0=http://etcd-0:2380",
			peerURLs:        "http://etcd-0:2380",
			walDir:          "custom-wal",
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:       "existing empty data dir",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, os.MkdirAll(dataDir, 0o700))
			},
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:        "member name missing from initial cluster",
			memberName:  "etcd-1",
			cluster:     "etcd-0=http://etcd-0:2380",
			peerURLs:    "http://etcd-0:2380",
			expectError: true,
			errorRegexp: `couldn't find local name "[^"]+" in the initial cluster configuration`,
		},
		{
			name:        "advertised peer urls not in initial cluster",
			memberName:  "etcd-0",
			cluster:     "etcd-0=http://etcd-0:2380",
			peerURLs:    "http://etcd-0:12380",
			expectError: true,
			errorRegexp: `--initial-cluster has [^ ]+ but missing from --initial-advertise-peer-urls=[^ ]+`,
		},
		{
			name:        "invalid peer url",
			memberName:  "etcd-0",
			cluster:     "etcd-0=http://etcd-0:2380",
			peerURLs:    "://bad-url",
			expectError: true,
			errorRegexp: `parse "[^"]+": missing protocol scheme`,
		},
		{
			name:       "non-etcd data dir",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, os.MkdirAll(dataDir, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dataDir, "keep"), []byte("data"), 0o600))
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" is not empty and has no backend database "[^"]+"; it is not an etcd data directory`,
		},
		{
			name:       "init is idempotent",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
			},
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:         "reinit with different cluster token",
			memberName:   "etcd-0",
			cluster:      "etcd-0=http://etcd-0:2380",
			clusterToken: "other-token",
			peerURLs:     "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" has no member "etcd-0" with ID [0-9a-f]+; it was initialized with a different --initial-cluster, --initial-cluster-token or --initial-advertise-peer-urls`,
		},
		{
			name:       "reinit with different peer urls",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:12380",
			peerURLs:   "http://etcd-0:12380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" has no member "etcd-0" with ID [0-9a-f]+; it was initialized with a different --initial-cluster, --initial-cluster-token or --initial-advertise-peer-urls`,
		},
		{
			name:       "reinit with initial cluster in different order",
			memberName: "etcd-1",
			cluster:    "etcd-2=http://etcd-2:2380,etcd-0=http://etcd-0:2380,etcd-1=http://etcd-1:2380",
			peerURLs:   "http://etcd-1:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-1", dataDir, "",
					"etcd-0=http://etcd-0:2380,etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380", embed.DefaultInitialClusterToken,
					"http://etcd-1:2380", backend.InitialMmapSize, false))
			},
			expectedMembers: []string{"etcd-0", "etcd-1", "etcd-2"},
			expectError:     false,
		},
		{
			name:       "reinit with peer urls in different order",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2381,etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2381,http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380,etcd-0=http://etcd-0:2381", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380,http://etcd-0:2381", backend.InitialMmapSize, false))
			},
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:       "reinit with custom wal dir",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			walDir:     "custom-wal",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "custom-wal",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
			},
			expectedMembers: []string{"etcd-0"},
			expectError:     false,
		},
		{
			name:       "reinit missing custom wal dir flag",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "custom-wal",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" is not empty and has no WAL directory "[^"]+"; it is not an etcd data directory`,
		},
		{
			name:       "data dir with backend but no wal",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
				require.NoError(t, os.RemoveAll(datadir.ToWALDir(dataDir)))
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" is not empty and has no WAL directory "[^"]+"; it is not an etcd data directory`,
		},
		{
			name:       "data dir with wal but no backend",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
				require.NoError(t, os.RemoveAll(datadir.ToSnapDir(dataDir)))
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" is not empty and has no backend database "[^"]+"; it is not an etcd data directory`,
		},
		{
			name:       "reinit as unknown member against existing data dir",
			memberName: "etcd-99",
			cluster:    "etcd-0=http://etcd-0:2380,etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380",
			peerURLs:   "http://etcd-99:2380",
			dataDir:    "member.etcd",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380,etcd-1=http://etcd-1:2380,etcd-2=http://etcd-2:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
			},
			expectError: true,
			errorRegexp: `couldn't find local name "etcd-99" in the initial cluster configuration`,
		},
		{
			name:       "reinit with corrupted wal fails verification",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
				corruptWAL(t, dataDir)
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" failed consistency verification`,
		},
		{
			name:       "reinit with corrupted wal passes with no-verify",
			memberName: "etcd-0",
			cluster:    "etcd-0=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
				corruptWAL(t, dataDir)
			},
			noVerify:    true,
			expectError: false,
		},
		{
			name:       "reinit with different member name",
			memberName: "etcd-1",
			cluster:    "etcd-1=http://etcd-0:2380",
			peerURLs:   "http://etcd-0:2380",
			dataDir:    "member.etcd",
			setupFunc: func(t *testing.T, dataDir string) {
				require.NoError(t, runInit("etcd-0", dataDir, "",
					"etcd-0=http://etcd-0:2380", embed.DefaultInitialClusterToken,
					"http://etcd-0:2380", backend.InitialMmapSize, false))
			},
			expectError: true,
			errorRegexp: `data-dir "[^"]+" member [0-9a-f]+ has name "etcd-0", expected "etcd-1"; it belongs to a different member`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			dataDir := tc.dataDir
			if dataDir == "" {
				dataDir = defaultDataDir(tc.memberName)
			}

			if tc.setupFunc != nil {
				tc.setupFunc(t, dataDir)
			}

			clusterToken := tc.clusterToken
			if clusterToken == "" {
				clusterToken = embed.DefaultInitialClusterToken
			}

			err := runInit(tc.memberName, tc.dataDir, tc.walDir, tc.cluster, clusterToken, tc.peerURLs, backend.InitialMmapSize, tc.noVerify)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorRegexp != "" {
					require.Regexp(t, tc.errorRegexp, err.Error())
				}
				return
			}

			require.NoError(t, err)
			if tc.expectedMembers != nil {
				assertInitializedDataDir(t, dataDir, tc.walDir, tc.expectedMembers)
			}
		})
	}
}

// corruptWAL replaces the WAL files of an initialized data directory with
// garbage, so that offline consistency verification fails while the
// membership stored in the backend database stays intact.
func corruptWAL(t *testing.T, dataDir string) {
	t.Helper()
	walDir := datadir.ToWALDir(dataDir)
	entries, err := os.ReadDir(walDir)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NoError(t, os.Remove(filepath.Join(walDir, entry.Name())))
	}
	require.NoError(t, os.WriteFile(filepath.Join(walDir, "0000000000000000-0000000000000000.wal"), []byte("garbage"), 0o600))
}

// assertInitializedDataDir checks that dataDir contains a backend db with all
// buckets and the current storage version, membership for all expected
// members, and a WAL and snapshot that pass offline verification.
func assertInitializedDataDir(t *testing.T, dataDir, walDir string, memberNames []string) {
	t.Helper()
	lg := zap.NewNop()

	if walDir == "" {
		walDir = datadir.ToWALDir(dataDir)
	}
	walFiles, err := filepath.Glob(filepath.Join(walDir, "*.wal"))
	require.NoError(t, err)
	require.NotEmpty(t, walFiles)

	dbPath := datadir.ToBackendFileName(dataDir)
	require.FileExists(t, dbPath)

	db, err := bolt.Open(dbPath, 0o400, &bolt.Options{ReadOnly: true})
	require.NoError(t, err)
	expectedVersion := semver.New(version.Version)
	require.NoError(t, db.View(func(tx *bolt.Tx) error {
		for _, bucket := range schema.AllBuckets {
			require.NotNilf(t, tx.Bucket(bucket.Name()), "bucket %q is missing", bucket)
		}
		storageVersion := schema.ReadStorageVersionFromSnapshot(tx)
		require.NotNil(t, storageVersion)
		require.Equal(t, semver.Version{Major: expectedVersion.Major, Minor: expectedVersion.Minor}, *storageVersion)
		return nil
	}))
	require.NoError(t, db.Close())

	be := backend.NewDefaultBackend(lg, dbPath)
	members, removed := schema.NewMembershipBackend(lg, be).MustReadMembersFromBackend()
	be.Close()
	require.Empty(t, removed)
	var names []string
	for _, m := range members {
		names = append(names, m.Name)
	}
	require.ElementsMatch(t, memberNames, names)

	// verify.Verify only supports the default layout with the WAL directory
	// inside the data directory.
	if walDir == datadir.ToWALDir(dataDir) {
		require.NoError(t, verify.Verify(verify.Config{
			ExactIndex: true,
			Logger:     lg,
			DataDir:    dataDir,
		}))
	}
}
