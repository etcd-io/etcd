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
	"os"

	"github.com/spf13/cobra"
	v3 "go.etcd.io/etcd/client/v3"
	snapshot "go.etcd.io/etcd/client/v3/snapshot"
	"go.etcd.io/etcd/etcdutl/v3/etcdutl"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.uber.org/zap"
)

const (
	defaultName                     = "default"
	defaultInitialAdvertisePeerURLs = "http://localhost:2380"
	defaultSnapshotKey              = "__do_etcd_snapshot"
)

var (
	restoreCluster      string
	restoreClusterToken string
	restoreDataDir      string
	restoreWalDir       string
	restorePeerURLs     string
	restoreName         string
	skipHashCheck       bool
	spClusterEndpoints  bool
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
	cmd.AddCommand(NewDoSnapshotNowCommand())
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
		Short: "[deprecated] Gets backend snapshot status of a given file",
		Long: `When --write-out is set to simple, this command prints out comma-separated status lists for each endpoint.
The items in the lists are hash, revision, total keys, total size.

Moved to 'etcdctl snapshot status ...'
`,
		Run: snapshotStatusCommandFunc,
	}
}

func NewSnapshotRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <filename> [options]",
		Short: "Restores an etcd member snapshot to an etcd directory",
		Run:   snapshotRestoreCommandFunc,
		Long:  "Moved to `etcdctl snapshot restore ...`\n",
	}
	cmd.Flags().StringVar(&restoreDataDir, "data-dir", "", "Path to the data directory")
	cmd.Flags().StringVar(&restoreWalDir, "wal-dir", "", "Path to the WAL directory (use --data-dir if none given)")
	cmd.Flags().StringVar(&restoreCluster, "initial-cluster", initialClusterFromName(defaultName), "Initial cluster configuration for restore bootstrap")
	cmd.Flags().StringVar(&restoreClusterToken, "initial-cluster-token", "etcd-cluster", "Initial cluster token for the etcd cluster during restore bootstrap")
	cmd.Flags().StringVar(&restorePeerURLs, "initial-advertise-peer-urls", defaultInitialAdvertisePeerURLs, "List of this member's peer URLs to advertise to the rest of the cluster")
	cmd.Flags().StringVar(&restoreName, "name", defaultName, "Human-readable name for this member")
	cmd.Flags().BoolVar(&skipHashCheck, "skip-hash-check", false, "Ignore snapshot integrity hash value (required if copied from data directory)")
	cmd.MarkFlagDirname("data-dir")
	cmd.MarkFlagDirname("wal-dir")

	return cmd
}

func NewDoSnapshotNowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "donow [options]",
		Short: "DoSnapshotNow do an auto snapshot immediately",
		Run:   doSnapshotNowCommandFunc,
	}
	cmd.PersistentFlags().BoolVar(&spClusterEndpoints, "cluster", false, "use all endpoints from the cluster member list")
	return cmd
}

func snapshotSaveCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		err := fmt.Errorf("snapshot save expects one argument")
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	cfg := mustClientCfgFromCmd(cmd)

	// if user does not specify "--command-timeout" flag, there will be no timeout for snapshot save command
	ctx, cancel := context.WithCancel(context.Background())
	if isCommandTimeoutFlagSet(cmd) {
		ctx, cancel = commandCtx(cmd)
	}
	defer cancel()

	path := args[0]
	version, err := snapshot.SaveWithVersion(ctx, lg, *cfg, path)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitInterrupted, err)
	}
	fmt.Printf("Snapshot saved at %s\n", path)
	if version != "" {
		fmt.Printf("Server version %s\n", version)
	}
}

func snapshotStatusCommandFunc(cmd *cobra.Command, args []string) {
	fmt.Fprintf(os.Stderr, "Deprecated: Use `etcdutl snapshot status` instead.\n\n")
	etcdutl.SnapshotStatusCommandFunc(cmd, args)
}

func snapshotRestoreCommandFunc(cmd *cobra.Command, args []string) {
	fmt.Fprintf(os.Stderr, "Deprecated: Use `etcdutl snapshot restore` instead.\n\n")
	etcdutl.SnapshotRestoreCommandFunc(restoreCluster, restoreClusterToken, restoreDataDir, restoreWalDir,
		restorePeerURLs, restoreName, skipHashCheck, args)
}

func doSnapshotNowCommandFunc(cmd *cobra.Command, args []string) {
	c := mustClientFromCmd(cmd)
	var err error
	for _, ep := range endpointsFromClusterForSnapshot(cmd) {
		ctx, cancel := commandCtx(cmd)
		serr := c.DoSnapshotNow(ctx, ep)
		cancel()
		if serr != nil {
			err = serr
			fmt.Fprintf(os.Stderr, "Failed to set doSnapshotNow conf of endpoint %s (%v)\n", ep, serr)
			continue
		}
		fmt.Fprintf(os.Stderr, "Set doSnapshotNow conf passed, endpoint: %s\n", ep)
	}
	if err != nil {
		os.Exit(cobrautl.ExitError)
	}

	// put a key to trigger snapshot
	ctx, cancel := commandCtx(cmd)
	_, err = c.Put(ctx, defaultSnapshotKey, "")
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to doSnapshotNow, err: %v\n", err)
	} else {
		fmt.Println("doSnapshotNow success")
	}
}

func endpointsFromClusterForSnapshot(cmd *cobra.Command) []string {
	if !spClusterEndpoints {
		endpoints, err := cmd.Flags().GetStringSlice("endpoints")
		if err != nil {
			cobrautl.ExitWithError(cobrautl.ExitError, err)
		}
		return endpoints
	}

	sec := secureCfgFromCmd(cmd)
	dt := dialTimeoutFromCmd(cmd)
	ka := keepAliveTimeFromCmd(cmd)
	kat := keepAliveTimeoutFromCmd(cmd)
	eps, err := endpointsFromCmd(cmd)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	// exclude auth for not asking needless password (MemberList() doesn't need authentication)

	cfg, err := newClientCfg(eps, dt, ka, kat, sec, nil)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	c, err := v3.New(*cfg)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	ctx, cancel := commandCtx(cmd)
	defer func() {
		c.Close()
		cancel()
	}()
	membs, err := c.MemberList(ctx)
	if err != nil {
		err = fmt.Errorf("failed to fetch endpoints from etcd cluster member list: %v", err)
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	ret := []string{}
	for _, m := range membs.Members {
		ret = append(ret, m.ClientURLs...)
	}
	return ret
}

func initialClusterFromName(name string) string {
	n := name
	if name == "" {
		n = defaultName
	}
	return fmt.Sprintf("%s=http://localhost:2380", n)
}
