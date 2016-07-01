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

package etcdmain

import (
	"fmt"
	"os"
	"runtime"

	"github.com/coreos/etcd/version"
	"github.com/spf13/cobra"
)

func printVersion() {
	fmt.Printf("etcd version: %s\n", version.Version)
	fmt.Printf("Git SHA:      %s\n", version.GitSHA)
	fmt.Printf("Go version:   %s\n", runtime.Version())
	fmt.Printf("Go OS/Arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	os.Exit(0)
}

func init() {
	rootCmd.AddCommand(newVersionCommand())
}

// newVersionCommand returns the cobra command for "version".
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Shows the etcd version information",
		Run: func(cmd *cobra.Command, args []string) {
			printVersion()
		},
	}
}
