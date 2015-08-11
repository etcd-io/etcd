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
	"os"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

// NewUpdateDirCommand returns the CLI command for "updatedir".
func NewUpdateDirCommand() cli.Command {
	return cli.Command{
		Name:  "updatedir",
		Usage: "update an existing directory",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "ttl", Value: 0, Usage: "key time-to-live"},
		},
		Action: func(c *cli.Context) {
			updatedirCommandFunc(c, mustNewKeyAPI(c))
		},
	}
}

// updatedirCommandFunc executes the "updatedir" command.
func updatedirCommandFunc(c *cli.Context, ki client.KeysAPI) {
	if len(c.Args()) == 0 {
		handleError(ExitBadArgs, errors.New("key required"))
	}
	key := c.Args()[0]
	value, err := argOrStdin(c.Args(), os.Stdin, 1)
	if err != nil {
		handleError(ExitBadArgs, errors.New("value required"))
	}

	ttl := c.Int("ttl")

	_, err = ki.Set(context.TODO(), key, value, &client.SetOptions{TTL: time.Duration(ttl) * time.Second, Dir: true, PrevExist: client.PrevExist})
	if err != nil {
		handleError(ExitServerError, err)
	}
}
