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
	"io"
	"os"
	"os/exec"
	"os/signal"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

// NewExecWatchCommand returns the CLI command for "exec-watch".
func NewExecWatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec-watch",
		Short: "watch a key for changes and exec an executable",
		Run: func(cmd *cobra.Command, args []string) {
			handleKey(cmd, args, execWatchCommandFunc)
		},
	}
	cmd.Flags().Uint64("after-index", 0, "watch after the given index")
	cmd.Flags().Bool("recursive", false, "watch all values for key and child keys")
	return cmd
}

// execWatchCommandFunc executes the "exec-watch" command.
func execWatchCommandFunc(cmd *cobra.Command, args []string, client *etcd.Client) (*etcd.Response, error) {
	_ = io.Copy
	_ = exec.Command
	argsLen := len(args)

	if argsLen < 2 {
		return nil, errors.New("Key and command to exec required")
	}

	key := args[argsLen-1]
	cmdArgs := args[:argsLen-1]

	index, _ := cmd.Flags().GetUint64("after-index")
	if index != 0 {
		index = index + 1
		key = args[0]
		cmdArgs = args[2:]
	}

	recursive, _ := cmd.Flags().GetBool("recursive")
	if recursive != false {
		key = args[0]
		cmdArgs = args[2:]
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	stop := make(chan bool)

	go func() {
		<-sigch
		stop <- true
		os.Exit(0)
	}()

	receiver := make(chan *etcd.Response)
	go client.Watch(key, index, recursive, receiver, stop)

	for {
		resp := <-receiver
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Env = environResponse(resp, os.Environ())

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		err = cmd.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, err.Error())
			os.Exit(1)
		}
		go io.Copy(os.Stdout, stdout)
		go io.Copy(os.Stderr, stderr)
		cmd.Wait()
	}
}

func environResponse(resp *etcd.Response, env []string) []string {
	env = append(env, "ETCD_WATCH_ACTION="+resp.Action)
	env = append(env, "ETCD_WATCH_MODIFIED_INDEX="+fmt.Sprintf("%d", resp.Node.ModifiedIndex))
	env = append(env, "ETCD_WATCH_KEY="+resp.Node.Key)
	env = append(env, "ETCD_WATCH_VALUE="+resp.Node.Value)
	return env
}
