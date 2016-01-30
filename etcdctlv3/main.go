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

// etcdctlv3 is a command line application that utilizes v3 API.
package main

import (
	"text/tabwriter"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/etcdctlv3/command"
)

const (
	cliName        = "etcdctlv3"
	cliDescription = "A simple command line client for etcd3."
)

var (
	tabOut      *tabwriter.Writer
	globalFlags = command.GlobalFlags{}
)

var (
	rootCmd = &cobra.Command{
		Use:        cliName,
		Short:      cliDescription,
		SuggestFor: []string{"etcctlv3", "etcdcltv3", "etlctlv3"},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&globalFlags.Endpoints, "endpoint", "127.0.0.1:2378", "gRPC endpoint")

	rootCmd.PersistentFlags().StringVar(&globalFlags.TLS.CertFile, "cert", "", "identify HTTPS client using this SSL certificate file")
	rootCmd.PersistentFlags().StringVar(&globalFlags.TLS.KeyFile, "key", "", "identify HTTPS client using this SSL key file")
	rootCmd.PersistentFlags().StringVar(&globalFlags.TLS.CAFile, "cacert", "", "verify certificates of HTTPS-enabled servers using this CA bundle")

	rootCmd.AddCommand(
		command.NewRangeCommand(),
		command.NewPutCommand(),
		command.NewDeleteRangeCommand(),
		command.NewTxnCommand(),
		command.NewCompactionCommand(),
		command.NewWatchCommand(),
		command.NewVersionCommand(),
		command.NewLeaseCommand(),
		command.NewMemberCommand(),
	)
}

func init() {
	cobra.EnablePrefixMatching = true
}

func main() {
	rootCmd.SetUsageFunc(usageFunc)

	// Make help just show the usage
	rootCmd.SetHelpTemplate(`{{.UsageString}}`)

	if err := rootCmd.Execute(); err != nil {
		command.ExitWithError(command.ExitError, err)
	}
}
