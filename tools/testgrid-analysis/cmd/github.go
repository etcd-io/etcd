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

	"github.com/google/go-github/v60/github"
)

func createIssues(tests []*TestResultSummary, labels []string) {
	openIssues := getOpenIssues(labels)
	for _, t := range tests {
		createIssueIfNonExist(t, openIssues, append(labels, "help wanted"))
	}
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

func createIssueIfNonExist(t *TestResultSummary, issues []*github.Issue, labels []string) {
	// check if there is already an open issue regarding this test
	for _, issue := range issues {
		if strings.Contains(*issue.Title, t.Name) {
			fmt.Printf("%s is already open for test %s\n\n", issue.GetHTMLURL(), t.Name)
			return
		}
	}
	fmt.Printf("Opening new issue for %s\n", t.Name)
	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	ctx := context.Background()
	req := &github.IssueRequest{
		Title:  github.String(fmt.Sprintf("Flaky test %s", t.Name)),
		Body:   &t.IssueBody,
		Labels: &labels,
	}
	issue, _, err := client.Issues.Create(ctx, githubOwner, githubRepo, req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("New issue %s created for %s\n\n", issue.GetHTMLURL(), t.Name)
}
