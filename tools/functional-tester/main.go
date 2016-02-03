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

//	functional-tester tests the functionality of an etcd cluster with a focus on failure-resistance under heavy usage.
//
//	Usage:
//	  functional-tester [command]
//
//	Available Commands:
//	  agent       agent is a daemon on each machine to control an etcd process: start, stop, restart, isolate, terminate, etc.
//	  tester      tester utilizes all etcd-agents to control the cluster and simulate various test cases.
//
//	Use "functional-tester [command] --help" for more information about a command.
//
package main

import (
	"fmt"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	agent "github.com/coreos/etcd/tools/functional-tester/etcd-agent"
	tester "github.com/coreos/etcd/tools/functional-tester/etcd-tester"
)

var (
	rootCommand = &cobra.Command{
		Use:        "functional-tester",
		Short:      "functional-tester tests the functionality of an etcd cluster with a focus on failure-resistance under heavy usage.",
		SuggestFor: []string{"function-tester", "functionaltester", "fnucino-tester"},
	}
)

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCommand.AddCommand(agent.Command)
	rootCommand.AddCommand(tester.Command)
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}
