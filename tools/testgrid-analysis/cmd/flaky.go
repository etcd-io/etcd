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
	"fmt"

	"github.com/spf13/cobra"
)

// flakyCmd represents the flaky command
var flakyCmd = &cobra.Command{
	Use:   "flaky",
	Short: "detect flaky tests",
	Long:  `detect flaky tests within the dashobard#tab, and create GitHub issues if desired.`,
	Run:   flakyFunc,
}

var (
	flakyThreshold    float32
	minRuns           int
	maxRuns           int
	maxDays           int
	createGithubIssue bool
	githubOwner       string
	githubRepo        string

	lineSep = "-------------------------------------------------------------"
)

func init() {
	rootCmd.AddCommand(flakyCmd)

	flakyCmd.Flags().BoolVar(&createGithubIssue, "create-issue", false, "create Github issue for each flaky test")
	flakyCmd.Flags().Float32Var(&flakyThreshold, "flaky-threshold", 0.1, "fraction threshold of test failures for a test to be considered flaky")
	flakyCmd.Flags().IntVar(&minRuns, "min-runs", 20, "minimum test runs for a test to be included in flaky analysis")
	flakyCmd.Flags().IntVar(&maxRuns, "max-runs", 0, "maximum test runs for a test to be included in flaky analysis, 0 to include all")
	flakyCmd.Flags().IntVar(&maxDays, "max-days", 0, "maximum days of results before today to be included in flaky analysis, 0 to include all")
	flakyCmd.Flags().StringVar(&githubOwner, "github-owner", "etcd-io", "the github organization to create the issue for")
	flakyCmd.Flags().StringVar(&githubRepo, "github-repo", "etcd", "the github repo to create the issue for")
}

func flakyFunc(cmd *cobra.Command, args []string) {
	fmt.Printf("flaky called, for %s#%s, createGithubIssue=%v, githubRepo=%s/%s, flakyThreshold=%f, minRuns=%d\n", dashboard, tab, createGithubIssue, githubOwner, githubRepo, flakyThreshold, minRuns)

	allTests := fetchTestResultSummaries(dashboard, tab)
	flakyTests := []*TestResultSummary{}
	for _, t := range allTests {
		if t.TotalRuns >= minRuns && t.FailureRate >= flakyThreshold {
			flakyTests = append(flakyTests, t)
		}
	}
	fmt.Println(lineSep)
	fmt.Printf("Detected total %d flaky tests above the %.0f%% threshold for %s#%s\n", len(flakyTests), flakyThreshold*100, dashboard, tab)
	fmt.Println(lineSep)
	if len(flakyTests) == 0 {
		return
	}
	for _, t := range flakyTests {
		fmt.Println(lineSep)
		fmt.Println(t.IssueBody)
		fmt.Println(lineSep)
	}
	if createGithubIssue {
		createIssues(flakyTests, []string{"type/flake"})
	}
}
