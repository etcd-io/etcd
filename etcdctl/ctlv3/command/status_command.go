// Copyright 2016 CoreOS, Inc.
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

	v3 "github.com/coreos/etcd/clientv3"
	"github.com/spf13/cobra"
)

// NewStatusCommand returns the cobra command for "Status".
func NewStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "status prints out the statuses of the members with given endpoints.",
		Run:   statusCommandFunc,
	}
}

type statusInfo struct {
	ep   string
	resp *v3.StatusResponse
}

func statusCommandFunc(cmd *cobra.Command, args []string) {
	c := mustClientFromCmd(cmd)

	statusList := []statusInfo{}
	var err error
	for _, ep := range c.Endpoints() {
		ctx, cancel := commandCtx(cmd)
		resp, serr := c.Status(ctx, ep)
		cancel()
		if serr != nil {
			err = serr
			fmt.Fprintf(os.Stderr, "Failed to get the status of endpoint %s (%v)", ep, serr)
			continue
		}
		statusList = append(statusList, statusInfo{ep: ep, resp: resp})
	}

	display.MemberStatus(statusList)

	if err != nil {
		os.Exit(ExitError)
	}
}
