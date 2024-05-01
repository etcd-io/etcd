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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
)

func createIssues(summary *TabResultSummary, minRuns, maxSubIssues int, labels []string) {
	subIssues := []string{}
	for _, t := range summary.TestsWithFailures {
		if t.TotalRuns < minRuns {
			continue
		}
		subIssue := createOrUpdateIssue(fmt.Sprintf("Flaky Test: %s", t.Name), t.IssueBody, labels)
		subIssues = append(subIssues, subIssue)
		if len(subIssues) >= maxSubIssues {
			break
		}
	}
	body := summary.IssueBody + fmt.Sprintf("\n## Sub-issues\nauto created sub-issues for the top %d failed tests with at least %d runs:\n", maxSubIssues, minRuns) + strings.Join(subIssues, "\n")
	createOrUpdateIssue(fmt.Sprintf("Flaky Test Set: %s", summary.TabName), body, labels)
}

func getOpenIssues(labels []string) []*github.Issue {
	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	ctx := context.Background()
	// list open issues with label type/flake
	issueOpt := &github.IssueListByRepoOptions{
		Labels:      labels,
		ListOptions: github.ListOptions{PerPage: 100},
	}
	allIssues := []*github.Issue{}
	for {
		issues, resp, err := client.Issues.ListByRepo(ctx, githubOwner, githubRepo, issueOpt)
		if err != nil {
			panic(err)
		}
		allIssues = append(allIssues, issues...)
		if resp.NextPage == 0 {
			break
		}
		issueOpt.Page = resp.NextPage
	}
	fmt.Printf("There are %d issues open with label %v\n", len(allIssues), labels)
	return allIssues
}

func createOrUpdateIssue(title, body string, labels []string) string {
	issues := getOpenIssues(labels)
	newLabels := append(labels, "help wanted")

	var currentIssue *github.Issue
	title = strings.TrimSpace(title)
	// check if there is already an open issue regarding this test
	for _, issue := range issues {
		if strings.Contains(*issue.Title, title) {
			fmt.Printf("%s is already open for %s\n", issue.GetHTMLURL(), title)
			currentIssue = issue
			break
		}
	}
	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	ctx := context.Background()
	if currentIssue == nil {
		fmt.Printf("Opening new issue for %s\n", title)
		req := &github.IssueRequest{
			Title:  github.String(title),
			Body:   &body,
			Labels: &newLabels,
		}
		issue, _, err := client.Issues.Create(ctx, githubOwner, githubRepo, req)
		if err != nil {
			panic(err)
		}
		fmt.Printf("New issue %s created for %s\n\n", issue.GetHTMLURL(), title)
		return issue.GetHTMLURL()
	}
	// if the issue already exists, append comments
	comment := &github.IssueComment{
		Body: &body,
	}
	issueComment, _, err := client.Issues.CreateComment(ctx, githubOwner, githubRepo, *currentIssue.Number, comment)
	if err != nil {
		panic(err)
	}
	fmt.Printf("New comment %s created for %s\n\n", issueComment.GetHTMLURL(), title)
	return currentIssue.GetHTMLURL()
}

func closeStaleIssues(daysBeforeAutoClose int, labels []string) {
	earliestTimeToConsider := time.Now().AddDate(0, 0, -1*daysBeforeAutoClose)
	issues := getOpenIssues(labels)
	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	ctx := context.Background()
	newLabels := append(labels, "stale")
	warning := fmt.Sprintf("auto close due to no updates for %d days", daysBeforeAutoClose)
	state := "closed"
	cnt := 0

	for _, issue := range issues {
		if issue.UpdatedAt.Before(earliestTimeToConsider) {
			fmt.Printf("closing stale issue %s last updated at %s\n", *issue.HTMLURL, issue.UpdatedAt.String())
			comment := &github.IssueComment{
				Body: &warning,
			}
			_, _, err := client.Issues.CreateComment(ctx, githubOwner, githubRepo, *issue.Number, comment)
			if err != nil {
				panic(err)
			}

			req := &github.IssueRequest{
				Labels: &newLabels,
				State:  &state,
			}
			respIssue, _, err := client.Issues.Edit(ctx, githubOwner, githubRepo, *issue.Number, req)
			if err != nil {
				panic(err)
			}
			if *respIssue.State == "closed" {
				fmt.Printf("closed stale issue %s\n", *issue.HTMLURL)
				cnt++
			} else {
				fmt.Printf("failed to close stale issue %s\n", *issue.HTMLURL)
			}
		}
	}
	fmt.Printf("closed %d/%d stale issues\n", cnt, len(issues))
}
