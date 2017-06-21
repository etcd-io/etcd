// Copyright 2015 The etcd Authors
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
	"sync"
	"time"

	v3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/spf13/cobra"
)

// NewEndpointCommand returns the cobra command for "endpoint".
func NewEndpointCommand() *cobra.Command {
	ec := &cobra.Command{
		Use:   "endpoint <subcommand>",
		Short: "Endpoint related commands",
	}

	ec.AddCommand(newEpHealthCommand())
	ec.AddCommand(newEpStatusCommand())

	return ec
}

func newEpHealthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Checks the healthiness of endpoints",
		Run:   epHealthCommandFunc,
	}

	return cmd
}

func newEpStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Prints out the status of endpoints",
		Long: `When --write-out is set to simple, this command prints out comma-separated status lists for each endpoint.
The items in the lists are endpoint, ID, version, db size, is leader, raft term, raft index.
`,
		Run: epStatusCommandFunc,
	}
}

// epHealthCommandFunc executes the "endpoint-health" command.
func epHealthCommandFunc(cmd *cobra.Command, args []string) {
	c := mustClientFromCmd(cmd)

	// collect all endpoints
	endpoints := []string{}
	ctx, cancel := commandCtx(cmd)
	memberListResp, err := c.MemberList(ctx)
	if err != nil {
		ExitWithError(ExitError, err)
	}
	cancel()
	for _, member := range memberListResp.Members {
		for _, endpoint := range member.ClientURLs {
			endpoints = append(endpoints, endpoint)
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(len(endpoints))
	for _, ep := range endpoints {
		go func(endpoint string) {
			defer wg.Done()
			ctx, cancel := commandCtx(cmd)
			// regenerate new client and reset endpoint to include the only client for health check
			clientHealth := mustClientFromCmd(cmd)
			clientHealth.SetEndpoints(endpoint)
			_, err := clientHealth.Get(ctx, "health")
			cancel()
			st := time.Now()
			// permission denied is OK since proposal goes through consensus to get it
			if err == nil || err == rpctypes.ErrPermissionDenied {
				fmt.Printf("%s is healthy: successfully committed proposal: took = %v\n", endpoint, time.Since(st))
			} else {
				fmt.Printf("%s is unhealthy: failed to commit proposal: %v\n", endpoint, err)
			}
		}(ep)
	}
	wg.Wait()
}

type epStatus struct {
	Ep   string             `json:"Endpoint"`
	Resp *v3.StatusResponse `json:"Status"`
}

func epStatusCommandFunc(cmd *cobra.Command, args []string) {
	c := mustClientFromCmd(cmd)

	// collect all endpoints
	ctx, cancel := commandCtx(cmd)
	memberListResp, err := c.MemberList(ctx)
	if err != nil {
		ExitWithError(ExitError, err)
	}
	cancel()
	for _, member := range memberListResp.Members {
		for _, endpoint := range member.ClientURLs {
			mutateEndpoints(c, endpoint)
		}
	}

	statusList := []epStatus{}
	wg := sync.WaitGroup{}
	wg.Add(len(c.Endpoints()))
	for _, ep := range c.Endpoints() {
		go func(endpoint string) {
			defer wg.Done()
			ctx, cancel := commandCtx(cmd)
			resp, serr := c.Status(ctx, endpoint)
			cancel()
			if serr != nil {
				err = serr
				fmt.Fprintf(os.Stderr, "Failed to get the status of endpoint %s (%v)\n", endpoint, serr)
			}
			statusList = append(statusList, epStatus{Ep: endpoint, Resp: resp})
		}(ep)
	}
	wg.Wait()

	display.EndpointStatus(statusList)

	if err != nil {
		os.Exit(ExitError)
	}
}

func mutateEndpoints(client *v3.Client, endpoint string) {
	endpoints := client.Endpoints()
	for _, ep := range endpoints {
		if ep == endpoint {
			return
		}
	}
	endpoints = append(endpoints, endpoint)
	client.SetEndpoints(endpoints...)
}
