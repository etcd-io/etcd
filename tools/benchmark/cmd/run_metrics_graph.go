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
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	walFsyncP99ElbowThresholdSeconds = 0.025
	benchmarkP99ElbowGrowthRatio     = 2.0
)

type metricsElbowPoint struct {
	Report                   RunMetricsReport
	Label                    string
	BenchmarkOperation       string
	BenchmarkRequests        int
	BenchmarkAverageMS       float64
	BenchmarkP50MS           float64
	BenchmarkP90MS           float64
	BenchmarkP99MS           float64
	BenchmarkP999MS          float64
	BenchmarkMaxMS           float64
	BenchmarkRPS             float64
	WALFsyncP99MS            float64
	BackendCommitP99MS       float64
	ProposalsPendingMax      float64
	LeaderChangesDelta       float64
	DBSizeDeltaBytes         float64
	CPUPercentMax            float64
	DiskWriteBytesDelta      uint64
	NetworkSentDelta         uint64
	NetworkRecvDelta         uint64
	NetworkSentBytesPerSec   float64
	NetworkRecvBytesPerSec   float64
	NetworkPacketsSentPerSec float64
	NetworkPacketsRecvPerSec float64
	NetworkDropTotalDelta    uint64
	NetworkDropTotalPerSec   float64
	ElbowReason              string
}

func detectedWriteRate(cmd *cobra.Command, stepName string) float64 {
	for _, flagName := range []string{"rate", "put-rate"} {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			continue
		}
		rate, err := strconv.ParseFloat(strings.TrimSpace(flag.Value.String()), 64)
		if err == nil && rate > 0 {
			return rate
		}
	}
	return writeRateFromStepName(stepName)
}

func writeRateFromStepName(stepName string) float64 {
	lowerStepName := strings.ToLower(stepName)
	if !strings.Contains(lowerStepName, "rate") &&
		!strings.Contains(lowerStepName, "write") &&
		!strings.Contains(lowerStepName, "wps") &&
		!strings.Contains(lowerStepName, "sec") {
		return 0
	}
	parts := strings.FieldsFunc(lowerStepName, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	for _, part := range parts {
		rate, err := strconv.ParseFloat(part, 64)
		if err == nil && rate > 0 {
			return rate
		}
	}
	return 0
}

func commandInputParameters(cmd *cobra.Command) map[string]string {
	params := make(map[string]string)
	collectChangedFlags := func(flagSet *pflag.FlagSet) {
		if flagSet == nil {
			return
		}
		flagSet.VisitAll(func(flag *pflag.Flag) {
			if !flag.Changed || flag.Name == "help" {
				return
			}
			params[flag.Name] = flag.Value.String()
		})
	}

	collectChangedFlags(cmd.InheritedFlags())
	collectChangedFlags(cmd.PersistentFlags())
	collectChangedFlags(cmd.Flags())
	if len(params) == 0 {
		return nil
	}
	return params
}

func inputParametersForReport(report RunMetricsReport) map[string]string {
	if len(report.InputParameters) > 0 {
		return report.InputParameters
	}
	return inputParametersFromArgs(report.Args)
}

func inputParametersFromArgs(args []string) map[string]string {
	params := make(map[string]string)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		nameValue := strings.TrimPrefix(arg, "--")
		name, value, found := strings.Cut(nameValue, "=")
		if found {
			params[name] = value
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			params[name] = args[i+1]
			i++
			continue
		}
		params[name] = "true"
	}
	if len(params) == 0 {
		return nil
	}
	return params
}

func writeMetricsElbowGraph(outputPath string, current RunMetricsReport, currentArtifactPath string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return "", nil
	}

	artifactDir := filepath.Dir(outputPath)
	if currentArtifactPath != "" {
		artifactDir = filepath.Dir(currentArtifactPath)
	}
	reports, err := loadRunMetricsReportsForGraph(artifactDir, current)
	if err != nil {
		return "", err
	}
	if !containsRunMetricsReport(reports, current) {
		reports = append(reports, current)
	}

	points := metricsElbowPoints(reports)
	payload := renderMetricsElbowGraphHTML(current, points)
	if err = os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", err
	}
	if err = os.WriteFile(outputPath, []byte(payload), 0o644); err != nil {
		return "", err
	}
	return outputPath, nil
}

func writeMetricsSummaryTable(outputPath string, current RunMetricsReport, currentArtifactPath string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return "", nil
	}

	artifactDir := filepath.Dir(outputPath)
	if currentArtifactPath != "" {
		artifactDir = filepath.Dir(currentArtifactPath)
	}
	reports, err := loadRunMetricsReportsForGraph(artifactDir, current)
	if err != nil {
		return "", err
	}
	if !containsRunMetricsReport(reports, current) {
		reports = append(reports, current)
	}

	points := metricsElbowPoints(reports)
	payload := renderMetricsSummaryMarkdown(current, points)
	if err = os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", err
	}
	if err = os.WriteFile(outputPath, []byte(payload), 0o644); err != nil {
		return "", err
	}
	return outputPath, nil
}

func writeMetricsFullSummaryMarkdown(outputPath string, current RunMetricsReport, currentArtifactPath string, aggregateRunGroup string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return "", nil
	}

	artifactDir := filepath.Dir(outputPath)
	if currentArtifactPath != "" {
		artifactDir = filepath.Dir(currentArtifactPath)
	}
	reports, err := loadRunMetricsReportsForAggregate(artifactDir, current, aggregateRunGroup)
	if err != nil {
		return "", err
	}
	if !containsRunMetricsReport(reports, current) {
		reports = append(reports, current)
	}

	payload := renderFullSummaryMarkdown(current, metricsElbowPoints(reports), aggregateRunGroup)
	if err = os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", err
	}
	if err = os.WriteFile(outputPath, []byte(payload), 0o644); err != nil {
		return "", err
	}
	return outputPath, nil
}

func writeMetricsFullSummaryHTML(outputPath string, current RunMetricsReport, currentArtifactPath string, aggregateRunGroup string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return "", nil
	}

	artifactDir := filepath.Dir(outputPath)
	if currentArtifactPath != "" {
		artifactDir = filepath.Dir(currentArtifactPath)
	}
	reports, err := loadRunMetricsReportsForAggregate(artifactDir, current, aggregateRunGroup)
	if err != nil {
		return "", err
	}
	if !containsRunMetricsReport(reports, current) {
		reports = append(reports, current)
	}

	payload := renderFullSummaryHTML(current, metricsElbowPoints(reports), aggregateRunGroup)
	if err = os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", err
	}
	if err = os.WriteFile(outputPath, []byte(payload), 0o644); err != nil {
		return "", err
	}
	return outputPath, nil
}

func loadRunMetricsReportsForGraph(dir string, current RunMetricsReport) ([]RunMetricsReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	reports := make([]RunMetricsReport, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		payload, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var report RunMetricsReport
		if err = json.Unmarshal(payload, &report); err != nil {
			continue
		}
		if report.Version == "" || report.Command == "" {
			continue
		}
		if !runMetricsReportMatchesGraph(report, current) {
			continue
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func loadRunMetricsReportsForAggregate(dir string, current RunMetricsReport, aggregateRunGroup string) ([]RunMetricsReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	aggregateRunGroup = strings.TrimSpace(aggregateRunGroup)
	if aggregateRunGroup == "" {
		aggregateRunGroup = rootRunGroup(current.RunGroup)
	}

	reports := make([]RunMetricsReport, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		payload, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var report RunMetricsReport
		if err = json.Unmarshal(payload, &report); err != nil {
			continue
		}
		if report.Version == "" || report.Command == "" {
			continue
		}
		if !runMetricsReportMatchesAggregate(report, aggregateRunGroup) {
			continue
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func runMetricsReportMatchesGraph(report RunMetricsReport, current RunMetricsReport) bool {
	if current.RunGroup != "" {
		return report.RunGroup == current.RunGroup
	}
	return report.Command == current.Command
}

func runMetricsReportMatchesAggregate(report RunMetricsReport, aggregateRunGroup string) bool {
	if aggregateRunGroup == "" {
		return true
	}
	return report.RunGroup == aggregateRunGroup || strings.HasPrefix(report.RunGroup, aggregateRunGroup+"-")
}

func rootRunGroup(runGroup string) string {
	runGroup = strings.TrimSpace(runGroup)
	for _, suffix := range []string{"-watch-latency", "-txn-put", "-mixed", "-range", "-put"} {
		if strings.HasSuffix(runGroup, suffix) {
			return strings.TrimSuffix(runGroup, suffix)
		}
	}
	return runGroup
}

func containsRunMetricsReport(reports []RunMetricsReport, current RunMetricsReport) bool {
	for _, report := range reports {
		if report.StartedAt.Equal(current.StartedAt) && report.Command == current.Command && report.StepName == current.StepName {
			return true
		}
	}
	return false
}

func metricsElbowPoints(reports []RunMetricsReport) []metricsElbowPoint {
	points := make([]metricsElbowPoint, 0, len(reports))
	for _, report := range reports {
		points = append(points, metricsElbowPointFromReport(report))
	}
	sort.Slice(points, func(i, j int) bool {
		leftRate := points[i].Report.WriteRate
		rightRate := points[j].Report.WriteRate
		if leftRate > 0 && rightRate > 0 && leftRate != rightRate {
			return leftRate < rightRate
		}
		return points[i].Report.StartedAt.Before(points[j].Report.StartedAt)
	})
	annotateMetricsElbow(points)
	return points
}

func metricsElbowPointFromReport(report RunMetricsReport) metricsElbowPoint {
	point := metricsElbowPoint{
		Report: report,
		Label:  metricsElbowPointLabel(report),
	}
	for _, benchmark := range report.Benchmark {
		if benchmark.P99Seconds > point.BenchmarkP99MS/1000 {
			point.BenchmarkOperation = benchmark.Operation
			point.BenchmarkRequests = benchmark.Requests
			point.BenchmarkAverageMS = benchmark.AverageSeconds * 1000
			point.BenchmarkP50MS = benchmark.P50Seconds * 1000
			point.BenchmarkP90MS = benchmark.P90Seconds * 1000
			point.BenchmarkP99MS = benchmark.P99Seconds * 1000
			point.BenchmarkP999MS = benchmark.P999Seconds * 1000
			point.BenchmarkMaxMS = benchmark.MaxSeconds * 1000
			point.BenchmarkRPS = benchmark.RequestsPerSecond
		}
	}
	for _, member := range report.Members {
		summary := member.Summary
		point.WALFsyncP99MS = maxFloat64(point.WALFsyncP99MS, summary.WALFsyncSecondsMax.P99Seconds*1000)
		point.BackendCommitP99MS = maxFloat64(point.BackendCommitP99MS, summary.BackendCommitSecondsMax.P99Seconds*1000)
		point.ProposalsPendingMax = maxFloat64(point.ProposalsPendingMax, summary.ProposalsPendingMax)
		point.LeaderChangesDelta += summary.LeaderChangesSeenTotalDelta
		point.DBSizeDeltaBytes = maxFloat64(point.DBSizeDeltaBytes, summary.DBTotalSizeInBytesDelta)
	}
	hostSummary := aggregateHostMetricsSummary(report)
	point.CPUPercentMax = hostSummary.CPUPercentMax
	point.DiskWriteBytesDelta = hostSummary.DiskWriteBytesDelta
	point.NetworkSentDelta = hostSummary.NetworkBytesSentDelta
	point.NetworkRecvDelta = hostSummary.NetworkBytesRecvDelta
	point.NetworkSentBytesPerSec = hostSummary.NetworkBytesSentPerSec
	point.NetworkRecvBytesPerSec = hostSummary.NetworkBytesRecvPerSec
	point.NetworkPacketsSentPerSec = hostSummary.NetworkPacketsSentPerSec
	point.NetworkPacketsRecvPerSec = hostSummary.NetworkPacketsRecvPerSec
	point.NetworkDropTotalDelta = hostSummary.NetworkDropTotalDelta
	point.NetworkDropTotalPerSec = hostSummary.NetworkDropTotalPerSec
	return point
}

func aggregateHostMetricsSummary(report RunMetricsReport) HostMetricsSummary {
	var summary HostMetricsSummary
	add := func(hostSummary HostMetricsSummary) {
		if hostSummary.SampleCount == 0 {
			return
		}
		summary.SampleCount += hostSummary.SampleCount
		summary.CPUPercentMax = maxFloat64(summary.CPUPercentMax, hostSummary.CPUPercentMax)
		summary.MemoryUsedBytesMax = maxUint64(summary.MemoryUsedBytesMax, hostSummary.MemoryUsedBytesMax)
		summary.MemoryUsedPercentMax = maxFloat64(summary.MemoryUsedPercentMax, hostSummary.MemoryUsedPercentMax)
		summary.DiskReadBytesDelta += hostSummary.DiskReadBytesDelta
		summary.DiskWriteBytesDelta += hostSummary.DiskWriteBytesDelta
		summary.DiskReadCountDelta += hostSummary.DiskReadCountDelta
		summary.DiskWriteCountDelta += hostSummary.DiskWriteCountDelta
		summary.NetworkBytesSentDelta += hostSummary.NetworkBytesSentDelta
		summary.NetworkBytesRecvDelta += hostSummary.NetworkBytesRecvDelta
		summary.NetworkPacketsSentDelta += hostSummary.NetworkPacketsSentDelta
		summary.NetworkPacketsRecvDelta += hostSummary.NetworkPacketsRecvDelta
		summary.NetworkErrinDelta += hostSummary.NetworkErrinDelta
		summary.NetworkErroutDelta += hostSummary.NetworkErroutDelta
		summary.NetworkDropinDelta += hostSummary.NetworkDropinDelta
		summary.NetworkDropoutDelta += hostSummary.NetworkDropoutDelta
		summary.NetworkDropTotalDelta += hostSummary.NetworkDropTotalDelta
		summary.NetworkBytesSentPerSec += hostSummary.NetworkBytesSentPerSec
		summary.NetworkBytesRecvPerSec += hostSummary.NetworkBytesRecvPerSec
		summary.NetworkPacketsSentPerSec += hostSummary.NetworkPacketsSentPerSec
		summary.NetworkPacketsRecvPerSec += hostSummary.NetworkPacketsRecvPerSec
		summary.NetworkDropinPerSec += hostSummary.NetworkDropinPerSec
		summary.NetworkDropoutPerSec += hostSummary.NetworkDropoutPerSec
		summary.NetworkDropTotalPerSec += hostSummary.NetworkDropTotalPerSec
	}
	add(report.Host.Summary)
	for _, host := range report.RemoteHosts {
		add(host.Summary)
	}
	return summary
}

func metricsElbowPointLabel(report RunMetricsReport) string {
	if report.WriteRate > 0 {
		return formatWriteRate(report.WriteRate)
	}
	if report.StepName != "" {
		return report.StepName
	}
	if !report.StartedAt.IsZero() {
		return report.StartedAt.Format(time.RFC3339)
	}
	return "unknown step"
}

func annotateMetricsElbow(points []metricsElbowPoint) {
	for i := range points {
		reasons := make([]string, 0, 2)
		if points[i].WALFsyncP99MS >= walFsyncP99ElbowThresholdSeconds*1000 {
			reasons = append(reasons, fmt.Sprintf("WAL fsync P99 %.2f ms crossed %.2f ms", points[i].WALFsyncP99MS, walFsyncP99ElbowThresholdSeconds*1000))
		}
		if i > 0 && points[i-1].BenchmarkP99MS > 0 && points[i].BenchmarkP99MS > 0 {
			growthRatio := points[i].BenchmarkP99MS / points[i-1].BenchmarkP99MS
			if growthRatio >= benchmarkP99ElbowGrowthRatio {
				reasons = append(reasons, fmt.Sprintf("benchmark P99 grew %.1fx from previous step", growthRatio))
			}
		}
		if len(reasons) > 0 {
			points[i].ElbowReason = strings.Join(reasons, "; ")
			return
		}
	}
}

func renderMetricsElbowGraphHTML(current RunMetricsReport, points []metricsElbowPoint) string {
	elbowIndex := firstElbowPointIndex(points)
	status := "No elbow detected across the captured ramp steps."
	if elbowIndex >= 0 {
		status = fmt.Sprintf("Elbow reached at %s: %s.", points[elbowIndex].Label, points[elbowIndex].ElbowReason)
	}

	var sb strings.Builder
	sb.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>etcd benchmark elbow graph</title>")
	sb.WriteString("<style>")
	sb.WriteString("body{font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,\"Segoe UI\",sans-serif;margin:32px;color:#17202a;background:#f7f8fb}")
	sb.WriteString("main{max-width:1180px;margin:0 auto;background:#fff;border:1px solid #d9dee8;border-radius:8px;padding:28px;box-shadow:0 10px 30px rgba(20,32,52,.08)}")
	sb.WriteString("h1{font-size:24px;margin:0 0 8px}p{margin:6px 0;color:#465568}.status{font-size:18px;color:#17202a;margin:18px 0 20px;padding:14px 16px;background:#eef6ff;border-left:4px solid #3274d9}")
	sb.WriteString("svg{width:100%;height:auto;display:block;margin:20px 0 24px;background:#fbfcff;border:1px solid #e6eaf2;border-radius:6px}")
	sb.WriteString("table{width:100%;border-collapse:collapse;font-size:13px}th,td{padding:9px 10px;border-bottom:1px solid #e6eaf2;text-align:right}th:first-child,td:first-child{text-align:left}th{background:#f1f4f9;color:#2c3848}tr.elbow{background:#fff8e6}")
	sb.WriteString(".meta{font-size:13px;color:#5b697c}.note{max-width:280px;text-align:left}")
	sb.WriteString("</style></head><body><main>")
	sb.WriteString("<h1>etcd Benchmark Elbow Graph</h1>")
	fmt.Fprintf(&sb, "<p class=\"meta\">Command: %s</p>", html.EscapeString(current.Command))
	if current.RunGroup != "" {
		fmt.Fprintf(&sb, "<p class=\"meta\">Run group: %s</p>", html.EscapeString(current.RunGroup))
	}
	fmt.Fprintf(&sb, "<p class=\"status\">%s</p>", html.EscapeString(status))
	sb.WriteString(renderMetricsElbowSVG(points, elbowIndex))
	sb.WriteString(renderMetricsElbowTable(points))
	sb.WriteString(renderHostMetricsHTMLTable(points))
	sb.WriteString("</main></body></html>")
	return sb.String()
}

func renderMetricsSummaryMarkdown(current RunMetricsReport, points []metricsElbowPoint) string {
	elbowIndex := firstElbowPointIndex(points)
	status := "No elbow detected across the captured ramp steps."
	if elbowIndex >= 0 {
		status = fmt.Sprintf("Elbow reached at %s: %s.", points[elbowIndex].Label, points[elbowIndex].ElbowReason)
	}
	parameterNames := summaryTableParameterNames(points)

	var sb strings.Builder
	sb.WriteString("# etcd Benchmark Ramp Summary\n\n")
	fmt.Fprintf(&sb, "Command: `%s`\n\n", markdownCell(current.Command))
	if current.RunGroup != "" {
		fmt.Fprintf(&sb, "Run group: `%s`\n\n", markdownCell(current.RunGroup))
	}
	fmt.Fprintf(&sb, "Elbow: %s\n\n", markdownCell(status))

	headers := []string{
		"Step",
		"Operation",
	}
	for _, parameterName := range parameterNames {
		headers = append(headers, "--"+parameterName)
	}
	headers = append(headers, []string{
		"Requests",
		"Requests/sec",
		"Avg",
		"P50",
		"P90",
		"P99",
		"P999",
		"Max",
		"WAL P99",
		"Backend P99",
		"Pending max",
		"Leader delta",
		"DB growth",
		"CPU max",
		"Disk write",
		"All-host Net TX",
		"All-host Net RX",
		"All-host TX bandwidth",
		"All-host RX bandwidth",
		"All-host TX pps",
		"All-host RX pps",
		"All-host packet drops",
		"All-host drops/sec",
		"Elbow note",
	}...)
	sb.WriteString(markdownTableRow(headers))
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	sb.WriteString(markdownTableRow(separators))

	for _, point := range points {
		if len(point.Report.Benchmark) == 0 {
			sb.WriteString(markdownSummaryTableRow(point, BenchmarkReport{}, parameterNames))
			continue
		}
		for _, benchmark := range point.Report.Benchmark {
			sb.WriteString(markdownSummaryTableRow(point, benchmark, parameterNames))
		}
	}
	sb.WriteString(renderHostMetricsMarkdownTable(points))
	return sb.String()
}

func renderFullSummaryMarkdown(current RunMetricsReport, points []metricsElbowPoint, aggregateRunGroup string) string {
	points = sortedFullSummaryPoints(points, aggregateRunGroup)
	aggregateRunGroup = strings.TrimSpace(aggregateRunGroup)
	if aggregateRunGroup == "" {
		aggregateRunGroup = rootRunGroup(current.RunGroup)
	}

	var sb strings.Builder
	sb.WriteString("# etcd Benchmark Full Summary\n\n")
	if aggregateRunGroup != "" {
		fmt.Fprintf(&sb, "Run group prefix: `%s`\n\n", markdownCell(aggregateRunGroup))
	}
	fmt.Fprintf(&sb, "Updated: `%s`\n\n", time.Now().UTC().Format(time.RFC3339))
	sb.WriteString(fullSummaryMarkdownTable(points, aggregateRunGroup))
	return sb.String()
}

func renderFullSummaryHTML(current RunMetricsReport, points []metricsElbowPoint, aggregateRunGroup string) string {
	points = sortedFullSummaryPoints(points, aggregateRunGroup)
	aggregateRunGroup = strings.TrimSpace(aggregateRunGroup)
	if aggregateRunGroup == "" {
		aggregateRunGroup = rootRunGroup(current.RunGroup)
	}

	var sb strings.Builder
	sb.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>etcd benchmark full summary</title>")
	sb.WriteString("<style>")
	sb.WriteString("body{font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,\"Segoe UI\",sans-serif;margin:32px;color:#17202a;background:#f7f8fb}")
	sb.WriteString("main{max-width:1600px;margin:0 auto;background:#fff;border:1px solid #d9dee8;border-radius:8px;padding:28px;box-shadow:0 10px 30px rgba(20,32,52,.08)}")
	sb.WriteString("h1{font-size:24px;margin:0 0 8px}.meta{font-size:13px;color:#5b697c}")
	sb.WriteString("table{width:100%;border-collapse:collapse;font-size:12px}th,td{padding:8px 9px;border-bottom:1px solid #e6eaf2;text-align:right;white-space:nowrap}th:first-child,td:first-child,th:nth-child(2),td:nth-child(2),th:nth-child(3),td:nth-child(3){text-align:left}th{background:#f1f4f9;color:#2c3848;position:sticky;top:0}")
	sb.WriteString("</style></head><body><main>")
	sb.WriteString("<h1>etcd Benchmark Full Summary</h1>")
	if aggregateRunGroup != "" {
		fmt.Fprintf(&sb, "<p class=\"meta\">Run group prefix: %s</p>", html.EscapeString(aggregateRunGroup))
	}
	fmt.Fprintf(&sb, "<p class=\"meta\">Updated: %s</p>", html.EscapeString(time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fullSummaryHTMLTable(points, aggregateRunGroup))
	sb.WriteString("</main></body></html>")
	return sb.String()
}

func fullSummaryHeaders() []string {
	return []string{
		"Scenario",
		"Step",
		"Operation",
		"Total",
		"Requests",
		"Requests/sec",
		"Fastest",
		"Avg",
		"Stddev",
		"P10",
		"P25",
		"P50",
		"P75",
		"P90",
		"P95",
		"P99",
		"P999",
		"Max",
		"WAL P99",
		"Backend P99",
		"Pending max",
		"Leader delta",
		"DB growth",
		"CPU max",
		"Disk write",
		"Net TX",
		"Net RX",
		"TX pps",
		"RX pps",
		"Drops",
		"Warnings",
	}
}

func fullSummaryMarkdownTable(points []metricsElbowPoint, aggregateRunGroup string) string {
	headers := fullSummaryHeaders()
	var sb strings.Builder
	sb.WriteString(markdownTableRow(headers))
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	sb.WriteString(markdownTableRow(separators))
	for _, point := range points {
		benchmarks := point.Report.Benchmark
		if len(benchmarks) == 0 {
			benchmarks = []BenchmarkReport{{}}
		}
		for _, benchmark := range benchmarks {
			sb.WriteString(markdownTableRow(fullSummaryRow(point, benchmark, aggregateRunGroup)))
		}
	}
	return sb.String()
}

func fullSummaryHTMLTable(points []metricsElbowPoint, aggregateRunGroup string) string {
	var sb strings.Builder
	sb.WriteString("<table><thead><tr>")
	for _, heading := range fullSummaryHeaders() {
		fmt.Fprintf(&sb, "<th>%s</th>", html.EscapeString(heading))
	}
	sb.WriteString("</tr></thead><tbody>")
	for _, point := range points {
		benchmarks := point.Report.Benchmark
		if len(benchmarks) == 0 {
			benchmarks = []BenchmarkReport{{}}
		}
		for _, benchmark := range benchmarks {
			sb.WriteString("<tr>")
			for _, value := range fullSummaryRow(point, benchmark, aggregateRunGroup) {
				fmt.Fprintf(&sb, "<td>%s</td>", html.EscapeString(value))
			}
			sb.WriteString("</tr>")
		}
	}
	sb.WriteString("</tbody></table>")
	return sb.String()
}

func fullSummaryRow(point metricsElbowPoint, benchmark BenchmarkReport, aggregateRunGroup string) []string {
	return []string{
		scenarioLabel(point.Report, aggregateRunGroup),
		point.Label,
		emptyDash(benchmark.Operation),
		formatSeconds(benchmark.TotalSeconds),
		formatInt(benchmark.Requests),
		formatRPS(benchmark.RequestsPerSecond),
		formatMS(benchmark.FastestSeconds * 1000),
		formatMS(benchmark.AverageSeconds * 1000),
		formatMS(benchmark.StddevSeconds * 1000),
		formatMS(benchmark.P10Seconds * 1000),
		formatMS(benchmark.P25Seconds * 1000),
		formatMS(benchmark.P50Seconds * 1000),
		formatMS(benchmark.P75Seconds * 1000),
		formatMS(benchmark.P90Seconds * 1000),
		formatMS(benchmark.P95Seconds * 1000),
		formatMS(benchmark.P99Seconds * 1000),
		formatMS(benchmark.P999Seconds * 1000),
		formatMS(benchmark.MaxSeconds * 1000),
		formatMS(point.WALFsyncP99MS),
		formatMS(point.BackendCommitP99MS),
		fmt.Sprintf("%.0f", point.ProposalsPendingMax),
		fmt.Sprintf("%.0f", point.LeaderChangesDelta),
		formatBytes(point.DBSizeDeltaBytes),
		fmt.Sprintf("%.1f%%", point.CPUPercentMax),
		formatBytes(float64(point.DiskWriteBytesDelta)),
		formatBytes(float64(point.NetworkSentDelta)),
		formatBytes(float64(point.NetworkRecvDelta)),
		formatPerSecond(point.NetworkPacketsSentPerSec),
		formatPerSecond(point.NetworkPacketsRecvPerSec),
		formatUint64(point.NetworkDropTotalDelta),
		strconv.Itoa(len(point.Report.Errors)),
	}
}

func sortedFullSummaryPoints(points []metricsElbowPoint, aggregateRunGroup string) []metricsElbowPoint {
	sorted := append([]metricsElbowPoint(nil), points...)
	sort.Slice(sorted, func(i, j int) bool {
		leftScenario := scenarioLabel(sorted[i].Report, aggregateRunGroup)
		rightScenario := scenarioLabel(sorted[j].Report, aggregateRunGroup)
		if leftScenario != rightScenario {
			return leftScenario < rightScenario
		}
		leftRate := sorted[i].Report.WriteRate
		rightRate := sorted[j].Report.WriteRate
		if leftRate > 0 && rightRate > 0 && leftRate != rightRate {
			return leftRate < rightRate
		}
		return sorted[i].Report.StartedAt.Before(sorted[j].Report.StartedAt)
	})
	return sorted
}

func scenarioLabel(report RunMetricsReport, aggregateRunGroup string) string {
	runGroup := strings.TrimSpace(report.RunGroup)
	aggregateRunGroup = strings.TrimSpace(aggregateRunGroup)
	if aggregateRunGroup != "" {
		if runGroup == aggregateRunGroup {
			return "all"
		}
		if strings.HasPrefix(runGroup, aggregateRunGroup+"-") {
			return strings.TrimPrefix(runGroup, aggregateRunGroup+"-")
		}
	}
	if runGroup != "" {
		return runGroup
	}
	return report.Command
}

func summaryTableParameterNames(points []metricsElbowPoint) []string {
	seen := make(map[string]struct{})
	for _, point := range points {
		for name := range inputParametersForReport(point.Report) {
			seen[name] = struct{}{}
		}
	}

	names := make([]string, 0, len(seen))
	preferred := []string{
		"endpoints",
		"clients",
		"conns",
		"rate",
		"put-rate",
		"total",
		"put-total",
		"key-space-size",
		"key-size",
		"val-size",
		"streams",
		"watchers-per-stream",
		"range-total",
		"range-rate",
		"range-key-total",
		"range-limit",
		"range-consistency",
		"txn-ops",
		"metrics-step-name",
		"metrics-run-group",
		"metrics-output",
		"metrics-elbow-graph-output",
		"metrics-summary-table-output",
		"metrics-urls",
		"metrics-sample-interval",
		"metrics-remote-hosts",
		"metrics-remote-timeout",
		"capture-run-metrics",
	}
	for _, name := range preferred {
		if _, ok := seen[name]; ok {
			names = append(names, name)
			delete(seen, name)
		}
	}

	remaining := make([]string, 0, len(seen))
	for name := range seen {
		remaining = append(remaining, name)
	}
	sort.Strings(remaining)
	return append(names, remaining...)
}

func markdownSummaryTableRow(point metricsElbowPoint, benchmark BenchmarkReport, parameterNames []string) string {
	params := inputParametersForReport(point.Report)
	row := []string{
		point.Label,
		emptyDash(benchmark.Operation),
	}
	for _, parameterName := range parameterNames {
		row = append(row, emptyDash(params[parameterName]))
	}
	row = append(row, []string{
		formatInt(benchmark.Requests),
		formatRPS(benchmark.RequestsPerSecond),
		formatMS(benchmark.AverageSeconds * 1000),
		formatMS(benchmark.P50Seconds * 1000),
		formatMS(benchmark.P90Seconds * 1000),
		formatMS(benchmark.P99Seconds * 1000),
		formatMS(benchmark.P999Seconds * 1000),
		formatMS(benchmark.MaxSeconds * 1000),
		formatMS(point.WALFsyncP99MS),
		formatMS(point.BackendCommitP99MS),
		fmt.Sprintf("%.0f", point.ProposalsPendingMax),
		fmt.Sprintf("%.0f", point.LeaderChangesDelta),
		formatBytes(point.DBSizeDeltaBytes),
		fmt.Sprintf("%.1f%%", point.CPUPercentMax),
		formatBytes(float64(point.DiskWriteBytesDelta)),
		formatBytes(float64(point.NetworkSentDelta)),
		formatBytes(float64(point.NetworkRecvDelta)),
		formatBytesPerSecond(point.NetworkSentBytesPerSec),
		formatBytesPerSecond(point.NetworkRecvBytesPerSec),
		formatPerSecond(point.NetworkPacketsSentPerSec),
		formatPerSecond(point.NetworkPacketsRecvPerSec),
		formatUint64(point.NetworkDropTotalDelta),
		formatPerSecond(point.NetworkDropTotalPerSec),
		emptyDash(point.ElbowReason),
	}...)
	return markdownTableRow(row)
}

func firstElbowPointIndex(points []metricsElbowPoint) int {
	for i, point := range points {
		if point.ElbowReason != "" {
			return i
		}
	}
	return -1
}

func renderMetricsElbowSVG(points []metricsElbowPoint, elbowIndex int) string {
	const (
		width  = 1080.0
		height = 500.0
		left   = 76.0
		right  = 34.0
		top    = 34.0
		bottom = 82.0
	)
	plotWidth := width - left - right
	plotHeight := height - top - bottom
	yMax := walFsyncP99ElbowThresholdSeconds * 1000
	for _, point := range points {
		yMax = maxFloat64(yMax, point.BenchmarkP99MS)
		yMax = maxFloat64(yMax, point.WALFsyncP99MS)
		yMax = maxFloat64(yMax, point.BackendCommitP99MS)
	}
	if yMax <= 0 {
		yMax = 1
	}
	yMax *= 1.15

	xFor := func(i int) float64 {
		if len(points) <= 1 {
			return left + plotWidth/2
		}
		return left + float64(i)*plotWidth/float64(len(points)-1)
	}
	yFor := func(value float64) float64 {
		return top + plotHeight - (value/yMax)*plotHeight
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "<svg viewBox=\"0 0 %.0f %.0f\" role=\"img\" aria-label=\"etcd benchmark latency elbow graph\">", width, height)
	fmt.Fprintf(&sb, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#9aa7b7\"/>", left, top+plotHeight, left+plotWidth, top+plotHeight)
	fmt.Fprintf(&sb, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#9aa7b7\"/>", left, top, left, top+plotHeight)
	for i := 0; i <= 4; i++ {
		value := yMax * float64(i) / 4
		y := yFor(value)
		fmt.Fprintf(&sb, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#e2e7ef\"/>", left, y, left+plotWidth, y)
		fmt.Fprintf(&sb, "<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" text-anchor=\"end\" fill=\"#526173\">%.1f ms</text>", left-10, y+4, value)
	}

	thresholdY := yFor(walFsyncP99ElbowThresholdSeconds * 1000)
	fmt.Fprintf(&sb, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#be3a34\" stroke-dasharray=\"6 5\"/>", left, thresholdY, left+plotWidth, thresholdY)
	fmt.Fprintf(&sb, "<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" fill=\"#8f2d2a\">WAL P99 25 ms threshold</text>", left+8, thresholdY-8)

	sb.WriteString(renderMetricsLine(points, xFor, yFor, func(point metricsElbowPoint) float64 { return point.BenchmarkP99MS }, "#2457c5"))
	sb.WriteString(renderMetricsLine(points, xFor, yFor, func(point metricsElbowPoint) float64 { return point.WALFsyncP99MS }, "#c43c32"))
	sb.WriteString(renderMetricsLine(points, xFor, yFor, func(point metricsElbowPoint) float64 { return point.BackendCommitP99MS }, "#278b65"))

	for i, point := range points {
		x := xFor(i)
		fmt.Fprintf(&sb, "<text x=\"%.1f\" y=\"%.1f\" font-size=\"11\" text-anchor=\"middle\" fill=\"#526173\">%s</text>", x, top+plotHeight+26, html.EscapeString(shortGraphLabel(point.Label)))
	}
	if elbowIndex >= 0 {
		x := xFor(elbowIndex)
		fmt.Fprintf(&sb, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#a06100\" stroke-dasharray=\"4 4\"/>", x, top, x, top+plotHeight)
		fmt.Fprintf(&sb, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"7\" fill=\"#ffbf3f\" stroke=\"#7a4a00\" stroke-width=\"2\"/>", x, yFor(maxFloat64(points[elbowIndex].BenchmarkP99MS, points[elbowIndex].WALFsyncP99MS)))
		fmt.Fprintf(&sb, "<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" text-anchor=\"middle\" fill=\"#7a4a00\">elbow</text>", x, top+14)
	}

	renderLegend(&sb, left+plotWidth-284, top+16, "#2457c5", "benchmark P99")
	renderLegend(&sb, left+plotWidth-284, top+38, "#c43c32", "WAL fsync P99")
	renderLegend(&sb, left+plotWidth-284, top+60, "#278b65", "backend commit P99")
	fmt.Fprintf(&sb, "<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" text-anchor=\"middle\" fill=\"#526173\">ramp step</text>", left+plotWidth/2, height-16)
	fmt.Fprintf(&sb, "<text x=\"18\" y=\"%.1f\" font-size=\"12\" text-anchor=\"middle\" fill=\"#526173\" transform=\"rotate(-90 18 %.1f)\">latency</text>", top+plotHeight/2, top+plotHeight/2)
	sb.WriteString("</svg>")
	return sb.String()
}

func renderMetricsLine(points []metricsElbowPoint, xFor func(int) float64, yFor func(float64) float64, valueFor func(metricsElbowPoint) float64, color string) string {
	var path strings.Builder
	drewAny := false
	for i, point := range points {
		value := valueFor(point)
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		if drewAny {
			fmt.Fprintf(&path, " L %.1f %.1f", xFor(i), yFor(value))
		} else {
			fmt.Fprintf(&path, "M %.1f %.1f", xFor(i), yFor(value))
			drewAny = true
		}
	}
	if !drewAny {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "<path d=\"%s\" fill=\"none\" stroke=\"%s\" stroke-width=\"3\" stroke-linejoin=\"round\" stroke-linecap=\"round\"/>", path.String(), color)
	for i, point := range points {
		value := valueFor(point)
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		fmt.Fprintf(&sb, "<circle cx=\"%.1f\" cy=\"%.1f\" r=\"4\" fill=\"%s\"/>", xFor(i), yFor(value), color)
	}
	return sb.String()
}

func renderLegend(sb *strings.Builder, x float64, y float64, color string, label string) {
	fmt.Fprintf(sb, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"%s\" stroke-width=\"3\"/>", x, y, x+24, y, color)
	fmt.Fprintf(sb, "<text x=\"%.1f\" y=\"%.1f\" font-size=\"12\" fill=\"#334155\">%s</text>", x+32, y+4, html.EscapeString(label))
}

func renderHostMetricsMarkdownTable(points []metricsElbowPoint) string {
	if !hasAnyHostMetrics(points) {
		return ""
	}

	headers := []string{
		"Step",
		"Host",
		"Role",
		"CPU max",
		"Memory max",
		"Disk write",
		"Net TX",
		"Net RX",
		"TX bandwidth",
		"RX bandwidth",
		"TX pps",
		"RX pps",
		"RX drops",
		"TX drops",
		"Drops/sec",
	}
	var sb strings.Builder
	sb.WriteString("\n## Per-Node Host Metrics\n\n")
	sb.WriteString(markdownTableRow(headers))
	separators := make([]string, len(headers))
	for i := range separators {
		separators[i] = "---"
	}
	sb.WriteString(markdownTableRow(separators))
	for _, point := range points {
		for _, host := range hostMetricsReportsForReport(point.Report) {
			summary := host.Summary
			if summary.SampleCount == 0 {
				continue
			}
			sb.WriteString(markdownTableRow([]string{
				point.Label,
				hostReportLabel(host),
				emptyDash(host.Role),
				fmt.Sprintf("%.1f%%", summary.CPUPercentMax),
				formatBytes(float64(summary.MemoryUsedBytesMax)),
				formatBytes(float64(summary.DiskWriteBytesDelta)),
				formatBytes(float64(summary.NetworkBytesSentDelta)),
				formatBytes(float64(summary.NetworkBytesRecvDelta)),
				formatBytesPerSecond(summary.NetworkBytesSentPerSec),
				formatBytesPerSecond(summary.NetworkBytesRecvPerSec),
				formatPerSecond(summary.NetworkPacketsSentPerSec),
				formatPerSecond(summary.NetworkPacketsRecvPerSec),
				formatUint64(summary.NetworkDropinDelta),
				formatUint64(summary.NetworkDropoutDelta),
				formatPerSecond(summary.NetworkDropTotalPerSec),
			}))
		}
	}
	return sb.String()
}

func renderHostMetricsHTMLTable(points []metricsElbowPoint) string {
	if !hasAnyHostMetrics(points) {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<h2>Per-Node Host Metrics</h2>")
	sb.WriteString("<table><thead><tr>")
	for _, heading := range []string{"Step", "Host", "Role", "CPU max", "Memory max", "Disk write", "Net TX", "Net RX", "TX bandwidth", "RX bandwidth", "TX pps", "RX pps", "RX drops", "TX drops", "Drops/sec"} {
		fmt.Fprintf(&sb, "<th>%s</th>", html.EscapeString(heading))
	}
	sb.WriteString("</tr></thead><tbody>")
	for _, point := range points {
		for _, host := range hostMetricsReportsForReport(point.Report) {
			summary := host.Summary
			if summary.SampleCount == 0 {
				continue
			}
			sb.WriteString("<tr>")
			fmt.Fprintf(&sb, "<td>%s</td>", html.EscapeString(point.Label))
			fmt.Fprintf(&sb, "<td>%s</td>", html.EscapeString(hostReportLabel(host)))
			fmt.Fprintf(&sb, "<td>%s</td>", html.EscapeString(emptyDash(host.Role)))
			fmt.Fprintf(&sb, "<td>%.1f%%</td>", summary.CPUPercentMax)
			fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(float64(summary.MemoryUsedBytesMax)))
			fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(float64(summary.DiskWriteBytesDelta)))
			fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(float64(summary.NetworkBytesSentDelta)))
			fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(float64(summary.NetworkBytesRecvDelta)))
			fmt.Fprintf(&sb, "<td>%s</td>", formatBytesPerSecond(summary.NetworkBytesSentPerSec))
			fmt.Fprintf(&sb, "<td>%s</td>", formatBytesPerSecond(summary.NetworkBytesRecvPerSec))
			fmt.Fprintf(&sb, "<td>%s</td>", formatPerSecond(summary.NetworkPacketsSentPerSec))
			fmt.Fprintf(&sb, "<td>%s</td>", formatPerSecond(summary.NetworkPacketsRecvPerSec))
			fmt.Fprintf(&sb, "<td>%s</td>", formatUint64(summary.NetworkDropinDelta))
			fmt.Fprintf(&sb, "<td>%s</td>", formatUint64(summary.NetworkDropoutDelta))
			fmt.Fprintf(&sb, "<td>%s</td>", formatPerSecond(summary.NetworkDropTotalPerSec))
			sb.WriteString("</tr>")
		}
	}
	sb.WriteString("</tbody></table>")
	return sb.String()
}

func hasAnyHostMetrics(points []metricsElbowPoint) bool {
	for _, point := range points {
		for _, host := range hostMetricsReportsForReport(point.Report) {
			if host.Summary.SampleCount > 0 {
				return true
			}
		}
	}
	return false
}

func hostMetricsReportsForReport(report RunMetricsReport) []HostMetricsReport {
	hosts := make([]HostMetricsReport, 0, 1+len(report.RemoteHosts))
	if report.Host.Summary.SampleCount > 0 {
		hosts = append(hosts, report.Host)
	}
	hosts = append(hosts, report.RemoteHosts...)
	return hosts
}

func renderMetricsElbowTable(points []metricsElbowPoint) string {
	var sb strings.Builder
	sb.WriteString("<table><thead><tr>")
	for _, heading := range []string{"Step", "Benchmark op", "Benchmark P99", "WAL P99", "Backend P99", "Pending max", "Leader delta", "DB growth", "CPU max", "Disk write", "All-host Net TX", "All-host Net RX", "All-host TX bandwidth", "All-host RX bandwidth", "All-host TX pps", "All-host RX pps", "All-host packet drops", "All-host drops/sec", "Note"} {
		fmt.Fprintf(&sb, "<th>%s</th>", html.EscapeString(heading))
	}
	sb.WriteString("</tr></thead><tbody>")
	for _, point := range points {
		rowClass := ""
		if point.ElbowReason != "" {
			rowClass = " class=\"elbow\""
		}
		fmt.Fprintf(&sb, "<tr%s>", rowClass)
		fmt.Fprintf(&sb, "<td>%s</td>", html.EscapeString(point.Label))
		fmt.Fprintf(&sb, "<td>%s</td>", html.EscapeString(emptyDash(point.BenchmarkOperation)))
		fmt.Fprintf(&sb, "<td>%s</td>", formatMS(point.BenchmarkP99MS))
		fmt.Fprintf(&sb, "<td>%s</td>", formatMS(point.WALFsyncP99MS))
		fmt.Fprintf(&sb, "<td>%s</td>", formatMS(point.BackendCommitP99MS))
		fmt.Fprintf(&sb, "<td>%.0f</td>", point.ProposalsPendingMax)
		fmt.Fprintf(&sb, "<td>%.0f</td>", point.LeaderChangesDelta)
		fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(point.DBSizeDeltaBytes))
		fmt.Fprintf(&sb, "<td>%.1f%%</td>", point.CPUPercentMax)
		fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(float64(point.DiskWriteBytesDelta)))
		fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(float64(point.NetworkSentDelta)))
		fmt.Fprintf(&sb, "<td>%s</td>", formatBytes(float64(point.NetworkRecvDelta)))
		fmt.Fprintf(&sb, "<td>%s</td>", formatBytesPerSecond(point.NetworkSentBytesPerSec))
		fmt.Fprintf(&sb, "<td>%s</td>", formatBytesPerSecond(point.NetworkRecvBytesPerSec))
		fmt.Fprintf(&sb, "<td>%s</td>", formatPerSecond(point.NetworkPacketsSentPerSec))
		fmt.Fprintf(&sb, "<td>%s</td>", formatPerSecond(point.NetworkPacketsRecvPerSec))
		fmt.Fprintf(&sb, "<td>%s</td>", formatUint64(point.NetworkDropTotalDelta))
		fmt.Fprintf(&sb, "<td>%s</td>", formatPerSecond(point.NetworkDropTotalPerSec))
		fmt.Fprintf(&sb, "<td class=\"note\">%s</td>", html.EscapeString(point.ElbowReason))
		sb.WriteString("</tr>")
	}
	sb.WriteString("</tbody></table>")
	return sb.String()
}

func formatWriteRate(rate float64) string {
	if math.Abs(rate-math.Round(rate)) < 1e-9 {
		return fmt.Sprintf("%.0f writes/sec", rate)
	}
	return fmt.Sprintf("%.2f writes/sec", rate)
}

func shortGraphLabel(label string) string {
	if len(label) <= 18 {
		return label
	}
	return label[:15] + "..."
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func formatInt(value int) string {
	if value == 0 {
		return "-"
	}
	return strconv.Itoa(value)
}

func formatUint64(value uint64) string {
	if value == 0 {
		return "-"
	}
	return strconv.FormatUint(value, 10)
}

func formatRPS(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", value)
}

func formatPerSecond(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f/s", value)
}

func formatSeconds(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.4f secs", value)
}

func formatMS(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f ms", value)
}

func formatBytes(value float64) string {
	switch {
	case value >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GiB", value/(1024*1024*1024))
	case value >= 1024*1024:
		return fmt.Sprintf("%.2f MiB", value/(1024*1024))
	case value >= 1024:
		return fmt.Sprintf("%.2f KiB", value/1024)
	default:
		return fmt.Sprintf("%.0f B", value)
	}
}

func formatBytesPerSecond(value float64) string {
	if value <= 0 {
		return "-"
	}
	return formatBytes(value) + "/s"
}

func markdownTableRow(values []string) string {
	escaped := make([]string, len(values))
	for i, value := range values {
		escaped[i] = markdownCell(value)
	}
	return "| " + strings.Join(escaped, " | ") + " |\n"
}

func markdownCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}
