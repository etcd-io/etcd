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

// NewLeaseRenewerCommand returns the cobra command for "lease-renewer runner".
func NewLeaseRenewerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lease-renewer",
		Short: "Performs lease renew operation",
		Run:   runLeaseRenewerFunc,
	}
	return cmd
}

func runLeaseRenewerFunc(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		ExitWithError(ExitBadArgs, errors.New("lease-renewer does not take any argument"))
	}

	eps := endpointsFromFlag(cmd)
	dialTimeout := dialTimeoutFromCmd(cmd)

	lcf := &runner.EtcdRunnerConfig{
		Eps:                    eps,
		DialTimeout:            dialTimeout,
		TotalClientConnections: totalClientConnections,
		Rounds:                 rounds,
	}

	runner.RunLeaseRenewer(lcf)
}
