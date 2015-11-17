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
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func NewTreeCommand() cli.Command {
	return cli.Command{
		Name:  "tree",
		Usage: "List directory as a tree",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "sort", Usage: "returns result in sorted order"},
		},
		Action: func(c *cli.Context) {
			treeCommandFunc(c, mustNewKeyAPI(c))
		},
	}
}

var numDirs int
var numKeys int

// treeCommandFunc executes the "tree" command.
func treeCommandFunc(c *cli.Context, ki client.KeysAPI) {
	key := "/"
	if len(c.Args()) != 0 {
		key = c.Args()[0]
	}

	sort := c.Bool("sort")

	resp, err := ki.Get(context.TODO(), key, &client.GetOptions{Sort: sort, Recursive: true})
	if err != nil {
		handleError(ExitServerError, err)
	}

	numDirs = 0
	numKeys = 0
	fmt.Println(strings.TrimRight(key, "/") + "/")
	printTree(resp.Node, "")
	fmt.Printf("\n%d directories, %d keys\n", numDirs, numKeys)
}

// printTree writes a response out in a manner similar to the `tree` command in unix.
func printTree(root *client.Node, indent string) {
	for i, n := range root.Nodes {
		keys := strings.Split(n.Key, "/")
		k := keys[len(keys)-1]

		if n.Dir {
			if i == root.Nodes.Len()-1 {
				fmt.Printf("%s└── %s/\n", indent, k)
				printTree(n, indent+"    ")
			} else {
				fmt.Printf("%s├── %s/\n", indent, k)
				printTree(n, indent+"│   ")
			}
			numDirs++
		} else {
			if i == root.Nodes.Len()-1 {
				fmt.Printf("%s└── %s\n", indent, k)
			} else {
				fmt.Printf("%s├── %s\n", indent, k)
			}

			numKeys++
		}
	}
}
