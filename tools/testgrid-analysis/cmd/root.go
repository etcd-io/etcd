// Copyright 2024 The etcd Authors
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

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	dashboard string
	tab       string
)

var rootCmd = &cobra.Command{
	Use:   "testgrid-analysis",
	Short: "testgrid-analysis",
	Long:  `testgrid-analysis analyzes the testgrid test results of sig-etcd.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dashboard, "dashboard", "sig-etcd-periodics", "testgrid dashboard to retrieve data from")
	rootCmd.PersistentFlags().StringVar(&tab, "tab", "ci-etcd-e2e-amd64", "testgrid tab within the dashboard to retrieve data from")
}
