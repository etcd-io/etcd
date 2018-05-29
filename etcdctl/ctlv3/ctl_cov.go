// Copyright 2017 The etcd Authors
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

// +build cov

package ctlv3

import (
	"os"
	"strings"

	"github.com/coreos/etcd/etcdctl/ctlv3/command"
)

func Start(apiv string) {
	// ETCDCTL_ARGS=etcdctl_test arg1 arg2...
	// SetArgs() takes arg1 arg2...
	if apiv == "" {
		rootCmd.Short += "\n\n" +
			"WARNING:\n" +
			"        Environment variable ETCDCTL_API is not set; defaults to etcdctl v3.\n" +
			"        Set environment variable ETCDCTL_API=2 to use v2 API or ETCDCTL_API=3 to use v3 API."

	}
	rootCmd.SetArgs(strings.Split(os.Getenv("ETCDCTL_ARGS"), "\xe7\xcd")[1:])
	os.Unsetenv("ETCDCTL_ARGS")
	if err := rootCmd.Execute(); err != nil {
		command.ExitWithError(command.ExitError, err)
	}
}
