// Copyright 2026 The etcd Authors
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
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	benchmarkreport "go.etcd.io/etcd/pkg/v3/report"
)

func TestDetectedWriteRateUsesCommandRateFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "put"}
	cmd.Flags().Int("rate", 0, "")
	if err := cmd.Flags().Set("rate", "1000"); err != nil {
		t.Fatalf("failed to set rate flag: %v", err)
	}

	if got := detectedWriteRate(cmd, "step-1"); got != 1000 {
		t.Fatalf("detectedWriteRate = %v, want 1000", got)
	}
}

func TestDetectedWriteRateFallsBackToStepName(t *testing.T) {
	cmd := &cobra.Command{Use: "put"}
	cmd.Flags().Int("rate", 0, "")

	if got := detectedWriteRate(cmd, "rate-4000-writes-sec"); got != 4000 {
		t.Fatalf("detectedWriteRate = %v, want 4000", got)
	}
}

func TestBenchmarkReportFromStats(t *testing.T) {
	stats := benchmarkreport.Stats{
		Lats:    []float64{0.001, 0.002, 0.003, 0.004, 0.005},
		Total:   time.Second,
		Fastest: 0.001,
		Average: 0.003,
		Stddev:  0.0014,
		Slowest: 0.005,
		RPS:     100,
		ErrorDist: map[string]int{
			"deadline exceeded": 1,
		},
	}

	got := benchmarkReportFromStats("put", stats)
	if got.Operation != "put" {
		t.Fatalf("Operation = %q, want %q", got.Operation, "put")
	}
	if got.Requests != 5 {
		t.Fatalf("Requests = %d, want 5", got.Requests)
	}
	if got.TotalSeconds != stats.Total.Seconds() {
		t.Fatalf("TotalSeconds = %v, want %v", got.TotalSeconds, stats.Total.Seconds())
	}
	if got.FastestSeconds != 0.001 {
		t.Fatalf("FastestSeconds = %v, want 0.001", got.FastestSeconds)
	}
	if got.StddevSeconds == 0 {
		t.Fatalf("StddevSeconds = 0, want non-zero")
	}
	if got.P10Seconds != 0.001 {
		t.Fatalf("P10Seconds = %v, want 0.001", got.P10Seconds)
	}
	if got.P25Seconds != 0.002 {
		t.Fatalf("P25Seconds = %v, want 0.002", got.P25Seconds)
	}
	if got.P75Seconds != 0.005 {
		t.Fatalf("P75Seconds = %v, want 0.005", got.P75Seconds)
	}
	if got.P95Seconds != 0.005 {
		t.Fatalf("P95Seconds = %v, want 0.005", got.P95Seconds)
	}
	if got.P99Seconds != 0.005 {
		t.Fatalf("P99Seconds = %v, want 0.005", got.P99Seconds)
	}
	if got.Errors["deadline exceeded"] != 1 {
		t.Fatalf("Errors = %v, want deadline exceeded count", got.Errors)
	}
}

func TestWriteMetricsElbowGraphDetectsWALThreshold(t *testing.T) {
	dir := t.TempDir()
	first := testRunMetricsReport("ramp-a", 500, 0.004, 0.010, time.Unix(1, 0))
	second := testRunMetricsReport("ramp-a", 1000, 0.006, 0.030, time.Unix(2, 0))
	writeRunMetricsReportForTest(t, filepath.Join(dir, "step-500.json"), first)
	secondPath := filepath.Join(dir, "step-1000.json")
	writeRunMetricsReportForTest(t, secondPath, second)

	outputPath := filepath.Join(dir, "elbow.html")
	gotPath, err := writeMetricsElbowGraph(outputPath, second, secondPath)
	if err != nil {
		t.Fatalf("writeMetricsElbowGraph returned error: %v", err)
	}
	if gotPath != outputPath {
		t.Fatalf("writeMetricsElbowGraph path = %q, want %q", gotPath, outputPath)
	}

	payload, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read graph output: %v", err)
	}
	html := string(payload)
	if !strings.Contains(html, "Elbow reached at 1000 writes/sec") {
		t.Fatalf("graph did not report expected elbow status:\n%s", html)
	}
	if !strings.Contains(html, "WAL fsync P99 30.00 ms crossed 25.00 ms") {
		t.Fatalf("graph did not report WAL threshold reason:\n%s", html)
	}
}

func TestWriteMetricsElbowGraphFiltersRunGroup(t *testing.T) {
	dir := t.TempDir()
	current := testRunMetricsReport("target-ramp", 1000, 0.004, 0.010, time.Unix(2, 0))
	other := testRunMetricsReport("other-ramp", 2000, 0.004, 0.050, time.Unix(1, 0))
	writeRunMetricsReportForTest(t, filepath.Join(dir, "current.json"), current)
	writeRunMetricsReportForTest(t, filepath.Join(dir, "other.json"), other)

	outputPath := filepath.Join(dir, "elbow.html")
	if _, err := writeMetricsElbowGraph(outputPath, current, filepath.Join(dir, "current.json")); err != nil {
		t.Fatalf("writeMetricsElbowGraph returned error: %v", err)
	}

	payload, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read graph output: %v", err)
	}
	if strings.Contains(string(payload), "2000 writes/sec") {
		t.Fatalf("graph included a different run group:\n%s", string(payload))
	}
}

func TestWriteMetricsSummaryTableIncludesBenchmarkDistribution(t *testing.T) {
	dir := t.TempDir()
	current := testRunMetricsReport("target-ramp", 1000, 0.004, 0.010, time.Unix(2, 0))
	currentPath := filepath.Join(dir, "current.json")
	writeRunMetricsReportForTest(t, currentPath, current)

	outputPath := filepath.Join(dir, "summary.md")
	gotPath, err := writeMetricsSummaryTable(outputPath, current, currentPath)
	if err != nil {
		t.Fatalf("writeMetricsSummaryTable returned error: %v", err)
	}
	if gotPath != outputPath {
		t.Fatalf("writeMetricsSummaryTable path = %q, want %q", gotPath, outputPath)
	}

	payload, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read summary output: %v", err)
	}
	markdown := string(payload)
	for _, want := range []string{
		"| Step | Operation | --endpoints | --clients | --conns | --rate | --total | --key-space-size | --capture-run-metrics | Requests | Requests/sec | Avg | P50 | P90 | P99 | P999 | Max | WAL P99 | Backend P99 | Pending max | Leader delta | DB growth | CPU max | Disk write | All-host Net TX | All-host Net RX | All-host TX bandwidth | All-host RX bandwidth | All-host TX pps | All-host RX pps | All-host packet drops | All-host drops/sec | Elbow note |",
		"| 1000 writes/sec | put | 127.0.0.1:2379 | 1024 | 10000 | 1000 | 300000 | 1 | true | 100 | 1000.00 | 1.00 ms | 2.00 ms | 3.00 ms | 4.00 ms | 5.00 ms | 6.00 ms | 10.00 ms | 5.00 ms | 0 | 0 | 0 B | 10.0% | 1.00 KiB | 2.00 KiB | 3.00 KiB | 512 B/s | 1.50 KiB/s | 64.50/s | 128.25/s | 3 | 0.05/s | - |",
		"## Per-Node Host Metrics",
		"| Step | Host | Role | CPU max | Memory max | Disk write | Net TX | Net RX | TX bandwidth | RX bandwidth | TX pps | RX pps | RX drops | TX drops | Drops/sec |",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("summary table missing %q:\n%s", want, markdown)
		}
	}
}

func TestWriteMetricsFullSummaryIncludesAllScenarioRunGroups(t *testing.T) {
	dir := t.TempDir()
	put := testRunMetricsReport("kilt-capacity-put", 500, 0.004, 0.010, time.Unix(1, 0))
	txn := testRunMetricsReport("kilt-capacity-txn-put", 1000, 0.006, 0.020, time.Unix(2, 0))
	other := testRunMetricsReport("other-capacity-put", 2000, 0.010, 0.030, time.Unix(3, 0))
	putPath := filepath.Join(dir, "put.json")
	writeRunMetricsReportForTest(t, putPath, put)
	writeRunMetricsReportForTest(t, filepath.Join(dir, "txn.json"), txn)
	writeRunMetricsReportForTest(t, filepath.Join(dir, "other.json"), other)

	outputPath := filepath.Join(dir, "full-summary.md")
	gotPath, err := writeMetricsFullSummaryMarkdown(outputPath, put, putPath, "kilt-capacity")
	if err != nil {
		t.Fatalf("writeMetricsFullSummaryMarkdown returned error: %v", err)
	}
	if gotPath != outputPath {
		t.Fatalf("writeMetricsFullSummaryMarkdown path = %q, want %q", gotPath, outputPath)
	}

	payload, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read full summary output: %v", err)
	}
	markdown := string(payload)
	for _, want := range []string{
		"| Scenario | Step | Operation | Total | Requests | Requests/sec | Fastest | Avg | Stddev | P10 | P25 | P50 | P75 | P90 | P95 | P99 | P999 | Max | WAL P99 | Backend P99 | Pending max | Leader delta | DB growth | CPU max | Disk write | Net TX | Net RX | TX pps | RX pps | Drops | Warnings |",
		"| put | 500 writes/sec | put | 60.0000 secs | 100 | 500.00 | 0.50 ms | 1.00 ms | 0.10 ms | 0.70 ms | 0.80 ms | 2.00 ms | 3.50 ms | 3.00 ms | 3.75 ms | 4.00 ms | 5.00 ms | 6.00 ms | 10.00 ms | 5.00 ms | 0 | 0 | 0 B | 10.0% | 1.00 KiB | 2.00 KiB | 3.00 KiB | 64.50/s | 128.25/s | 3 | 0 |",
		"| txn-put | 1000 writes/sec | put | 60.0000 secs | 100 | 1000.00 | 0.50 ms | 1.00 ms | 0.10 ms | 0.70 ms | 0.80 ms | 2.00 ms | 3.50 ms | 3.00 ms | 3.75 ms | 6.00 ms | 5.00 ms | 6.00 ms | 20.00 ms | 10.00 ms | 0 | 0 | 0 B | 10.0% | 1.00 KiB | 2.00 KiB | 3.00 KiB | 64.50/s | 128.25/s | 3 | 0 |",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("full summary missing %q:\n%s", want, markdown)
		}
	}
	if strings.Contains(markdown, "other-capacity") {
		t.Fatalf("full summary included unrelated run group:\n%s", markdown)
	}
}

func TestInputParametersFromArgs(t *testing.T) {
	got := inputParametersFromArgs([]string{
		"put",
		"--endpoints=127.0.0.1:2379",
		"--clients",
		"1024",
		"--capture-run-metrics",
		"--rate",
		"1000",
	})

	want := map[string]string{
		"endpoints":           "127.0.0.1:2379",
		"clients":             "1024",
		"capture-run-metrics": "true",
		"rate":                "1000",
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			t.Fatalf("inputParametersFromArgs[%q] = %q, want %q; got %v", key, got[key], wantValue, got)
		}
	}
}

func testRunMetricsReport(runGroup string, writeRate float64, benchmarkP99Seconds float64, walP99Seconds float64, startedAt time.Time) RunMetricsReport {
	return RunMetricsReport{
		Version:   "v3",
		Command:   "benchmark put",
		RunGroup:  runGroup,
		WriteRate: writeRate,
		InputParameters: map[string]string{
			"endpoints":           "127.0.0.1:2379",
			"clients":             "1024",
			"conns":               "10000",
			"rate":                strconv.FormatFloat(writeRate, 'f', -1, 64),
			"total":               "300000",
			"key-space-size":      "1",
			"capture-run-metrics": "true",
		},
		StartedAt: startedAt.UTC(),
		EndedAt:   startedAt.Add(time.Minute).UTC(),
		Benchmark: []BenchmarkReport{
			{
				Operation:         "put",
				Requests:          100,
				TotalSeconds:      60,
				FastestSeconds:    0.0005,
				AverageSeconds:    0.001,
				StddevSeconds:     0.0001,
				P10Seconds:        0.0007,
				P25Seconds:        0.0008,
				P50Seconds:        0.002,
				P75Seconds:        0.0035,
				P90Seconds:        0.003,
				P95Seconds:        0.00375,
				P99Seconds:        benchmarkP99Seconds,
				P999Seconds:       0.005,
				MaxSeconds:        0.006,
				RequestsPerSecond: writeRate,
			},
		},
		Members: []MemberMetricsReport{
			{
				MetricsURL: "http://127.0.0.1:2381/metrics",
				Summary: MemberMetricsSummary{
					SampleCount: 1,
					WALFsyncSecondsMax: LatencyPercentiles{
						P99Seconds: walP99Seconds,
					},
					BackendCommitSecondsMax: LatencyPercentiles{
						P99Seconds: walP99Seconds / 2,
					},
				},
			},
		},
		Host: HostMetricsReport{
			Summary: HostMetricsSummary{
				SampleCount:              1,
				CPUPercentMax:            10,
				DiskWriteBytesDelta:      1024,
				NetworkBytesSentDelta:    2048,
				NetworkBytesRecvDelta:    3072,
				NetworkBytesSentPerSec:   512,
				NetworkBytesRecvPerSec:   1536,
				NetworkPacketsSentPerSec: 64.5,
				NetworkPacketsRecvPerSec: 128.25,
				NetworkDropTotalDelta:    3,
				NetworkDropTotalPerSec:   0.05,
			},
		},
	}
}

func writeRunMetricsReportForTest(t *testing.T, path string, report RunMetricsReport) {
	t.Helper()
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed to marshal report: %v", err)
	}
	if err = os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("failed to write report: %v", err)
	}
}
