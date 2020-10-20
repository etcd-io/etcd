// Copyright 2016 The etcd Authors
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

package command

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"go.etcd.io/etcd/etcdctl/v3/snapshot"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

const (
	defaultName                     = "default"
	defaultInitialAdvertisePeerURLs = "http://localhost:2380"
)

var (
	restoreCluster      string
	restoreClusterToken string
	restoreDataDir      string
	restoreWalDir       string
	restorePeerURLs     string
	restoreName         string
	skipHashCheck       bool
)

// NewSnapshotCommand returns the cobra command for "snapshot".
func NewSnapshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot <subcommand>",
		Short: "Manages etcd node snapshots",
	}
	cmd.AddCommand(NewSnapshotSaveCommand())
	cmd.AddCommand(NewSnapshotRestoreCommand())
	cmd.AddCommand(newSnapshotStatusCommand())
	return cmd
}

func NewSnapshotSaveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "save <filename>",
		Short: "Stores an etcd node backend snapshot to a given file",
		Run:   snapshotSaveCommandFunc,
	}
}

func newSnapshotStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <filename>",
		Short: "Gets backend snapshot status of a given file",
		Long: `When --write-out is set to simple, this command prints out comma-separated status lists for each endpoint.
The items in the lists are hash, revision, total keys, total size.
`,
		Run: snapshotStatusCommandFunc,
	}
}

func NewSnapshotRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <filename> [options]",
		Short: "Restores an etcd member snapshot to an etcd directory",
		Run:   snapshotRestoreCommandFunc,
	}
	cmd.Flags().StringVar(&restoreDataDir, "data-dir", "", "Path to the data directory")
	cmd.Flags().StringVar(&restoreWalDir, "wal-dir", "", "Path to the WAL directory (use --data-dir if none given)")
	cmd.Flags().StringVar(&restoreCluster, "initial-cluster", initialClusterFromName(defaultName), "Initial cluster configuration for restore bootstrap")
	cmd.Flags().StringVar(&restoreClusterToken, "initial-cluster-token", "etcd-cluster", "Initial cluster token for the etcd cluster during restore bootstrap")
	cmd.Flags().StringVar(&restorePeerURLs, "initial-advertise-peer-urls", defaultInitialAdvertisePeerURLs, "List of this member's peer URLs to advertise to the rest of the cluster")
	cmd.Flags().StringVar(&restoreName, "name", defaultName, "Human-readable name for this member")
	cmd.Flags().BoolVar(&skipHashCheck, "skip-hash-check", false, "Ignore snapshot integrity hash value (required if copied from data directory)")

	return cmd
}

func snapshotSaveCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		err := fmt.Errorf("snapshot save expects one argument")
		ExitWithError(ExitBadArgs, err)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	sp := snapshot.NewV3(lg)
	cfg := mustClientCfgFromCmd(cmd)

	// if user does not specify "--command-timeout" flag, there will be no timeout for snapshot save command
	ctx, cancel := context.WithCancel(context.Background())
	if isCommandTimeoutFlagSet(cmd) {
		ctx, cancel = commandCtx(cmd)
	}
	defer cancel()

	path := args[0]
	if err := sp.Save(ctx, *cfg, path); err != nil {
		ExitWithError(ExitInterrupted, err)
	}
	fmt.Printf("Snapshot saved at %s\n", path)
}

func snapshotStatusCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		err := fmt.Errorf("snapshot status requires exactly one argument")
		ExitWithError(ExitBadArgs, err)
	}
	initDisplayFromCmd(cmd)

	lg, err := zap.NewProduction()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	sp := snapshot.NewV3(lg)
	ds, err := sp.Status(args[0])
	if err != nil {
		ExitWithError(ExitError, err)
	}
	display.DBStatus(ds)
}

func snapshotRestoreCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		err := fmt.Errorf("snapshot restore requires exactly one argument")
		ExitWithError(ExitBadArgs, err)
	}

	dataDir := restoreDataDir
	if dataDir == "" {
		dataDir = restoreName + ".etcd"
	}

	walDir := restoreWalDir
	if walDir == "" {
		walDir = filepath.Join(dataDir, "member", "wal")
	}

	lg, err := zap.NewProduction()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	sp := snapshot.NewV3(lg)

	if err := sp.Restore(snapshot.RestoreConfig{
		SnapshotPath:        args[0],
		Name:                restoreName,
		OutputDataDir:       dataDir,
		OutputWALDir:        walDir,
		PeerURLs:            strings.Split(restorePeerURLs, ","),
		InitialCluster:      restoreCluster,
		InitialClusterToken: restoreClusterToken,
		SkipHashCheck:       skipHashCheck,
	}); err != nil {
		ExitWithError(ExitError, err)
	}
}

func initialClusterFromName(name string) string {
	n := name
	if name == "" {
		n = defaultName
	}
	return fmt.Sprintf("%s=http://localhost:2380", n)
}
