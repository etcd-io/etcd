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
	"time"

	"github.com/spf13/cobra"
)

var (
	rounds                 int           // total number of rounds the operation needs to be performed
	totalClientConnections int           // total number of client connections to be made with server
	numPrefixes            int           // total number of prefixes which will be watched upon
	watchesPerPrefix       int           // number of watchers per prefix
	reqRate                int           // put request per second
	totalKeys              int           // total number of keys for operation
	runningTime            time.Duration // time for which operation should be performed
)

// GlobalFlags are flags that defined globally
// and are inherited to all sub-commands.
type GlobalFlags struct {
	Endpoints   []string
	DialTimeout time.Duration
}

func endpointsFromFlag(cmd *cobra.Command) []string {
	endpoints, err := cmd.Flags().GetStringSlice("endpoints")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return endpoints
}

func dialTimeoutFromCmd(cmd *cobra.Command) time.Duration {
	dialTimeout, err := cmd.Flags().GetDuration("dial-timeout")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return dialTimeout
}
