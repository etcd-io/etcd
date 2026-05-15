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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	netio "github.com/shirou/gopsutil/v4/net"
	"github.com/spf13/cobra"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	benchmarkreport "go.etcd.io/etcd/pkg/v3/report"
)

var (
	captureRunMetrics        bool
	metricsURLs              []string
	metricsSampleInterval    time.Duration
	metricsStepName          string
	metricsRunGroup          string
	metricsOutput            string
	metricsElbowGraph        string
	metricsSummaryTable      string
	metricsFullSummary       string
	metricsFullSummaryHTML   string
	metricsAggregateRunGroup string
	metricsRemoteHosts       []string
	metricsRemoteTimeout     time.Duration

	activeRunMetricsCollector *runMetricsCollector
	restoreStatsObserver      func()
)

type runMetricsCollector struct {
	httpClient     *http.Client
	httpTransport  *http.Transport
	memberURLs     []string
	remoteHosts    []string
	remoteTimeout  time.Duration
	sampleInterval time.Duration
	report         RunMetricsReport
	ctx            context.Context
	cancel         context.CancelFunc
	donec          chan struct{}
	sampleMu       sync.Mutex
}

type RunMetricsReport struct {
	Version         string                `json:"version"`
	Command         string                `json:"command"`
	StepName        string                `json:"stepName,omitempty"`
	RunGroup        string                `json:"runGroup,omitempty"`
	Args            []string              `json:"args,omitempty"`
	InputParameters map[string]string     `json:"inputParameters,omitempty"`
	WriteRate       float64               `json:"writeRate,omitempty"`
	StartedAt       time.Time             `json:"startedAt"`
	EndedAt         time.Time             `json:"endedAt"`
	SampleInterval  string                `json:"sampleInterval"`
	Members         []MemberMetricsReport `json:"members"`
	Host            HostMetricsReport     `json:"host"`
	RemoteHosts     []HostMetricsReport   `json:"remoteHosts,omitempty"`
	Benchmark       []BenchmarkReport     `json:"benchmark,omitempty"`
	Errors          []RunMetricsError     `json:"errors,omitempty"`
}

type RunMetricsError struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
}

type MemberMetricsReport struct {
	MetricsURL string                `json:"metricsURL"`
	Samples    []MemberMetricsSample `json:"samples"`
	Summary    MemberMetricsSummary  `json:"summary"`
}

type LatencyPercentiles struct {
	P50Seconds  float64 `json:"p50Seconds"`
	P90Seconds  float64 `json:"p90Seconds"`
	P99Seconds  float64 `json:"p99Seconds"`
	P999Seconds float64 `json:"p999Seconds"`
}

type BenchmarkReport struct {
	Operation         string         `json:"operation"`
	Requests          int            `json:"requests"`
	TotalSeconds      float64        `json:"totalSeconds"`
	FastestSeconds    float64        `json:"fastestSeconds"`
	AverageSeconds    float64        `json:"averageSeconds"`
	StddevSeconds     float64        `json:"stddevSeconds"`
	P10Seconds        float64        `json:"p10Seconds"`
	P25Seconds        float64        `json:"p25Seconds"`
	P50Seconds        float64        `json:"p50Seconds"`
	P75Seconds        float64        `json:"p75Seconds"`
	P90Seconds        float64        `json:"p90Seconds"`
	P95Seconds        float64        `json:"p95Seconds"`
	P99Seconds        float64        `json:"p99Seconds"`
	P999Seconds       float64        `json:"p999Seconds"`
	MaxSeconds        float64        `json:"maxSeconds"`
	RequestsPerSecond float64        `json:"requestsPerSecond"`
	Errors            map[string]int `json:"errors,omitempty"`
}

type MemberMetricsSample struct {
	Timestamp              time.Time          `json:"timestamp"`
	WALFsyncSeconds        LatencyPercentiles `json:"walFsyncSeconds"`
	BackendCommitSeconds   LatencyPercentiles `json:"backendCommitSeconds"`
	ProposalsPending       float64            `json:"proposalsPending"`
	LeaderChangesSeenTotal float64            `json:"leaderChangesSeenTotal"`
	DBTotalSizeInBytes     float64            `json:"dbTotalSizeInBytes"`
}

type MemberMetricsSummary struct {
	SampleCount                 int                `json:"sampleCount"`
	WALFsyncSecondsMax          LatencyPercentiles `json:"walFsyncSecondsMax"`
	BackendCommitSecondsMax     LatencyPercentiles `json:"backendCommitSecondsMax"`
	ProposalsPendingMax         float64            `json:"proposalsPendingMax"`
	LeaderChangesSeenTotalStart float64            `json:"leaderChangesSeenTotalStart"`
	LeaderChangesSeenTotalEnd   float64            `json:"leaderChangesSeenTotalEnd"`
	LeaderChangesSeenTotalDelta float64            `json:"leaderChangesSeenTotalDelta"`
	DBTotalSizeInBytesStart     float64            `json:"dbTotalSizeInBytesStart"`
	DBTotalSizeInBytesEnd       float64            `json:"dbTotalSizeInBytesEnd"`
	DBTotalSizeInBytesDelta     float64            `json:"dbTotalSizeInBytesDelta"`
	DBTotalSizeInBytesMax       float64            `json:"dbTotalSizeInBytesMax"`
}

type HostMetricsReport struct {
	Name    string              `json:"name,omitempty"`
	Target  string              `json:"target,omitempty"`
	Role    string              `json:"role,omitempty"`
	Samples []HostMetricsSample `json:"samples"`
	Summary HostMetricsSummary  `json:"summary"`
}

type HostMetricsSample struct {
	Timestamp            time.Time `json:"timestamp"`
	CPUPercent           float64   `json:"cpuPercent"`
	MemoryUsedBytes      uint64    `json:"memoryUsedBytes"`
	MemoryUsedPercent    float64   `json:"memoryUsedPercent"`
	MemoryAvailableBytes uint64    `json:"memoryAvailableBytes"`
	DiskReadBytes        uint64    `json:"diskReadBytes"`
	DiskWriteBytes       uint64    `json:"diskWriteBytes"`
	DiskReadCount        uint64    `json:"diskReadCount"`
	DiskWriteCount       uint64    `json:"diskWriteCount"`
	NetworkBytesSent     uint64    `json:"networkBytesSent"`
	NetworkBytesRecv     uint64    `json:"networkBytesRecv"`
	NetworkPacketsSent   uint64    `json:"networkPacketsSent"`
	NetworkPacketsRecv   uint64    `json:"networkPacketsRecv"`
	NetworkErrin         uint64    `json:"networkErrin"`
	NetworkErrout        uint64    `json:"networkErrout"`
	NetworkDropin        uint64    `json:"networkDropin"`
	NetworkDropout       uint64    `json:"networkDropout"`
	CPUTotalJiffies      uint64    `json:"cpuTotalJiffies,omitempty"`
	CPUIdleJiffies       uint64    `json:"cpuIdleJiffies,omitempty"`
}

type HostMetricsSummary struct {
	SampleCount              int     `json:"sampleCount"`
	CPUPercentMax            float64 `json:"cpuPercentMax"`
	MemoryUsedBytesMax       uint64  `json:"memoryUsedBytesMax"`
	MemoryUsedPercentMax     float64 `json:"memoryUsedPercentMax"`
	DiskReadBytesStart       uint64  `json:"diskReadBytesStart"`
	DiskReadBytesEnd         uint64  `json:"diskReadBytesEnd"`
	DiskReadBytesDelta       uint64  `json:"diskReadBytesDelta"`
	DiskWriteBytesStart      uint64  `json:"diskWriteBytesStart"`
	DiskWriteBytesEnd        uint64  `json:"diskWriteBytesEnd"`
	DiskWriteBytesDelta      uint64  `json:"diskWriteBytesDelta"`
	DiskReadCountDelta       uint64  `json:"diskReadCountDelta"`
	DiskWriteCountDelta      uint64  `json:"diskWriteCountDelta"`
	NetworkBytesSentStart    uint64  `json:"networkBytesSentStart"`
	NetworkBytesSentEnd      uint64  `json:"networkBytesSentEnd"`
	NetworkBytesSentDelta    uint64  `json:"networkBytesSentDelta"`
	NetworkBytesRecvStart    uint64  `json:"networkBytesRecvStart"`
	NetworkBytesRecvEnd      uint64  `json:"networkBytesRecvEnd"`
	NetworkBytesRecvDelta    uint64  `json:"networkBytesRecvDelta"`
	NetworkPacketsSentDelta  uint64  `json:"networkPacketsSentDelta"`
	NetworkPacketsRecvDelta  uint64  `json:"networkPacketsRecvDelta"`
	NetworkErrinDelta        uint64  `json:"networkErrinDelta"`
	NetworkErroutDelta       uint64  `json:"networkErroutDelta"`
	NetworkDropinDelta       uint64  `json:"networkDropinDelta"`
	NetworkDropoutDelta      uint64  `json:"networkDropoutDelta"`
	NetworkDropTotalDelta    uint64  `json:"networkDropTotalDelta"`
	NetworkBytesSentPerSec   float64 `json:"networkBytesSentPerSec"`
	NetworkBytesRecvPerSec   float64 `json:"networkBytesRecvPerSec"`
	NetworkPacketsSentPerSec float64 `json:"networkPacketsSentPerSec"`
	NetworkPacketsRecvPerSec float64 `json:"networkPacketsRecvPerSec"`
	NetworkDropinPerSec      float64 `json:"networkDropinPerSec"`
	NetworkDropoutPerSec     float64 `json:"networkDropoutPerSec"`
	NetworkDropTotalPerSec   float64 `json:"networkDropTotalPerSec"`
}

func startRunMetricsCapture(cmd *cobra.Command) error {
	if !captureRunMetrics {
		return nil
	}
	if metricsSampleInterval <= 0 {
		return fmt.Errorf("expected positive --metrics-sample-interval, got %v", metricsSampleInterval)
	}
	if activeRunMetricsCollector != nil {
		stopRunMetricsCapture()
	}

	collector, err := newRunMetricsCollector(cmd)
	if err != nil {
		return err
	}
	activeRunMetricsCollector = collector
	restoreStatsObserver = benchmarkreport.SetStatsObserver(collector.recordBenchmarkStats)
	activeRunMetricsCollector.start()
	return nil
}

func stopRunMetricsCapture() {
	if activeRunMetricsCollector == nil {
		return
	}
	collector := activeRunMetricsCollector
	activeRunMetricsCollector = nil
	if restoreStatsObserver != nil {
		restoreStatsObserver()
		restoreStatsObserver = nil
	}
	report, artifactPath, err := collector.stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write run metrics report for %q: %v\n", report.Command, err)
	}
	fmt.Print(formatRunMetricsSummary(report))
	if artifactPath != "" {
		fmt.Printf("Run metrics artifact:\t%s\n", artifactPath)
	}
	if metricsElbowGraph != "" {
		graphPath, graphErr := writeMetricsElbowGraph(metricsElbowGraph, report, artifactPath)
		if graphErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write run metrics elbow graph for %q: %v\n", report.Command, graphErr)
		} else if graphPath != "" {
			fmt.Printf("Run metrics elbow graph:\t%s\n", graphPath)
		}
	}
	if metricsSummaryTable != "" {
		tablePath, tableErr := writeMetricsSummaryTable(metricsSummaryTable, report, artifactPath)
		if tableErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write run metrics summary table for %q: %v\n", report.Command, tableErr)
		} else if tablePath != "" {
			fmt.Printf("Run metrics summary table:\t%s\n", tablePath)
		}
	}
	if metricsFullSummary != "" {
		tablePath, tableErr := writeMetricsFullSummaryMarkdown(metricsFullSummary, report, artifactPath, metricsAggregateRunGroup)
		if tableErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write full run metrics summary for %q: %v\n", report.Command, tableErr)
		} else if tablePath != "" {
			fmt.Printf("Full run metrics summary:\t%s\n", tablePath)
		}
	}
	if metricsFullSummaryHTML != "" {
		htmlPath, htmlErr := writeMetricsFullSummaryHTML(metricsFullSummaryHTML, report, artifactPath, metricsAggregateRunGroup)
		if htmlErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write full run metrics HTML summary for %q: %v\n", report.Command, htmlErr)
		} else if htmlPath != "" {
			fmt.Printf("Full run metrics HTML summary:\t%s\n", htmlPath)
		}
	}
}

func newRunMetricsCollector(cmd *cobra.Command) (*runMetricsCollector, error) {
	memberURLs, err := resolvedMetricsURLs()
	if err != nil {
		return nil, err
	}

	httpTransport, err := transport.NewTimeoutTransport(tls, 5*time.Second, 5*time.Second, 5*time.Second)
	if err != nil {
		return nil, err
	}

	stepName := strings.TrimSpace(metricsStepName)
	runGroup := strings.TrimSpace(metricsRunGroup)
	remoteHosts := normalizedRemoteHosts(metricsRemoteHosts)
	ctx, cancel := context.WithCancel(context.Background())
	localHostName, err := os.Hostname()
	if err != nil || strings.TrimSpace(localHostName) == "" {
		localHostName = "local"
	}
	report := RunMetricsReport{
		Version:         "v3",
		Command:         cmd.CommandPath(),
		StepName:        stepName,
		RunGroup:        runGroup,
		Args:            os.Args[1:],
		InputParameters: commandInputParameters(cmd),
		WriteRate:       detectedWriteRate(cmd, stepName),
		StartedAt:       time.Now().UTC(),
		SampleInterval:  metricsSampleInterval.String(),
		Members:         make([]MemberMetricsReport, len(memberURLs)),
		Host: HostMetricsReport{
			Name:   localHostName,
			Target: "local",
			Role:   "benchmark-client",
		},
		RemoteHosts: make([]HostMetricsReport, len(remoteHosts)),
	}
	for i, target := range memberURLs {
		report.Members[i] = MemberMetricsReport{MetricsURL: target}
	}
	for i, target := range remoteHosts {
		report.RemoteHosts[i] = HostMetricsReport{
			Name:   remoteHostDisplayName(target),
			Target: target,
			Role:   "remote-etcd-node",
		}
	}

	return &runMetricsCollector{
		httpClient: &http.Client{
			Transport: httpTransport,
			Timeout:   10 * time.Second,
		},
		httpTransport:  httpTransport,
		memberURLs:     memberURLs,
		remoteHosts:    remoteHosts,
		remoteTimeout:  metricsRemoteTimeout,
		sampleInterval: metricsSampleInterval,
		report:         report,
		ctx:            ctx,
		cancel:         cancel,
		donec:          make(chan struct{}),
	}, nil
}

func (c *runMetricsCollector) start() {
	c.sampleOnce()

	go func() {
		ticker := time.NewTicker(c.sampleInterval)
		defer func() {
			ticker.Stop()
			close(c.donec)
		}()

		for {
			select {
			case <-ticker.C:
				c.sampleOnce()
			case <-c.ctx.Done():
				return
			}
		}
	}()
}

func (c *runMetricsCollector) stop() (RunMetricsReport, string, error) {
	c.sampleOnce()
	c.cancel()
	<-c.donec
	if c.httpTransport != nil {
		c.httpTransport.CloseIdleConnections()
	}

	c.report.EndedAt = time.Now().UTC()
	for i := range c.report.Members {
		c.report.Members[i].Summary = summarizeMemberMetrics(c.report.Members[i].Samples)
	}
	c.report.Host.Summary = summarizeHostMetrics(c.report.Host.Samples)
	for i := range c.report.RemoteHosts {
		c.report.RemoteHosts[i].Summary = summarizeHostMetrics(c.report.RemoteHosts[i].Samples)
	}

	artifactPath, err := c.writeArtifact()
	return c.report, artifactPath, err
}

func (c *runMetricsCollector) sampleOnce() {
	c.sampleMu.Lock()
	defer c.sampleMu.Unlock()

	now := time.Now().UTC()
	for i, target := range c.memberURLs {
		sample, err := c.collectMemberMetrics(now, target)
		if err != nil {
			c.recordError(now, target, err)
			continue
		}
		c.report.Members[i].Samples = append(c.report.Members[i].Samples, sample)
	}

	hostSample, err := collectHostMetrics(now)
	if err != nil {
		c.recordError(now, "host", err)
	}
	c.report.Host.Samples = append(c.report.Host.Samples, hostSample)

	for _, result := range c.collectRemoteHostMetrics(now) {
		if result.err != nil {
			c.recordError(now, "remote host "+result.target, result.err)
			continue
		}
		c.report.RemoteHosts[result.index].Samples = append(c.report.RemoteHosts[result.index].Samples, result.sample)
	}
}

type remoteHostMetricResult struct {
	index  int
	target string
	sample HostMetricsSample
	err    error
}

func (c *runMetricsCollector) collectRemoteHostMetrics(now time.Time) []remoteHostMetricResult {
	if len(c.remoteHosts) == 0 {
		return nil
	}

	results := make([]remoteHostMetricResult, len(c.remoteHosts))
	var wg sync.WaitGroup
	for i, target := range c.remoteHosts {
		i, target := i, target
		results[i] = remoteHostMetricResult{index: i, target: target}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sample, err := collectRemoteHostMetrics(c.ctx, now, target, c.remoteTimeout)
			results[i].sample = sample
			results[i].err = err
		}()
	}
	wg.Wait()
	return results
}

func (c *runMetricsCollector) recordBenchmarkStats(operation string, stats benchmarkreport.Stats) {
	c.sampleMu.Lock()
	defer c.sampleMu.Unlock()

	c.report.Benchmark = append(c.report.Benchmark, benchmarkReportFromStats(operation, stats))
}

func benchmarkReportFromStats(operation string, stats benchmarkreport.Stats) BenchmarkReport {
	report := BenchmarkReport{
		Operation:         operation,
		Requests:          len(stats.Lats),
		TotalSeconds:      stats.Total.Seconds(),
		FastestSeconds:    stats.Fastest,
		AverageSeconds:    stats.Average,
		StddevSeconds:     stats.Stddev,
		MaxSeconds:        stats.Slowest,
		RequestsPerSecond: stats.RPS,
		Errors:            copyErrorDist(stats.ErrorDist),
	}
	percentiles, data := benchmarkreport.Percentiles(stats.Lats)
	for i, percentile := range percentiles {
		switch percentile {
		case 10:
			report.P10Seconds = data[i]
		case 25:
			report.P25Seconds = data[i]
		case 50:
			report.P50Seconds = data[i]
		case 75:
			report.P75Seconds = data[i]
		case 90:
			report.P90Seconds = data[i]
		case 95:
			report.P95Seconds = data[i]
		case 99:
			report.P99Seconds = data[i]
		case 99.9:
			report.P999Seconds = data[i]
		}
	}
	return report
}

func copyErrorDist(errorDist map[string]int) map[string]int {
	if len(errorDist) == 0 {
		return nil
	}
	copied := make(map[string]int, len(errorDist))
	for key, value := range errorDist {
		copied[key] = value
	}
	return copied
}

func (c *runMetricsCollector) collectMemberMetrics(now time.Time, target string) (MemberMetricsSample, error) {
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, target, nil)
	if err != nil {
		return MemberMetricsSample{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MemberMetricsSample{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return MemberMetricsSample{}, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return MemberMetricsSample{}, err
	}
	parser := expfmt.NewTextParser(model.LegacyValidation)
	metricFamilies, err := parser.TextToMetricFamilies(bytes.NewReader(payload))
	if err != nil {
		return MemberMetricsSample{}, err
	}

	walFsync, err := collectLatencyPercentiles(metricFamilies["etcd_disk_wal_fsync_duration_seconds"])
	if err != nil {
		return MemberMetricsSample{}, fmt.Errorf("etcd_disk_wal_fsync_duration_seconds: %w", err)
	}
	backendCommit, err := collectLatencyPercentiles(metricFamilies["etcd_disk_backend_commit_duration_seconds"])
	if err != nil {
		return MemberMetricsSample{}, fmt.Errorf("etcd_disk_backend_commit_duration_seconds: %w", err)
	}
	proposalsPending, err := metricFamilyValue(metricFamilies["etcd_server_proposals_pending"])
	if err != nil {
		return MemberMetricsSample{}, fmt.Errorf("etcd_server_proposals_pending: %w", err)
	}
	leaderChangesSeen, err := metricFamilyValue(metricFamilies["etcd_server_leader_changes_seen_total"])
	if err != nil {
		return MemberMetricsSample{}, fmt.Errorf("etcd_server_leader_changes_seen_total: %w", err)
	}
	dbTotalSize, err := metricFamilyValue(metricFamilies["etcd_mvcc_db_total_size_in_bytes"])
	if err != nil {
		return MemberMetricsSample{}, fmt.Errorf("etcd_mvcc_db_total_size_in_bytes: %w", err)
	}

	return MemberMetricsSample{
		Timestamp:              now,
		WALFsyncSeconds:        walFsync,
		BackendCommitSeconds:   backendCommit,
		ProposalsPending:       proposalsPending,
		LeaderChangesSeenTotal: leaderChangesSeen,
		DBTotalSizeInBytes:     dbTotalSize,
	}, nil
}

func (c *runMetricsCollector) recordError(ts time.Time, source string, err error) {
	c.report.Errors = append(c.report.Errors, RunMetricsError{
		Timestamp: ts,
		Source:    source,
		Message:   err.Error(),
	})
}

func (c *runMetricsCollector) writeArtifact() (string, error) {
	reportPath := strings.TrimSpace(metricsOutput)
	if reportPath == "" {
		artifactsDir := os.Getenv("ARTIFACTS")
		if artifactsDir == "" {
			artifactsDir = "./_artifacts"
		}
		baseName := sanitizeArtifactName(c.report.Command)
		if c.report.StepName != "" {
			baseName += "_" + sanitizeArtifactName(c.report.StepName)
		}
		fileName := fmt.Sprintf("benchmark_run_metrics_%s_%s.json", baseName, c.report.EndedAt.UTC().Format("20060102T150405Z0700"))
		reportPath = filepath.Join(artifactsDir, fileName)
	}

	reportBytes, err := json.MarshalIndent(c.report, "", "  ")
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return "", err
	}
	if err = os.WriteFile(reportPath, reportBytes, 0o644); err != nil {
		return "", err
	}
	return reportPath, nil
}

func resolvedMetricsURLs() ([]string, error) {
	if len(metricsURLs) != 0 {
		urls := make([]string, 0, len(metricsURLs))
		for _, raw := range metricsURLs {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return nil, errors.New("empty value found in --metrics-urls")
			}
			urls = append(urls, raw)
		}
		return urls, nil
	}

	urls := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		metricsURL, err := deriveMetricsURL(endpoint)
		if err != nil {
			return nil, err
		}
		urls = append(urls, metricsURL)
	}
	return urls, nil
}

func deriveMetricsURL(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", errors.New("empty endpoint")
	}

	if !strings.Contains(endpoint, "://") {
		scheme := "http"
		if benchmarkUsesTLS() {
			scheme = "https"
		}
		return (&url.URL{Scheme: scheme, Host: endpoint, Path: "/metrics"}).String(), nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/metrics"
	} else if !strings.HasSuffix(u.Path, "/metrics") {
		u.Path = path.Join(u.Path, "metrics")
	}
	return u.String(), nil
}

func benchmarkUsesTLS() bool {
	return !tls.Empty() || tls.TrustedCAFile != "" || tls.InsecureSkipVerify
}

func sanitizeArtifactName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "run"
	}
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		" ", "_",
		":", "_",
	)
	return replacer.Replace(value)
}

func summarizeMemberMetrics(samples []MemberMetricsSample) MemberMetricsSummary {
	summary := MemberMetricsSummary{SampleCount: len(samples)}
	if len(samples) == 0 {
		return summary
	}

	first := samples[0]
	last := samples[len(samples)-1]
	summary.LeaderChangesSeenTotalStart = first.LeaderChangesSeenTotal
	summary.LeaderChangesSeenTotalEnd = last.LeaderChangesSeenTotal
	summary.LeaderChangesSeenTotalDelta = last.LeaderChangesSeenTotal - first.LeaderChangesSeenTotal
	summary.DBTotalSizeInBytesStart = first.DBTotalSizeInBytes
	summary.DBTotalSizeInBytesEnd = last.DBTotalSizeInBytes
	summary.DBTotalSizeInBytesDelta = last.DBTotalSizeInBytes - first.DBTotalSizeInBytes

	for _, sample := range samples {
		summary.WALFsyncSecondsMax = maxLatencyPercentiles(summary.WALFsyncSecondsMax, sample.WALFsyncSeconds)
		summary.BackendCommitSecondsMax = maxLatencyPercentiles(summary.BackendCommitSecondsMax, sample.BackendCommitSeconds)
		summary.ProposalsPendingMax = maxFloat64(summary.ProposalsPendingMax, sample.ProposalsPending)
		summary.DBTotalSizeInBytesMax = maxFloat64(summary.DBTotalSizeInBytesMax, sample.DBTotalSizeInBytes)
	}

	return summary
}

func summarizeHostMetrics(samples []HostMetricsSample) HostMetricsSummary {
	summary := HostMetricsSummary{SampleCount: len(samples)}
	if len(samples) == 0 {
		return summary
	}

	first := samples[0]
	last := samples[len(samples)-1]
	summary.DiskReadBytesStart = first.DiskReadBytes
	summary.DiskReadBytesEnd = last.DiskReadBytes
	summary.DiskReadBytesDelta = deltaUint64(last.DiskReadBytes, first.DiskReadBytes)
	summary.DiskWriteBytesStart = first.DiskWriteBytes
	summary.DiskWriteBytesEnd = last.DiskWriteBytes
	summary.DiskWriteBytesDelta = deltaUint64(last.DiskWriteBytes, first.DiskWriteBytes)
	summary.DiskReadCountDelta = deltaUint64(last.DiskReadCount, first.DiskReadCount)
	summary.DiskWriteCountDelta = deltaUint64(last.DiskWriteCount, first.DiskWriteCount)
	summary.NetworkBytesSentStart = first.NetworkBytesSent
	summary.NetworkBytesSentEnd = last.NetworkBytesSent
	summary.NetworkBytesSentDelta = deltaUint64(last.NetworkBytesSent, first.NetworkBytesSent)
	summary.NetworkBytesRecvStart = first.NetworkBytesRecv
	summary.NetworkBytesRecvEnd = last.NetworkBytesRecv
	summary.NetworkBytesRecvDelta = deltaUint64(last.NetworkBytesRecv, first.NetworkBytesRecv)
	summary.NetworkPacketsSentDelta = deltaUint64(last.NetworkPacketsSent, first.NetworkPacketsSent)
	summary.NetworkPacketsRecvDelta = deltaUint64(last.NetworkPacketsRecv, first.NetworkPacketsRecv)
	summary.NetworkErrinDelta = deltaUint64(last.NetworkErrin, first.NetworkErrin)
	summary.NetworkErroutDelta = deltaUint64(last.NetworkErrout, first.NetworkErrout)
	summary.NetworkDropinDelta = deltaUint64(last.NetworkDropin, first.NetworkDropin)
	summary.NetworkDropoutDelta = deltaUint64(last.NetworkDropout, first.NetworkDropout)
	summary.NetworkDropTotalDelta = summary.NetworkDropinDelta + summary.NetworkDropoutDelta

	durationSeconds := last.Timestamp.Sub(first.Timestamp).Seconds()
	if durationSeconds > 0 {
		summary.NetworkBytesSentPerSec = float64(summary.NetworkBytesSentDelta) / durationSeconds
		summary.NetworkBytesRecvPerSec = float64(summary.NetworkBytesRecvDelta) / durationSeconds
		summary.NetworkPacketsSentPerSec = float64(summary.NetworkPacketsSentDelta) / durationSeconds
		summary.NetworkPacketsRecvPerSec = float64(summary.NetworkPacketsRecvDelta) / durationSeconds
		summary.NetworkDropinPerSec = float64(summary.NetworkDropinDelta) / durationSeconds
		summary.NetworkDropoutPerSec = float64(summary.NetworkDropoutDelta) / durationSeconds
		summary.NetworkDropTotalPerSec = float64(summary.NetworkDropTotalDelta) / durationSeconds
	}

	for _, sample := range samples {
		summary.CPUPercentMax = maxFloat64(summary.CPUPercentMax, sample.CPUPercent)
		summary.MemoryUsedBytesMax = maxUint64(summary.MemoryUsedBytesMax, sample.MemoryUsedBytes)
		summary.MemoryUsedPercentMax = maxFloat64(summary.MemoryUsedPercentMax, sample.MemoryUsedPercent)
	}
	for i := 1; i < len(samples); i++ {
		totalDelta := deltaUint64(samples[i].CPUTotalJiffies, samples[i-1].CPUTotalJiffies)
		idleDelta := deltaUint64(samples[i].CPUIdleJiffies, samples[i-1].CPUIdleJiffies)
		if totalDelta == 0 || idleDelta > totalDelta {
			continue
		}
		cpuPercent := (1 - float64(idleDelta)/float64(totalDelta)) * 100
		summary.CPUPercentMax = maxFloat64(summary.CPUPercentMax, cpuPercent)
	}

	return summary
}

func collectHostMetrics(now time.Time) (HostMetricsSample, error) {
	sample := HostMetricsSample{Timestamp: now}
	var sampleErr error

	cpuPercent, err := cpu.Percent(0, false)
	if err != nil {
		sampleErr = errors.Join(sampleErr, fmt.Errorf("cpu: %w", err))
	} else if len(cpuPercent) > 0 {
		sample.CPUPercent = cpuPercent[0]
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		sampleErr = errors.Join(sampleErr, fmt.Errorf("memory: %w", err))
	} else {
		sample.MemoryUsedBytes = vm.Used
		sample.MemoryUsedPercent = vm.UsedPercent
		sample.MemoryAvailableBytes = vm.Available
	}

	ioCounters, err := disk.IOCounters()
	if err != nil {
		sampleErr = errors.Join(sampleErr, fmt.Errorf("disk io: %w", err))
	} else {
		for _, stat := range ioCounters {
			sample.DiskReadBytes += stat.ReadBytes
			sample.DiskWriteBytes += stat.WriteBytes
			sample.DiskReadCount += stat.ReadCount
			sample.DiskWriteCount += stat.WriteCount
		}
	}

	netCounters, err := netio.IOCounters(false)
	if err != nil {
		sampleErr = errors.Join(sampleErr, fmt.Errorf("network io: %w", err))
	} else {
		for _, stat := range netCounters {
			sample.NetworkBytesSent += stat.BytesSent
			sample.NetworkBytesRecv += stat.BytesRecv
			sample.NetworkPacketsSent += stat.PacketsSent
			sample.NetworkPacketsRecv += stat.PacketsRecv
			sample.NetworkErrin += stat.Errin
			sample.NetworkErrout += stat.Errout
			sample.NetworkDropin += stat.Dropin
			sample.NetworkDropout += stat.Dropout
		}
	}

	return sample, sampleErr
}

const remoteHostMetricsCommand = `cat /proc/stat; printf '\n__KILT_MEMINFO__\n'; cat /proc/meminfo; printf '\n__KILT_DISKSTATS__\n'; cat /proc/diskstats; printf '\n__KILT_NETDEV__\n'; cat /proc/net/dev`

func normalizedRemoteHosts(rawHosts []string) []string {
	hosts := make([]string, 0, len(rawHosts))
	seen := make(map[string]struct{})
	for _, raw := range rawHosts {
		for _, host := range strings.Split(raw, ",") {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			if _, ok := seen[host]; ok {
				continue
			}
			seen[host] = struct{}{}
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func remoteHostDisplayName(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "remote"
	}
	if before, _, found := strings.Cut(target, "@"); found && strings.TrimSpace(before) != "" {
		target = strings.TrimPrefix(target, before+"@")
	}
	if host, _, err := net.SplitHostPort(target); err == nil {
		target = host
	}
	return strings.Trim(target, "[]")
}

func collectRemoteHostMetrics(parent context.Context, now time.Time, target string, timeout time.Duration) (HostMetricsSample, error) {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=3",
		target,
		remoteHostMetricsCommand,
	)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return HostMetricsSample{}, fmt.Errorf("ssh metrics sample timed out after %s", timeout)
	}
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if len(detail) > 512 {
			detail = detail[:512] + "..."
		}
		if detail == "" {
			return HostMetricsSample{}, fmt.Errorf("ssh metrics sample: %w", err)
		}
		return HostMetricsSample{}, fmt.Errorf("ssh metrics sample: %w: %s", err, detail)
	}
	return parseProcHostMetrics(now, string(output))
}

func parseProcHostMetrics(now time.Time, payload string) (HostMetricsSample, error) {
	sample := HostMetricsSample{Timestamp: now}
	section := "stat"
	var memoryTotalBytes uint64
	scanner := bufio.NewScanner(strings.NewReader(payload))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "__KILT_MEMINFO__":
			section = "meminfo"
			continue
		case "__KILT_DISKSTATS__":
			section = "diskstats"
			continue
		case "__KILT_NETDEV__":
			section = "netdev"
			continue
		}
		if line == "" {
			continue
		}

		switch section {
		case "stat":
			parseProcStatLine(&sample, line)
		case "meminfo":
			parseProcMeminfoLine(&sample, &memoryTotalBytes, line)
		case "diskstats":
			parseProcDiskstatsLine(&sample, line)
		case "netdev":
			parseProcNetdevLine(&sample, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return sample, err
	}
	if memoryTotalBytes > 0 {
		sample.MemoryUsedBytes = deltaUint64(memoryTotalBytes, sample.MemoryAvailableBytes)
		sample.MemoryUsedPercent = float64(sample.MemoryUsedBytes) / float64(memoryTotalBytes) * 100
	}
	return sample, nil
}

func parseProcStatLine(sample *HostMetricsSample, line string) {
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return
	}
	var total uint64
	for _, field := range fields[1:] {
		total += parseUint64(field)
	}
	idle := parseUint64(fields[4])
	if len(fields) > 5 {
		idle += parseUint64(fields[5])
	}
	sample.CPUTotalJiffies = total
	sample.CPUIdleJiffies = idle
}

func parseProcMeminfoLine(sample *HostMetricsSample, memoryTotalBytes *uint64, line string) {
	name, value, found := strings.Cut(line, ":")
	if !found {
		return
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return
	}
	bytes := parseUint64(fields[0]) * 1024
	switch name {
	case "MemTotal":
		*memoryTotalBytes = bytes
	case "MemAvailable":
		sample.MemoryAvailableBytes = bytes
	}
}

func parseProcDiskstatsLine(sample *HostMetricsSample, line string) {
	fields := strings.Fields(line)
	if len(fields) < 10 || !isWholeDiskDevice(fields[2]) {
		return
	}
	sample.DiskReadCount += parseUint64(fields[3])
	sample.DiskReadBytes += parseUint64(fields[5]) * 512
	sample.DiskWriteCount += parseUint64(fields[7])
	sample.DiskWriteBytes += parseUint64(fields[9]) * 512
}

func parseProcNetdevLine(sample *HostMetricsSample, line string) {
	if !strings.Contains(line, ":") || strings.HasPrefix(line, "Inter-|") || strings.HasPrefix(line, "face |") {
		return
	}
	iface, values, _ := strings.Cut(line, ":")
	if strings.TrimSpace(iface) == "lo" {
		return
	}
	fields := strings.Fields(values)
	if len(fields) < 12 {
		return
	}
	sample.NetworkBytesRecv += parseUint64(fields[0])
	sample.NetworkPacketsRecv += parseUint64(fields[1])
	sample.NetworkErrin += parseUint64(fields[2])
	sample.NetworkDropin += parseUint64(fields[3])
	sample.NetworkBytesSent += parseUint64(fields[8])
	sample.NetworkPacketsSent += parseUint64(fields[9])
	sample.NetworkErrout += parseUint64(fields[10])
	sample.NetworkDropout += parseUint64(fields[11])
}

func isWholeDiskDevice(name string) bool {
	for _, prefix := range []string{"loop", "ram", "fd", "sr", "dm-"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	if len(name) == 0 {
		return false
	}
	last := name[len(name)-1]
	if last < '0' || last > '9' {
		return true
	}
	// nvme0n1 and mmcblk0 are whole devices; their partitions are nvme0n1p1 and mmcblk0p1.
	return (strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "mmcblk")) && !strings.Contains(name, "p")
}

func parseUint64(value string) uint64 {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func metricFamilyValue(mf *dto.MetricFamily) (float64, error) {
	if mf == nil {
		return 0, errors.New("metric not found")
	}
	var total float64
	switch mf.GetType() {
	case dto.MetricType_COUNTER:
		for _, metric := range mf.Metric {
			total += metric.GetCounter().GetValue()
		}
	case dto.MetricType_GAUGE:
		for _, metric := range mf.Metric {
			total += metric.GetGauge().GetValue()
		}
	case dto.MetricType_UNTYPED:
		for _, metric := range mf.Metric {
			total += metric.GetUntyped().GetValue()
		}
	default:
		return 0, fmt.Errorf("unsupported metric type %s", mf.GetType())
	}
	return total, nil
}

func metricFamilyHistogramQuantile(mf *dto.MetricFamily, quantile float64) (float64, error) {
	if mf == nil {
		return 0, errors.New("metric not found")
	}
	if quantile < 0 || quantile > 1 {
		return 0, fmt.Errorf("quantile out of range: %v", quantile)
	}
	if mf.GetType() != dto.MetricType_HISTOGRAM {
		return 0, fmt.Errorf("unsupported metric type %s", mf.GetType())
	}

	cumulativeBuckets := make(map[float64]float64)
	var sampleCount float64
	for _, metric := range mf.Metric {
		histogram := metric.GetHistogram()
		if histogram == nil {
			continue
		}
		sampleCount += float64(histogram.GetSampleCount())
		for _, bucket := range histogram.Bucket {
			cumulativeBuckets[bucket.GetUpperBound()] += float64(bucket.GetCumulativeCount())
		}
	}
	if sampleCount == 0 {
		return 0, nil
	}

	bounds := make([]float64, 0, len(cumulativeBuckets))
	for upperBound := range cumulativeBuckets {
		bounds = append(bounds, upperBound)
	}
	sort.Float64s(bounds)

	rank := quantile * sampleCount
	var (
		prevCount float64
		prevBound float64
	)
	for _, upperBound := range bounds {
		currentCount := cumulativeBuckets[upperBound]
		if currentCount >= rank {
			if math.IsInf(upperBound, 1) {
				return prevBound, nil
			}
			if currentCount == prevCount {
				return upperBound, nil
			}
			fraction := (rank - prevCount) / (currentCount - prevCount)
			return prevBound + (upperBound-prevBound)*fraction, nil
		}
		prevCount = currentCount
		prevBound = upperBound
	}

	return bounds[len(bounds)-1], nil
}

func collectLatencyPercentiles(mf *dto.MetricFamily) (LatencyPercentiles, error) {
	p50, err := metricFamilyHistogramQuantile(mf, 0.50)
	if err != nil {
		return LatencyPercentiles{}, err
	}
	p90, err := metricFamilyHistogramQuantile(mf, 0.90)
	if err != nil {
		return LatencyPercentiles{}, err
	}
	p99, err := metricFamilyHistogramQuantile(mf, 0.99)
	if err != nil {
		return LatencyPercentiles{}, err
	}
	p999, err := metricFamilyHistogramQuantile(mf, 0.999)
	if err != nil {
		return LatencyPercentiles{}, err
	}
	return LatencyPercentiles{
		P50Seconds:  p50,
		P90Seconds:  p90,
		P99Seconds:  p99,
		P999Seconds: p999,
	}, nil
}

func maxLatencyPercentiles(a, b LatencyPercentiles) LatencyPercentiles {
	return LatencyPercentiles{
		P50Seconds:  maxFloat64(a.P50Seconds, b.P50Seconds),
		P90Seconds:  maxFloat64(a.P90Seconds, b.P90Seconds),
		P99Seconds:  maxFloat64(a.P99Seconds, b.P99Seconds),
		P999Seconds: maxFloat64(a.P999Seconds, b.P999Seconds),
	}
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxUint64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func deltaUint64(last, first uint64) uint64 {
	if last < first {
		return 0
	}
	return last - first
}

func formatRunMetricsSummary(report RunMetricsReport) string {
	var sb strings.Builder
	sb.WriteString("\nRun metrics summary:\n")
	fmt.Fprintf(&sb, "  Command:\t%s\n", report.Command)
	if report.StepName != "" {
		fmt.Fprintf(&sb, "  Step:\t%s\n", report.StepName)
	}
	fmt.Fprintf(&sb, "  Window:\t%s -> %s\n", report.StartedAt.Format(time.RFC3339), report.EndedAt.Format(time.RFC3339))
	fmt.Fprintf(&sb, "  Sample interval:\t%s\n", report.SampleInterval)

	for _, member := range report.Members {
		summary := member.Summary
		fmt.Fprintf(&sb, "  Member %s:\n", member.MetricsURL)
		fmt.Fprintf(&sb, "    wal fsync max:\t%s\n", formatLatencyPercentiles(summary.WALFsyncSecondsMax))
		fmt.Fprintf(&sb, "    backend commit max:\t%s\n", formatLatencyPercentiles(summary.BackendCommitSecondsMax))
		fmt.Fprintf(&sb, "    proposals pending max:\t%.0f\n", summary.ProposalsPendingMax)
		fmt.Fprintf(&sb, "    leader changes delta:\t%.0f\n", summary.LeaderChangesSeenTotalDelta)
		fmt.Fprintf(&sb, "    db size delta:\t%.0f bytes\n", summary.DBTotalSizeInBytesDelta)
	}

	hostSummary := report.Host.Summary
	if hostSummary.SampleCount > 0 {
		appendHostMetricsSummary(&sb, "Host "+hostReportLabel(report.Host), hostSummary)
	}
	for _, host := range report.RemoteHosts {
		if host.Summary.SampleCount == 0 {
			continue
		}
		appendHostMetricsSummary(&sb, "Remote host "+hostReportLabel(host), host.Summary)
	}

	if len(report.Errors) > 0 {
		fmt.Fprintf(&sb, "  Collection warnings:\t%d\n", len(report.Errors))
	}

	return sb.String()
}

func appendHostMetricsSummary(sb *strings.Builder, label string, summary HostMetricsSummary) {
	fmt.Fprintf(sb, "  %s:\n", label)
	fmt.Fprintf(sb, "    cpu max:\t%.2f%%\n", summary.CPUPercentMax)
	fmt.Fprintf(sb, "    memory used max:\t%d bytes\n", summary.MemoryUsedBytesMax)
	fmt.Fprintf(sb, "    memory used max pct:\t%.2f%%\n", summary.MemoryUsedPercentMax)
	fmt.Fprintf(sb, "    disk read delta:\t%d bytes\n", summary.DiskReadBytesDelta)
	fmt.Fprintf(sb, "    disk write delta:\t%d bytes\n", summary.DiskWriteBytesDelta)
	fmt.Fprintf(sb, "    network tx delta:\t%d bytes (%.2f bytes/sec)\n", summary.NetworkBytesSentDelta, summary.NetworkBytesSentPerSec)
	fmt.Fprintf(sb, "    network rx delta:\t%d bytes (%.2f bytes/sec)\n", summary.NetworkBytesRecvDelta, summary.NetworkBytesRecvPerSec)
	fmt.Fprintf(sb, "    network tx packets delta:\t%d (%.2f packets/sec)\n", summary.NetworkPacketsSentDelta, summary.NetworkPacketsSentPerSec)
	fmt.Fprintf(sb, "    network rx packets delta:\t%d (%.2f packets/sec)\n", summary.NetworkPacketsRecvDelta, summary.NetworkPacketsRecvPerSec)
	fmt.Fprintf(sb, "    network errors delta:\tin=%d out=%d dropin=%d dropout=%d\n",
		summary.NetworkErrinDelta,
		summary.NetworkErroutDelta,
		summary.NetworkDropinDelta,
		summary.NetworkDropoutDelta,
	)
	fmt.Fprintf(sb, "    network drops delta:\trx=%d tx=%d total=%d (%.2f packets/sec)\n",
		summary.NetworkDropinDelta,
		summary.NetworkDropoutDelta,
		summary.NetworkDropTotalDelta,
		summary.NetworkDropTotalPerSec,
	)
}

func hostReportLabel(host HostMetricsReport) string {
	if host.Name != "" {
		return host.Name
	}
	if host.Target != "" {
		return host.Target
	}
	return "local"
}

func formatLatencyPercentiles(p LatencyPercentiles) string {
	return fmt.Sprintf("P50=%.6f s P90=%.6f s P99=%.6f s P999=%.6f s", p.P50Seconds, p.P90Seconds, p.P99Seconds, p.P999Seconds)
}
