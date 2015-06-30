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

// NewUpdateCommand returns the CLI command for "update".
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update an existing key with a given value",
		Run: func(cmd *cobra.Command, args []string) {
			handleKey(cmd, args, updateCommandFunc)
		},
	}
	cmd.Flags().Uint64("ttl", 0, "key time-to-live")
	return cmd
}

// updateCommandFunc executes the "update" command.
func updateCommandFunc(cmd *cobra.Command, args []string, client *etcd.Client) (*etcd.Response, error) {
	if len(args) == 0 {
		return nil, errors.New("Key required")
	}
	key := args[0]
	value, err := argOrStdin(args, os.Stdin, 1)
	if err != nil {
		return nil, errors.New("Value required")
	}

	ttl, _ := cmd.Flags().GetUint64("ttl")

	return client.Update(key, value, ttl)
}
