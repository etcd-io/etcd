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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

// NewSetCommand returns the CLI command for "set".
func NewSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set the value of a key",
		Run: func(cmd *cobra.Command, args []string) {
			handleKey(cmd, args, setCommandFunc)
		},
	}
	cmd.Flags().Uint64("ttl", 0, "key time-to-live")
	cmd.Flags().String("swap-with-value", "", "previous value")
	cmd.Flags().Uint64("swap-with-index", 0, "previous index")
	return cmd
}

// setCommandFunc executes the "set" command.
func setCommandFunc(cmd *cobra.Command, args []string, client *etcd.Client) (*etcd.Response, error) {
	if len(args) == 0 {
		return nil, errors.New("Key required")
	}
	key := args[0]
	value, err := argOrStdin(args, os.Stdin, 1)
	if err != nil {
		return nil, errors.New("value required")
	}

	ttl, _ := cmd.Flags().GetUint64("ttl")
	prevValue, _ := cmd.Flags().GetString("swap-with-value")
	prevIndex, _ := cmd.Flags().GetUint64("swap-with-index")

	if prevValue == "" && prevIndex == 0 {
		return client.Set(key, value, ttl)
	}
	return client.CompareAndSwap(key, value, ttl, prevValue, prevIndex)
}
