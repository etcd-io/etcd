// Copyright 2016 The etcd Authors
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

	"github.com/coreos/etcd/tools/functional-tester/etcd-runner/runner"
	"github.com/spf13/cobra"
)

// NewWatchCommand returns the cobra command for "watcher runner".
func NewWatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watcher",
		Short: "Performs watch operation",
		Run:   runWatcherFunc,
	}
	cmd.Flags().IntVar(&rounds, "rounds", 100, "number of rounds to run")
	cmd.Flags().DurationVar(&runningTime, "running-time", 60, "number of seconds to run")
	cmd.Flags().IntVar(&numPrefixes, "total-prefixes", 10, "total no of prefixes to use")
	cmd.Flags().IntVar(&watchesPerPrefix, "watch-per-prefix", 10, "number of watchers per prefix")
	cmd.Flags().IntVar(&reqRate, "req-rate", 30, "rate at which put request will be performed")
	cmd.Flags().IntVar(&totalKeys, "total-keys", 1000, "total number of keys to watch")
	return cmd
}

func runWatcherFunc(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		ExitWithError(ExitBadArgs, errors.New("watcher does not take any argument"))
	}

	eps := endpointsFromFlag(cmd)
	dialTimeout := dialTimeoutFromCmd(cmd)

	wcf := &runner.WatchRunnerConfig{
		EtcdRunnerConfig: runner.EtcdRunnerConfig{
			Eps:                    eps,
			DialTimeout:            dialTimeout,
			TotalClientConnections: totalClientConnections,
			Rounds:                 rounds,
		},

		RunningTime:      runningTime,
		NumPrefixes:      numPrefixes,
		WatchesPerPrefix: watchesPerPrefix,
		ReqRate:          reqRate,
		TotalKeys:        totalKeys,
	}

	runner.RunWatch(wcf)
}
