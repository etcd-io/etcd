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

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	snapshot "go.etcd.io/etcd/client/v3/snapshot"
	"go.etcd.io/etcd/etcdctl/v3/util"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
)

var snapshotExample = util.Normalize(`
	# Save snapshot to a given file
	etcdctl snapshot save /backup/etcd-snapshot.db

	# Get snapshot from given address and save it to file
	etcdctl snapshot save --endpoints=127.0.0.1:3000 /backup/etcd-snapshot.db 
	
	# Get snapshot from given address with certificates
	etcdctl --endpoints=https://127.0.0.1:2379 --cacert=/etc/etcd/ca.crt --cert=/etc/etcd/etcd.crt --key=/etc/etcd/etcd.key snapshot save /backup/etcd-snapshot.db

	# Get snapshot wih certain user and password
	etcdctl --user=root --password=password123 snapshot save /backup/etcd-snapshot.db

	# Get snapshot from given address with timeout
	etcdctl --endpoints=https://127.0.0.1:2379 --dial-timeout=20s snapshot save /backup/etcd-snapshot.db

	# Save snapshot with desirable time format
	etcdctl snapshot save /mnt/backup/etcd/backup_$(date +%Y%m%d_%H%M%S).db`)

// NewSnapshotCommand returns the cobra command for "snapshot".
func NewSnapshotCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "snapshot <subcommand>",
		Short:   "Manages etcd node snapshots",
		Example: snapshotExample,
	}
	cmd.AddCommand(NewSnapshotSaveCommand())
	return cmd
}

func NewSnapshotSaveCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "save <filename>",
		Short:   "Stores an etcd node backend snapshot to a given file",
		Run:     snapshotSaveCommandFunc,
		Example: snapshotExample,
	}
}

func snapshotSaveCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		err := fmt.Errorf("snapshot save expects one argument <filename>")
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}

	lg, err := logutil.CreateDefaultZapLogger(zap.InfoLevel)
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
