// Copyright 2016 CoreOS, Inc.
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

package agent

import (
	"fmt"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

type flag struct {
	EtcdBinary   string
	AgentRPCPort string
}

var (
	Command = &cobra.Command{
		Use:   "agent",
		Short: "agent is a daemon on each machine to control an etcd process: start, stop, restart, isolate, terminate, etc.",
		Run:   CommandFunc,
	}

	cmdFlag = flag{}
)

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	Command.PersistentFlags().StringVarP(&cmdFlag.EtcdBinary, "etcd-binary", "b", "bin/etcd", "Path of executable etcd binary.")
	Command.PersistentFlags().StringVarP(&cmdFlag.AgentRPCPort, "agent-rpc-port", "p", ":9027", "Port to serve agent RPC server. Tester requests to this endpoint.")
}

func CommandFunc(cmd *cobra.Command, args []string) {
	a, err := newAgent(cmdFlag.EtcdBinary)
	if err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
	a.serveRPC(cmdFlag.AgentRPCPort)

	var done chan struct{}
	<-done
}
