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

// NewRemoveCommand returns the CLI command for "rmdir".
func NewRemoveDirCommand() cli.Command {
	return cli.Command{
		Name:  "rmdir",
		Usage: "removes the key if it is an empty directory or a key-value pair",
		Action: func(c *cli.Context) {
			handleDir(c, removeDirCommandFunc)
		},
	}
}

// removeDirCommandFunc executes the "rmdir" command.
func removeDirCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]

	return client.DeleteDir(key)
}
