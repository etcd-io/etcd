// Copyright 2015 CoreOS, Inc.
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
	"errors"
	"fmt"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

// NewGetCommand returns the CLI command for "get".
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "retrieve the value of a key",
		Run: func(cmd *cobra.Command, args []string) {
			handleGet(cmd, args, getCommandFunc)
		},
	}
	cmd.Flags().Bool("sort", false, "returns result in sorted order")
	return cmd
}

// handleGet handles a request that intends to do get-like operations.
func handleGet(cmd *cobra.Command, args []string, fn handlerFunc) {
	handlePrint(cmd, args, fn, printGet)
}

// printGet writes error message when getting the value of a directory.
func printGet(resp *etcd.Response, format string) {
	if resp.Node.Dir {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("%s: is a directory", resp.Node.Key))
		os.Exit(1)
	}

	printKey(resp, format)
}

// getCommandFunc executes the "get" command.
func getCommandFunc(cmd *cobra.Command, args []string, client *etcd.Client) (*etcd.Response, error) {
	if len(args) == 0 {
		return nil, errors.New("Key required")
	}
	key := args[0]
	sorted, _ := cmd.Flags().GetBool("sort")

	// Retrieve the value from the server.
	return client.Get(key, sorted, false)
}
