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
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
)

// NewSetCommand returns the CLI command for "set".
func NewSetCommand() cli.Command {
	return cli.Command{
		Name:  "set",
		Usage: "set the value of a key",
		Flags: []cli.Flag{
			cli.IntFlag{Name: "ttl", Value: 0, Usage: "key time-to-live"},
			cli.StringFlag{Name: "swap-with-value", Value: "", Usage: "previous value"},
			cli.IntFlag{Name: "swap-with-index", Value: 0, Usage: "previous index"},
		},
		Action: func(c *cli.Context) {
			handleKey(c, setCommandFunc)
		},
	}
}

// setCommandFunc executes the "set" command.
func setCommandFunc(c *cli.Context, client *etcd.Client) (*etcd.Response, error) {
	if len(c.Args()) == 0 {
		return nil, errors.New("Key required")
	}
	key := c.Args()[0]
	value, err := argOrStdin(c.Args(), os.Stdin, 1)
	if err != nil {
		return nil, errors.New("Value required")
	}

	ttl := c.Int("ttl")
	prevValue := c.String("swap-with-value")
	prevIndex := c.Int("swap-with-index")

	if prevValue == "" && prevIndex == 0 {
		return client.Set(key, value, uint64(ttl))
	} else {
		return client.CompareAndSwap(key, value, uint64(ttl), prevValue, uint64(prevIndex))
	}
}
