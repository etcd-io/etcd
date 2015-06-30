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
	"fmt"
	"os"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func NewAuthCommands() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "overall auth controls",
		Run:   handleBackup,
	}

	enable := &cobra.Command{
		Use:   "enable",
		Short: "enable auth access controls",
		Run:   actionAuthEnable,
	}

	disable := &cobra.Command{
		Use:   "disable",
		Short: "disable auth access controls",
		Run:   actionAuthDisable,
	}

	cmd.AddCommand(enable, disable)
	return cmd
}

func actionAuthEnable(cmd *cobra.Command, args []string) {

	authEnableDisable(cmd, args, true)
}

func actionAuthDisable(cmd *cobra.Command, args []string) {
	authEnableDisable(cmd, args, false)
}

func mustNewAuthAPI(cmd *cobra.Command) client.AuthAPI {
	hc := mustNewClient(cmd)

	if d, _ := cmd.Flags().GetBool("debug"); d {
		fmt.Fprintf(os.Stderr, "Cluster-Endpoints: %s\n", strings.Join(hc.Endpoints(), ", "))
	}

	return client.NewAuthAPI(hc)
}

func authEnableDisable(cmd *cobra.Command, args []string, enable bool) {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "No arguments accepted")
		os.Exit(1)
	}
	s := mustNewAuthAPI(cmd)
	ctx, cancel := context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
	var err error
	if enable {
		err = s.Enable(ctx)
	} else {
		err = s.Disable(ctx)
	}
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if enable {
		fmt.Println("Authentication Enabled")
	} else {
		fmt.Println("Authentication Disabled")
	}
}
