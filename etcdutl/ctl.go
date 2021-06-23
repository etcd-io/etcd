// Copyright 2021 The etcd Authors
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

// Package etcdutl contains the main entry point for the etcdutl.
package main

import (
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/etcdutl/v3/etcdutl"
)

const (
	cliName        = "etcdutl"
	cliDescription = "An administrative command line tool for etcd3."
)

var (
	rootCmd = &cobra.Command{
		Use:        cliName,
		Short:      cliDescription,
		SuggestFor: []string{"etcdutl"},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&etcdutl.OutputFormat, "write-out", "w", "simple", "set the output format (fields, json, protobuf, simple, table)")
	rootCmd.RegisterFlagCompletionFunc("write-out", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"fields", "json", "protobuf", "simple", "table"}, cobra.ShellCompDirectiveDefault
	})

	rootCmd.AddCommand(
		etcdutl.NewBackupCommand(),
		etcdutl.NewDefragCommand(),
		etcdutl.NewSnapshotCommand(),
		etcdutl.NewVersionCommand(),
		etcdutl.NewCompletionCommand(),
	)
}

func Start() error {
	// Make help just show the usage
	rootCmd.SetHelpTemplate(`{{.UsageString}}`)
	return rootCmd.Execute()
}

func init() {
	cobra.EnablePrefixMatching = true
}
