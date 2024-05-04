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
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	apipb "github.com/GoogleCloudPlatform/testgrid/pb/api/v1"
	statuspb "github.com/GoogleCloudPlatform/testgrid/pb/test_status"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	validTestStatuses      = []statuspb.TestStatus{statuspb.TestStatus_PASS, statuspb.TestStatus_FAIL, statuspb.TestStatus_FLAKY}
	failureTestStatuses    = []statuspb.TestStatus{statuspb.TestStatus_FAIL, statuspb.TestStatus_FLAKY}
	validTestStatusesInt   = intStatusSet(validTestStatuses)
	failureTestStatusesInt = intStatusSet(failureTestStatuses)

	skippedTestStatuses = make(map[int32]struct{})
)

type TabResultSummary struct {
	DashboardName, TabName string
	TestsWithFailures      []*TestResultSummary
	FailureRate            float32
	IssueBody              string
	allBuilds              map[string]struct{}
	failedBuilds           map[string]struct{}
}

type TestResultSummary struct {
	Name                  string
	FullName              string
	TotalRuns, FailedRuns int
	FailureRate           float32
	FailureLogs           []string
	IssueBody             string
	allBuilds             map[string]struct{}
	failedBuilds          map[string]struct{}
}

func FetchTabResultSummary(dashboard, tab string) *TabResultSummary {
	summary := TabResultSummary{DashboardName: dashboard, TabName: tab}
	summary.analyzeTestResults()
	return &summary
}

func (tab *TabResultSummary) dataURLs() (rowsURL, headersURL string) {
	rowsURL = fmt.Sprintf("http://testgrid-data.k8s.io/api/v1/dashboards/%s/tabs/%s/rows", tab.DashboardName, tab.TabName)
	headersURL = fmt.Sprintf("http://testgrid-data.k8s.io/api/v1/dashboards/%s/tabs/%s/headers", tab.DashboardName, tab.TabName)
	return
}

func (tab *TabResultSummary) analyzeTestResults() {
	// Fetch test data
	rowsURL, headersURL := tab.dataURLs()
	var testData apipb.ListRowsResponse
	var headerData apipb.ListHeadersResponse
	protojson.Unmarshal(fetchJSON(rowsURL), &testData)
	protojson.Unmarshal(fetchJSON(headersURL), &headerData)

	var allTests []string
	for _, row := range testData.Rows {
		allTests = append(allTests, row.Name)
	}

	tab.allBuilds = map[string]struct{}{}
	tab.failedBuilds = map[string]struct{}{}

	// Process rows
	for _, row := range testData.Rows {
		t := processRow(tab.DashboardName, tab.TabName, row, allTests, headerData.Headers)
		mergeMaps(t.allBuilds, tab.allBuilds)
		mergeMaps(t.failedBuilds, tab.failedBuilds)
		if t.FailedRuns > 0 {
			tab.TestsWithFailures = append(tab.TestsWithFailures, t)
		}
	}
	sort.Slice(tab.TestsWithFailures, func(i, j int) bool {
		ti := tab.TestsWithFailures[i]
		tj := tab.TestsWithFailures[j]
		if ti.FailureRate == tj.FailureRate {
			if ti.FailedRuns == tj.FailedRuns {
				return ti.FullName < tj.FullName
			}
			return ti.FailedRuns > tj.FailedRuns
		}
		return ti.FailureRate > tj.FailureRate
	})
	if len(tab.allBuilds) > 0 {
		tab.FailureRate = float32(len(tab.failedBuilds)) / float32(len(tab.allBuilds))
	}
	tab.IssueBody += fmt.Sprintf("%s#%s failed %.1f%% (%d/%d) of the time\n", tab.DashboardName, tab.TabName,
		100*tab.FailureRate, len(tab.failedBuilds), len(tab.allBuilds))
	if len(tab.failedBuilds) > 0 {
		tab.IssueBody += "<details>\n<summary><b>Recent failed test logs</b></summary>\n"
		for _, header := range headerData.Headers {
			if _, found := tab.failedBuilds[header.Build]; found {
				tab.IssueBody += fmt.Sprintf("\n* %s", buildLogURL(tab.TabName, header))
			}
		}
		tab.IssueBody += "\n</details>\n<details>\n<summary><b>Failed tests</b></summary>\n"
		for _, t := range tab.TestsWithFailures {
			tab.IssueBody += fmt.Sprintf("\n* %s failed %.1f%% (%d/%d) of the time", t.FullName, t.FailureRate*100, t.FailedRuns, t.TotalRuns)
		}
		tab.IssueBody += "\n</details>\n"
	}
	fmt.Println(tab.IssueBody)
}

func processRow(dashboard, tab string, row *apipb.ListRowsResponse_Row, allTests []string, headers []*apipb.ListHeadersResponse_Header) *TestResultSummary {
	t := TestResultSummary{Name: shortenTestName(row.Name), FullName: row.Name}
	// we do not want to create issues for a parent test.
	if isParentTest(row.Name, allTests) {
		return &t
	}
	if !strings.HasPrefix(row.Name, "go.etcd.io") {
		return &t
	}
	earliestTimeToConsider := time.Now().AddDate(0, 0, -1*maxDays)
	total := 0
	failed := 0
	allBuilds := map[string]struct{}{}
	failedBuilds := map[string]struct{}{}
	logs := []string{}
	for i, cell := range row.Cells {
		// ignore tests with status not in the validTestStatuses
		// cell result codes are listed in https://github.com/GoogleCloudPlatform/testgrid/blob/main/pb/test_status/test_status.proto
		if _, ok := validTestStatusesInt[cell.Result]; !ok {
			if cell.Result != 0 {
				skippedTestStatuses[cell.Result] = struct{}{}
			}
			continue
		}
		header := headers[i]
		if maxDays > 0 && header.Started.AsTime().Before(earliestTimeToConsider) {
			continue
		}
		total++
		allBuilds[header.Build] = struct{}{}
		if _, ok := failureTestStatusesInt[cell.Result]; ok {
			failed++
			failedBuilds[header.Build] = struct{}{}
			// markdown table format of | commit | log |
			logs = append(logs, fmt.Sprintf("| %s | %s | %s |", strings.Join(header.Extra, ","), header.Started.AsTime().String(), buildLogURL(tab, header)))
		}
		if maxRuns > 0 && total >= maxRuns {
			break
		}
	}
	t.FailedRuns = failed
	t.TotalRuns = total
	t.FailureLogs = logs
	t.FailureRate = float32(failed) / float32(total)
	t.failedBuilds = failedBuilds
	t.allBuilds = allBuilds
	if t.FailedRuns > 0 {
		t.IssueBody = fmt.Sprintf("## %s Test: %s \nTest failed %.1f%% (%d/%d) of the time\n\n<details>\n<summary><b>failure logs:</b></summary>\n\n| commit | started | log |\n| --- | --- | --- |\n%s\n",
			dashboardTabURL(dashboard, tab), t.FullName, t.FailureRate*100, t.FailedRuns, t.TotalRuns, strings.Join(t.FailureLogs, "\n"))
		t.IssueBody += "\n</details>\n\nPlease follow the [instructions in the contributing guide](https://github.com/etcd-io/etcd/blob/main/CONTRIBUTING.md#check-for-flaky-tests) to reproduce the issue.\n"
	}
	return &t
}

// isParentTest checks if a test is a rollup of some child tests.
func isParentTest(test string, allTests []string) bool {
	for _, t := range allTests {
		if t != test && strings.HasPrefix(t, test+"/") {
			return true
		}
	}
	return false
}

func fetchJSON(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching test data:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	testBody, _ := io.ReadAll(resp.Body)
	return testBody
}

// intStatusSet converts a list of statuspb.TestStatus into a set of int.
func intStatusSet(statuses []statuspb.TestStatus) map[int32]struct{} {
	s := make(map[int32]struct{})
	for _, status := range statuses {
		s[int32(status)] = struct{}{}
	}
	return s
}

func shortenTestName(fullname string) string {
	parts := strings.Split(fullname, "/")
	keepParts := []string{}
	// keep the package name of the test.
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		keepParts = append([]string{part}, keepParts...)
		if strings.Contains(part, ".") {
			break
		}
	}
	return strings.Join(keepParts, "/")
}

func mergeMaps(from, to map[string]struct{}) {
	for k, v := range from {
		to[k] = v
	}
}

func dashboardTabURL(dashboard, tab string) string {
	return fmt.Sprintf("[%s](https://testgrid.k8s.io/%s#%s)", tab, dashboard, tab)
}

func buildLogURL(tab string, header *apipb.ListHeadersResponse_Header) string {
	return fmt.Sprintf("https://prow.k8s.io/view/gs/kubernetes-jenkins/logs/%s/%s", tab, header.Build)
}
