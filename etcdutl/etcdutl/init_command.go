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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/config"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/verify"
)

var (
	initCluster      string
	initClusterToken string
	initDataDir      string
	initWALDir       string
	initPeerURLs     string
	initName         string
	initNoVerify     bool
)

// NewInitCommand returns the cobra command for "init".
func NewInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init --data-dir {output dir} [options]",
		Short: "Initializes a new etcd data directory for a member of a new cluster",
		Long: `Initializes a new etcd data directory, without requiring a snapshot file or a
running etcd server. The produced data directory contains an empty keyspace and
the full initial cluster membership, so the member can afterwards be started
with just --data-dir.

To bootstrap a multi-member cluster, run init once per member with that
member's --name and --initial-advertise-peer-urls and the same
--initial-cluster, then place each produced data directory on its member.
`,
		Run: initCommandFunc,
	}
	cmd.Flags().StringVar(&initDataDir, "data-dir", "", "Path to the output data directory")
	cmd.Flags().StringVar(&initWALDir, "wal-dir", "", "Path to the WAL directory (use --data-dir if none given)")
	cmd.Flags().StringVar(&initCluster, "initial-cluster", initialClusterFromName(defaultName), "Initial cluster configuration for init bootstrap")
	cmd.Flags().StringVar(&initClusterToken, "initial-cluster-token", embed.DefaultInitialClusterToken, "Initial cluster token for the etcd cluster during init bootstrap")
	cmd.Flags().StringVar(&initPeerURLs, "initial-advertise-peer-urls", defaultInitialAdvertisePeerURLs, "List of this member's peer URLs to advertise to the rest of the cluster")
	cmd.Flags().StringVar(&initName, "name", defaultName, "Human-readable name for this member")
	cmd.Flags().Uint64Var(&initialMmapSize, "initial-memory-map-size", initialMmapSize, "Initial memory map size of the database in bytes. It uses the default value if not defined or defined to 0")
	cmd.Flags().BoolVar(&initNoVerify, "no-verify", false, "Skip consistency verification of an already initialized data directory")

	cmd.MarkFlagDirname("data-dir")
	cmd.MarkFlagDirname("wal-dir")

	return cmd
}

func initCommandFunc(_ *cobra.Command, args []string) {
	if len(args) != 0 {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, errors.New("init doesn't take any positional arguments"))
	}
	if err := runInit(initName, initDataDir, initWALDir, initCluster, initClusterToken, initPeerURLs, initialMmapSize, initNoVerify); err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
}

// defaultDataDir is the data directory used when --data-dir is not given.
func defaultDataDir(name string) string {
	return name + ".etcd"
}

func runInit(name, dataDir, walDir, cluster, clusterToken, peerURLs string, mmapSize uint64, noVerify bool) error {
	if dataDir == "" {
		dataDir = defaultDataDir(name)
	}

	if walDir == "" {
		walDir = datadir.ToWALDir(dataDir)
	}

	lg := GetLogger()

	pURLs, err := types.NewURLs(strings.Split(peerURLs, ","))
	if err != nil {
		return err
	}
	ics, err := types.NewURLsMap(cluster)
	if err != nil {
		return err
	}
	srv := config.ServerConfig{
		Logger:              lg,
		Name:                name,
		PeerURLs:            pURLs,
		InitialPeerURLsMap:  ics,
		InitialClusterToken: clusterToken,
	}
	if err = srv.VerifyBootstrap(); err != nil {
		return err
	}

	// Make init idempotent: an already initialized data directory is fine as
	// long as it belongs to a member with the same configuration.
	if fileutil.Exist(dataDir) && !fileutil.DirEmpty(dataDir) {
		return validateExistingDataDir(lg, name, dataDir, walDir, ics, clusterToken, noVerify)
	}

	// Restore requires a snapshot file; synthesize an empty one so the
	// initialized member starts with an empty keyspace.
	seedDir, err := os.MkdirTemp("", "etcdutl-init-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(seedDir)
	seedDB := filepath.Join(seedDir, "db")
	if err := writeEmptySeedDB(lg, seedDB); err != nil {
		return err
	}

	return snapshot.NewV3(lg).Restore(snapshot.RestoreConfig{
		SnapshotPath:        seedDB,
		Name:                name,
		OutputDataDir:       dataDir,
		OutputWALDir:        walDir,
		PeerURLs:            strings.Split(peerURLs, ","),
		InitialCluster:      cluster,
		InitialClusterToken: clusterToken,
		// The synthesized db has no trailing sha256, like a db copied
		// from a data directory.
		SkipHashCheck:   true,
		InitialMmapSize: mmapSize,
	})
}

// validateExistingDataDir checks that an existing data directory was
// initialized for the same member and cluster configuration. The expected
// member ID is derived from the member's peer URLs and the initial cluster
// token, so a token, peer URL, or membership mismatch surfaces as an unknown
// member ID.
func validateExistingDataDir(lg *zap.Logger, name, dataDir, walDir string, ics types.URLsMap, clusterToken string, noVerify bool) error {
	expectedCluster, err := membership.NewClusterFromURLsMap(lg, clusterToken, ics)
	if err != nil {
		return err
	}
	expectedMember := expectedCluster.MemberByName(name) //nolint:staticcheck // See https://github.com/dominikh/go-tools/issues/1698

	dbPath := datadir.ToBackendFileName(dataDir)
	if !fileutil.Exist(dbPath) {
		return fmt.Errorf("data-dir %q is not empty and has no backend database %q; it is not an etcd data directory", dataDir, dbPath)
	}
	if !fileutil.Exist(walDir) || fileutil.DirEmpty(walDir) {
		return fmt.Errorf("data-dir %q is not empty and has no WAL directory %q; it is not an etcd data directory", dataDir, walDir)
	}

	be := backend.NewDefaultBackend(lg, dbPath)
	members, _ := schema.NewMembershipBackend(lg, be).MustReadMembersFromBackend()
	be.Close()

	found := members[expectedMember.ID]
	if found == nil {
		return fmt.Errorf("data-dir %q has no member %q with ID %s; it was initialized with a different --initial-cluster, --initial-cluster-token or --initial-advertise-peer-urls", dataDir, name, expectedMember.ID)
	}
	if found.Name != "" && found.Name != name {
		return fmt.Errorf("data-dir %q member %s has name %q, expected %q; it belongs to a different member", dataDir, expectedMember.ID, found.Name, name)
	}

	switch {
	case noVerify:
		lg.Warn("skipping data consistency verification", zap.String("data-dir", dataDir))
	case walDir != datadir.ToWALDir(dataDir):
		// verify.Verify only supports the default layout with the WAL
		// directory inside the data directory.
		lg.Warn(
			"skipping data consistency verification, not supported with a detached --wal-dir",
			zap.String("data-dir", dataDir),
			zap.String("wal-dir", walDir),
		)
	default:
		if err := verifyDataDir(lg, dataDir); err != nil {
			return fmt.Errorf("data-dir %q failed consistency verification: %w", dataDir, err)
		}
	}

	lg.Info(
		"data directory already initialized for this member, skipping",
		zap.String("data-dir", dataDir),
		zap.String("name", name),
		zap.String("member-id", expectedMember.ID.String()),
	)
	return nil
}

// verifyDataDir runs offline consistency verification of a data directory.
// verify.Verify reports problems as errors but panics on some kinds of
// corruption; recover those into an error so init fails cleanly.
func verifyDataDir(lg *zap.Logger, dataDir string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return verify.Verify(verify.Config{
		Logger:  lg,
		DataDir: dataDir,
	})
}

// writeEmptySeedDB creates an empty backend database with the same buckets a
// freshly bootstrapped member would create, stamped with this binary's storage
// version so the started server sees a current-format db rather than falling
// back to legacy schema-version detection.
func writeEmptySeedDB(lg *zap.Logger, path string) error {
	be := backend.NewDefaultBackend(lg, path)
	defer be.Close()

	tx := be.BatchTx()
	tx.LockOutsideApply()
	for _, bucket := range schema.AllBuckets {
		tx.UnsafeCreateBucket(bucket)
	}

	binaryVersion := semver.New(version.Version)
	storageVersion := semver.Version{Major: binaryVersion.Major, Minor: binaryVersion.Minor}
	schema.UnsafeSetStorageVersion(tx, &storageVersion)
	tx.Unlock()

	be.ForceCommit()
	return nil
}
