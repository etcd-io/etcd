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

package tester

import (
	"log"
	"net/http"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

type flag struct {
	AgentEndpoints []string

	DataDir string
	V2      bool

	StressKeySize        int
	StressKeySuffixRange int

	RoundLimit int

	TesterPort string
}

var (
	Command = &cobra.Command{
		Use:   "tester",
		Short: "tester utilizes all etcd-agents to control the cluster and simulate various test cases.",
		Run:   CommandFunc,
	}

	cmdFlag = flag{}
)

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	Command.PersistentFlags().StringSliceVar(&cmdFlag.AgentEndpoints, "agent-endpoints", []string{"http://localhost:9027"}, "HTTP RPC endpoints of agents.")

	Command.PersistentFlags().StringVar(&cmdFlag.DataDir, "data-dir", "agent.etcd", "etcd data directory path on agent machine.")
	Command.PersistentFlags().BoolVar(&cmdFlag.V2, "v2", false, "'true' to test v2 (to be deprecated in preference to v3).")

	Command.PersistentFlags().IntVar(&cmdFlag.StressKeySize, "stress-key-size", 100, "Size of each key written into etcd.")
	Command.PersistentFlags().IntVar(&cmdFlag.StressKeySuffixRange, "stress-key-suffix-range", 1000, "Count of key range written into etcd.")

	Command.PersistentFlags().IntVar(&cmdFlag.RoundLimit, "round-limit", 3, "Limit of rounds to run failure set.")

	Command.PersistentFlags().StringVarP(&cmdFlag.TesterPort, "tester-port", "p", ":9028", "Port to serve tester status.")
}

func CommandFunc(cmd *cobra.Command, args []string) {
	c, err := newCluster(cmdFlag)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Terminate()

	t := &tester{
		failures: []failure{
			newFailureKillAll(),
			newFailureKillMajority(),
			newFailureKillOne(),
			newFailureKillOneForLongTime(),
			newFailureIsolate(),
			newFailureIsolateAll(),
		},
		cluster: c,
		limit:   cmdFlag.RoundLimit,
	}

	sh := statusHandler{status: &t.status}
	http.Handle("/status", sh)
	go func() { log.Fatal(http.ListenAndServe(cmdFlag.TesterPort, nil)) }()

	t.runLoop()
}
