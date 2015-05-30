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
	"os/signal"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

// NewWatchCommand returns the CLI command for "watch".
func NewWatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "watch a key for changes",
		Run: func(cmd *cobra.Command, args []string) {
			handleKey(cmd, args, watchCommandFunc)
		},
	}
	cmd.Flags().Bool("forever", false, "forever watch a key until CTRL+C")
	cmd.Flags().Uint64("after-index", 0, "watch after the given index")
	cmd.Flags().Bool("recursive", false, "returns all values for key and child keys")
	return cmd
}

// watchCommandFunc executes the "watch" command.
func watchCommandFunc(cmd *cobra.Command, args []string, client *etcd.Client) (*etcd.Response, error) {
	if len(args) == 0 {
		return nil, errors.New("Key required")
	}
	key := args[0]
	recursive, _ := cmd.Flags().GetBool("recursive")
	forever, _ := cmd.Flags().GetBool("forever")

	index, _ := cmd.Flags().GetUint64("after-index")
	if index != 0 {
		index = index + 1
	}

	if forever {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt)
		stop := make(chan bool)

		go func() {
			<-sigch
			os.Exit(0)
		}()

		receiver := make(chan *etcd.Response)
		errCh := make(chan error, 1)

		go func() {
			_, err := client.Watch(key, index, recursive, receiver, stop)
			errCh <- err
		}()

		for {
			select {
			case resp := <-receiver:
				output, _ := cmd.Flags().GetString("output")
				printAll(resp, output)
			case err := <-errCh:
				handleError(-1, err)
			}
		}

	} else {
		var resp *etcd.Response
		var err error
		resp, err = client.Watch(key, index, recursive, nil, nil)

		if err != nil {
			handleError(ExitServerError, err)
		}

		if err != nil {
			return nil, err
		}
		output, _ := cmd.Flags().GetString("output")
		printAll(resp, output)
	}

	return nil, nil
}
