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

package main

import (
	"os"
	"path"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/etcdctl/command"
)

func main() {
	cmd := &cobra.Command{
		Use:   path.Base(os.Args[0]),
		Short: "A simple command line client for etcd.",
		BashCompletionFunction: command.BashCompletionFunction,
	}
	_ = cmd.PersistentFlags().Bool("debug", false, "output cURL commands which can be used to reproduce the request")
	_ = cmd.PersistentFlags().Bool("no-sync", false, "don't synchronize cluster information before sending request")
	_ = cmd.PersistentFlags().StringP("output", "o", "simple", "output response in the given format (`simple`, `extended` or `json`)")
	_ = cmd.PersistentFlags().StringP("peers", "C", "", "a comma-delimited list of machine addresses in the cluster (default: \"127.0.0.1:4001,127.0.0.1:2379\")")
	_ = cmd.PersistentFlags().String("cert-file", "", "identify HTTPS client using this SSL certificate file")
	_ = cmd.PersistentFlags().String("key-file", "", "identify HTTPS client using this SSL key file")
	_ = cmd.PersistentFlags().String("ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	_ = cmd.PersistentFlags().StringP("username", "u", "", "provide username[:password] and prompt if password is not supplied.")

	cmd.AddCommand(command.NewBackupCommand())
	cmd.AddCommand(command.NewClusterHealthCommand())
	cmd.AddCommand(command.NewMakeCommand())
	cmd.AddCommand(command.NewMakeDirCommand())
	cmd.AddCommand(command.NewRemoveCommand())
	cmd.AddCommand(command.NewRemoveDirCommand())
	cmd.AddCommand(command.NewGetCommand())
	cmd.AddCommand(command.NewLsCommand())
	cmd.AddCommand(command.NewSetCommand())
	cmd.AddCommand(command.NewSetDirCommand())
	cmd.AddCommand(command.NewUpdateCommand())
	cmd.AddCommand(command.NewUpdateDirCommand())
	cmd.AddCommand(command.NewWatchCommand())
	cmd.AddCommand(command.NewExecWatchCommand())
	cmd.AddCommand(command.NewMemberCommand())
	cmd.AddCommand(command.NewImportSnapCommand())
	cmd.AddCommand(command.NewUserCommands())
	cmd.AddCommand(command.NewRoleCommands())
	cmd.AddCommand(command.NewAuthCommands())
	cmd.AddCommand(command.NewGenerateBashCompletionsCommand())

	cmd.Execute()
}
