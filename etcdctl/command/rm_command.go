/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package command

import (
	"errors"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewRemoveCommand returns the CLI command for "rm".
func NewRemoveCommand() cli.Command {
	return cli.Command{
		Name:  "rm",
		Usage: "remove a key",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "dir", Usage: "removes the key if it is an empty directory or a key-value pair"},
			cli.BoolFlag{Name: "recursive", Usage: "removes the key and all child keys(if it is a directory)"},
			cli.StringFlag{Name: "with-value", Value: "", Usage: "previous value"},
			cli.IntFlag{Name: "with-index", Value: 0, Usage: "previous index"},
		},
		Action: func(c *cli.Context) {
			handleAll(c, removeCommandFunc)
		},
	}
}

// removeCommandFunc executes the "rm" command.
func removeCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	recursive := c.Bool("recursive")
	dir := c.Bool("dir")

	// TODO: distinguish with flag is not set and empty flag
	// the cli pkg need to provide this feature
	prevValue := c.String("with-value")
	prevIndex := uint64(c.Int("with-index"))

	if prevValue != "" || prevIndex != 0 {
		return client.CompareAndDelete(key, prevValue, prevIndex)
	}

	if recursive || !dir {
		return client.Delete(key, recursive)
	}

	return client.DeleteDir(key)
}
