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
	"path/filepath"
	"time"

	"github.com/coreos/etcd/mvcc/backend"
	"github.com/spf13/cobra"
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
	cmd.Flags().StringVar(&defragDataDir, "data-dir", "", "Optional. If present, defragments a data directory not in use by etcd.")
	return cmd
}

func defragCommandFunc(cmd *cobra.Command, args []string) {
	if len(defragDataDir) > 0 {
		err := defragData(defragDataDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to defragment etcd data[%s] (%v)\n", defragDataDir, err)
			os.Exit(ExitError)
		}
		return
	}

	failures := 0
	c := mustClientFromCmd(cmd)
	for _, ep := range c.Endpoints() {
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
		os.Exit(ExitError)
	}
}

func defragData(dataDir string) error {
	var be backend.Backend

	bch := make(chan struct{})
	dbDir := filepath.Join(dataDir, "member", "snap", "db")
	go func() {
		defer close(bch)
		be = backend.NewDefaultBackend(dbDir)

	}()
	select {
	case <-bch:
	case <-time.After(time.Second):
		fmt.Fprintf(os.Stderr, "waiting for etcd to close and release its lock on %q. "+
			"To defrag a running etcd instance, omit --data-dir.\n", dbDir)
		<-bch
	}
	return be.Defrag()
}
