// Copyright 2017 The etcd Authors
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
	"strconv"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/v3/clientv3"
)

// NewMoveLeaderCommand returns the cobra command for "move-leader".
func NewMoveLeaderCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move-leader <transferee-member-id>",
		Short: "Transfers leadership to another etcd cluster member.",
		Run:   transferLeadershipCommandFunc,
	}
	return cmd
}

// transferLeadershipCommandFunc executes the "compaction" command.
func transferLeadershipCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, fmt.Errorf("move-leader command needs 1 argument"))
	}
	target, err := strconv.ParseUint(args[0], 16, 64)
	if err != nil {
		ExitWithError(ExitBadArgs, err)
	}

	c := mustClientFromCmd(cmd)
	eps := c.Endpoints()
	c.Close()

	ctx, cancel := commandCtx(cmd)

	// find current leader
	var leaderCli *clientv3.Client
	var leaderID uint64
	for _, ep := range eps {
		cfg := clientConfigFromCmd(cmd)
		cfg.endpoints = []string{ep}
		cli := cfg.mustClient()
		resp, serr := cli.Status(ctx, ep)
		if serr != nil {
			ExitWithError(ExitError, serr)
		}

		if resp.Header.GetMemberId() == resp.Leader {
			leaderCli = cli
			leaderID = resp.Leader
			break
		}
		cli.Close()
	}
	if leaderCli == nil {
		ExitWithError(ExitBadArgs, fmt.Errorf("no leader endpoint given at %v", eps))
	}

	var resp *clientv3.MoveLeaderResponse
	resp, err = leaderCli.MoveLeader(ctx, target)
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}

	display.MoveLeader(leaderID, target, *resp)
}
