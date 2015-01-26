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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewGetWatchCommand returns the CLI command for "getwatch".
func NewGetWatchCommand() cli.Command {
	return cli.Command{
		Name:  "getwatch",
		Usage: "retrieve the value of a key, or if it doesn't exist, watch the key for changes",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "sort", Usage: "returns result in sorted order"},
			cli.IntFlag{Name: "after-index", Value: 0, Usage: "watch after the given index"},
			cli.BoolFlag{Name: "recursive", Usage: "returns all values for key and child keys"},
		},
		Action: func(c *cli.Context) {
			handleGetWatch(c, getWatchCommandFunc)
		},
	}
}

// handleGetWatch handles a request that intends to do getwatch-like operations.
func handleGetWatch(c *cli.Context, fn handlerFunc) {
	handlePrint(c, fn, printGetWatch)
}

// printGetWatch writes error message when getting the value of a directory.
func printGetWatch(resp *etcd.Response, format string) {
	if resp.Node.Dir {
		fmt.Fprintln(os.Stderr, fmt.Sprintf("%s: is a directory", resp.Node.Key))
		os.Exit(1)
	}

	printKey(resp, format)
}

// getWatchCommandFunc executes the "getwatch" command.
func getWatchCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	sorted := c.Bool("sort")
	recursive := c.Bool("recursive")

	index := 0
	if c.Int("after-index") != 0 {
		index = c.Int("after-index") + 1
	}

	// Retrieve the value from the server.
	value, _ := client.Get(key, sorted, false)

	if value == nil {
		var resp *etcd.Response
		var err error
		resp, err = client.Watch(key, uint64(index), recursive, nil, nil)

		if err != nil {
			handleError(ErrorFromEtcd, err)
		}

		if err != nil {
			return nil, err
		}
		printAll(resp, c.GlobalString("output"))
	}

	return value, nil
}
