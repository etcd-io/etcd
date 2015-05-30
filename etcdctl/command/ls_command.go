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
	"fmt"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

func NewLsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "retrieve a directory",
		Run: func(cmd *cobra.Command, args []string) {
			handleLs(cmd, args, lsCommandFunc)
		},
	}
	cmd.Flags().Bool("sort", false, "returns result in sorted order")
	cmd.Flags().Bool("recursive", false, "returns all values for key and child keys")
	cmd.Flags().Bool("p", false, "append slash (/) to directories")
	return cmd
}

// handleLs handles a request that intends to do ls-like operations.
func handleLs(cmd *cobra.Command, args []string, fn handlerFunc) {
	handleContextualPrint(cmd, args, fn, printLs)
}

// printLs writes a response out in a manner similar to the `ls` command in unix.
// Non-empty directories list their contents and files list their name.
func printLs(cmd *cobra.Command, resp *etcd.Response, format string) {
	if !resp.Node.Dir {
		fmt.Println(resp.Node.Key)
	}
	for _, node := range resp.Node.Nodes {
		rPrint(cmd, node)
	}
}

// lsCommandFunc executes the "ls" command.
func lsCommandFunc(cmd *cobra.Command, args []string, client *etcd.Client) (*etcd.Response, error) {
	key := "/"
	if len(args) != 0 {
		key = args[0]
	}
	recursive, _ := cmd.Flags().GetBool("recursive")
	sort, _ := cmd.Flags().GetBool("sort")

	// Retrieve the value from the server.
	return client.Get(key, sort, recursive)
}

// rPrint recursively prints out the nodes in the node structure.
func rPrint(cmd *cobra.Command, n *etcd.Node) {
	p, _ := cmd.Flags().GetBool("p")
	if n.Dir && p {
		fmt.Println(fmt.Sprintf("%v/", n.Key))
	} else {
		fmt.Println(n.Key)
	}

	for _, node := range n.Nodes {
		rPrint(cmd, node)
	}
}
