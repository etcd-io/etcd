// Copyright 2025 The etcd Authors
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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewOptionsCommand(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Show the global command-line flags",
		Run: func(cmd *cobra.Command, args []string) {
			fs := unhideCopy(rootCmd.PersistentFlags())
			fmt.Fprintf(cmd.OutOrStdout(), "The following options can be passed to any command:\n\n")
			fmt.Fprint(cmd.OutOrStdout(), fs.FlagUsages())
		},
		GroupID: groupUtilityID,
	}
	return cmd
}

func unhideCopy(src *pflag.FlagSet) *pflag.FlagSet {
	out := pflag.NewFlagSet("global", pflag.ContinueOnError)
	src.VisitAll(func(f *pflag.Flag) {
		nf := *f
		nf.Hidden = false
		out.AddFlag(&nf)
	})
	return out
}
