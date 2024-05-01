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

var closeStaleIssuesCmd = &cobra.Command{
	Use:   "auto-close-stale-issues",
	Short: "auto close stale flaky test issues",
	Long:  `automatically close stale Github issues for flaky test.`,
	Run:   closeStaleIssuesFunc,
}

var (
	flakyThreshold         float32
	maxSubIssuesForTestSet int
	minRuns                int
	maxRuns                int
	maxDays                int
	autoCreateIssues       bool
	daysBeforeAutoClose    int

	lineSep = "-------------------------------------------------------------"
)

func init() {
	rootCmd.AddCommand(flakyCmd)
	rootCmd.AddCommand(closeStaleIssuesCmd)

	flakyCmd.Flags().Float32Var(&flakyThreshold, "flaky-threshold", 0.1, "fraction threshold of test failures for a test to be considered flaky")
	flakyCmd.Flags().IntVar(&minRuns, "min-runs", 20, "minimum test runs for a test to be created an issue for")
	flakyCmd.Flags().IntVar(&maxRuns, "max-runs", 0, "maximum test runs for a test to be included in flaky analysis, 0 to include all")
	flakyCmd.Flags().IntVar(&maxDays, "max-days", 30, "maximum days of results before today to be included in flaky analysis, 0 to include all")
	flakyCmd.Flags().BoolVar(&autoCreateIssues, "auto-create-issues", false, "automatically create Github issue for flaky test")
	flakyCmd.Flags().IntVar(&maxSubIssuesForTestSet, "max-sub-issues", 3, "maximum number of sub-issues to create for a test set")

	closeStaleIssuesCmd.Flags().IntVar(&daysBeforeAutoClose, "days-before-auto-close", 30, "maximum days of no updates before an issue is automatically closed")
}

func flakyFunc(cmd *cobra.Command, args []string) {
	fmt.Println(lineSep)
	fmt.Printf("flaky called, for %s#%s, createGithubIssue=%v, githubRepo=%s/%s, flakyThreshold=%f, minRuns=%d\n", dashboard, tab, autoCreateIssues, githubOwner, githubRepo, flakyThreshold, minRuns)

	tabSummary := FetchTabResultSummary(dashboard, tab)
	fmt.Println(lineSep)
	if tabSummary.FailureRate < flakyThreshold {
		fmt.Printf("Failure rate for test set %s#%s is %.1f%%, below the flaky threshold %.0f%%\n", dashboard, tab, tabSummary.FailureRate*100, flakyThreshold*100)
		return
	}
	fmt.Printf("Failure rate for test set %s#%s is %.1f%%, above the flaky threshold %.0f%%\n", dashboard, tab, tabSummary.FailureRate*100, flakyThreshold*100)
	if autoCreateIssues {
		createIssues(tabSummary, minRuns, maxSubIssuesForTestSet, []string{"type/flake"})
	}
	fmt.Println(lineSep)
}

func closeStaleIssuesFunc(cmd *cobra.Command, args []string) {
	fmt.Println(lineSep)
	fmt.Printf("auto close stale issues with no updates for %d days in githubRepo=%s/%s\n", daysBeforeAutoClose, githubOwner, githubRepo)
	closeStaleIssues(daysBeforeAutoClose, []string{"type/flake"})
	fmt.Println(lineSep)
}
