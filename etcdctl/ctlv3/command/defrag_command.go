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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/etcdutl/v3/etcdutl"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
)

var (
	defragDataDir string
)

// NewDefragCommand returns the cobra command for "Defrag".
func NewDefragCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defrag",
		Short: "Defragments the storage of the etcd members with given endpoints",
		Run:   defragCommandFunc,
	}
	cmd.PersistentFlags().BoolVar(&epClusterEndpoints, "cluster", false, "use all endpoints from the cluster member list")
	cmd.Flags().StringVar(&defragDataDir, "data-dir", "", "Optional. If present, defragments a data directory not in use by etcd.")
	return cmd
}

func defragCommandFunc(cmd *cobra.Command, args []string) {
	if len(defragDataDir) > 0 {
		fmt.Fprintf(os.Stderr, "Use `etcdutl defrag` instead. The --data-dir is going to be decomissioned in v3.6.\n\n")
		err := etcdutl.DefragData(defragDataDir)
		if err != nil {
			cobrautl.ExitWithError(cobrautl.ExitError, err)
		}
		return
	}

	failures := 0
	c := mustClientFromCmd(cmd)
	for _, ep := range endpointsFromCluster(cmd) {
		ctx, cancel := commandCtx(cmd)
		_, err := c.Defragment(ctx, ep)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to defragment etcd member[%s] (%v)\n", ep, err)
			failures++
		} else {
			fmt.Printf("Finished defragmenting etcd member[%s]\n", ep)
		}
	}

	if failures != 0 {
		os.Exit(cobrautl.ExitError)
	}
}
