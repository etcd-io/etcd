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
	"math"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"

	"go.etcd.io/etcd/client/pkg/v3/transport"
)

func TestDeriveMetricsURL(t *testing.T) {
	originalTLS := tls
	defer func() {
		tls = originalTLS
	}()

	tests := []struct {
		name     string
		tlsInfo  transport.TLSInfo
		endpoint string
		want     string
	}{
		{
			name:     "plain host port",
			endpoint: "127.0.0.1:2379",
			want:     "http://127.0.0.1:2379/metrics",
		},
		{
			name:     "tls endpoint defaults to https",
			tlsInfo:  transport.TLSInfo{TrustedCAFile: "ca.crt"},
			endpoint: "127.0.0.1:2379",
			want:     "https://127.0.0.1:2379/metrics",
		},
		{
			name:     "existing http url",
			endpoint: "http://127.0.0.1:2381",
			want:     "http://127.0.0.1:2381/metrics",
		},
		{
			name:     "existing nested path",
			endpoint: "https://127.0.0.1:2381/debug",
			want:     "https://127.0.0.1:2381/debug/metrics",
		},
		{
			name:     "unix url",
			endpoint: "unix://localhost:27989",
			want:     "unix://localhost:27989/metrics",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tls = tc.tlsInfo
			got, err := deriveMetricsURL(tc.endpoint)
			if err != nil {
				t.Fatalf("deriveMetricsURL(%q) returned error: %v", tc.endpoint, err)
			}
			if got != tc.want {
				t.Fatalf("deriveMetricsURL(%q) = %q, want %q", tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestDeriveMetricsURLEmptyEndpoint(t *testing.T) {
	_, err := deriveMetricsURL(" ")
	if err == nil {
		t.Fatal("deriveMetricsURL returned nil error for empty endpoint")
	}
}

func TestMetricFamilyHistogramQuantile(t *testing.T) {
	mf := &dto.MetricFamily{
		Type: dto.MetricType_HISTOGRAM.Enum(),
		Metric: []*dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: uint64p(100),
					Bucket: []*dto.Bucket{
						{UpperBound: float64p(1), CumulativeCount: uint64p(50)},
						{UpperBound: float64p(2), CumulativeCount: uint64p(99)},
						{UpperBound: float64p(4), CumulativeCount: uint64p(100)},
					},
				},
			},
		},
	}

	testCases := []struct {
		name     string
		quantile float64
		want     float64
	}{
		{name: "p50", quantile: 0.50, want: 1},
		{name: "p99", quantile: 0.99, want: 2},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := metricFamilyHistogramQuantile(mf, tc.quantile)
			if err != nil {
				t.Fatalf("metricFamilyHistogramQuantile returned error: %v", err)
			}
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("metricFamilyHistogramQuantile(%v) = %v, want %v", tc.quantile, got, tc.want)
			}
		})
	}
}

func TestCollectLatencyPercentiles(t *testing.T) {
	mf := &dto.MetricFamily{
		Type: dto.MetricType_HISTOGRAM.Enum(),
		Metric: []*dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: uint64p(100),
					Bucket: []*dto.Bucket{
						{UpperBound: float64p(1), CumulativeCount: uint64p(50)},
						{UpperBound: float64p(2), CumulativeCount: uint64p(99)},
						{UpperBound: float64p(4), CumulativeCount: uint64p(100)},
					},
				},
			},
		},
	}

	got, err := collectLatencyPercentiles(mf)
	if err != nil {
		t.Fatalf("collectLatencyPercentiles returned error: %v", err)
	}

	want := LatencyPercentiles{
		P50Seconds:  1,
		P90Seconds:  1.816326530612245,
		P99Seconds:  2,
		P999Seconds: 3.8,
	}
	assertLatencyPercentiles(t, got, want)
}

func TestMetricFamilyHistogramQuantileErrors(t *testing.T) {
	gauge := &dto.MetricFamily{Type: dto.MetricType_GAUGE.Enum()}
	_, err := metricFamilyHistogramQuantile(gauge, 0.99)
	if err == nil || !strings.Contains(err.Error(), "unsupported metric type") {
		t.Fatalf("expected unsupported type error, got %v", err)
	}

	_, err = metricFamilyHistogramQuantile(nil, 0.99)
	if err == nil || !strings.Contains(err.Error(), "metric not found") {
		t.Fatalf("expected metric not found error, got %v", err)
	}

	_, err = metricFamilyHistogramQuantile(&dto.MetricFamily{Type: dto.MetricType_HISTOGRAM.Enum()}, 1.1)
	if err == nil || !strings.Contains(err.Error(), "quantile out of range") {
		t.Fatalf("expected quantile range error, got %v", err)
	}
}

func TestSummarizeMemberMetrics(t *testing.T) {
	samples := []MemberMetricsSample{
		{
			Timestamp: time.Unix(1, 0).UTC(),
			WALFsyncSeconds: LatencyPercentiles{
				P50Seconds:  0.001,
				P90Seconds:  0.002,
				P99Seconds:  0.003,
				P999Seconds: 0.004,
			},
			BackendCommitSeconds: LatencyPercentiles{
				P50Seconds:  0.002,
				P90Seconds:  0.003,
				P99Seconds:  0.004,
				P999Seconds: 0.005,
			},
			ProposalsPending:       1,
			LeaderChangesSeenTotal: 0,
			DBTotalSizeInBytes:     1024,
		},
		{
			Timestamp: time.Unix(2, 0).UTC(),
			WALFsyncSeconds: LatencyPercentiles{
				P50Seconds:  0.005,
				P90Seconds:  0.006,
				P99Seconds:  0.007,
				P999Seconds: 0.008,
			},
			BackendCommitSeconds: LatencyPercentiles{
				P50Seconds:  0.003,
				P90Seconds:  0.004,
				P99Seconds:  0.005,
				P999Seconds: 0.006,
			},
			ProposalsPending:       3,
			LeaderChangesSeenTotal: 0,
			DBTotalSizeInBytes:     2048,
		},
	}

	summary := summarizeMemberMetrics(samples)
	if summary.SampleCount != 2 {
		t.Fatalf("SampleCount = %d, want 2", summary.SampleCount)
	}
	assertLatencyPercentiles(t, summary.WALFsyncSecondsMax, LatencyPercentiles{P50Seconds: 0.005, P90Seconds: 0.006, P99Seconds: 0.007, P999Seconds: 0.008})
	assertLatencyPercentiles(t, summary.BackendCommitSecondsMax, LatencyPercentiles{P50Seconds: 0.003, P90Seconds: 0.004, P99Seconds: 0.005, P999Seconds: 0.006})
	if summary.ProposalsPendingMax != 3 {
		t.Fatalf("ProposalsPendingMax = %v, want 3", summary.ProposalsPendingMax)
	}
	if summary.LeaderChangesSeenTotalDelta != 0 {
		t.Fatalf("LeaderChangesSeenTotalDelta = %v, want 0", summary.LeaderChangesSeenTotalDelta)
	}
	if summary.DBTotalSizeInBytesDelta != 1024 {
		t.Fatalf("DBTotalSizeInBytesDelta = %v, want 1024", summary.DBTotalSizeInBytesDelta)
	}
	if summary.DBTotalSizeInBytesMax != 2048 {
		t.Fatalf("DBTotalSizeInBytesMax = %v, want 2048", summary.DBTotalSizeInBytesMax)
	}
}

func TestSummarizeHostMetrics(t *testing.T) {
	summary := summarizeHostMetrics([]HostMetricsSample{
		{
			Timestamp:          time.Unix(1, 0).UTC(),
			CPUPercent:         20,
			MemoryUsedBytes:    1024,
			MemoryUsedPercent:  30,
			DiskReadBytes:      100,
			DiskWriteBytes:     200,
			DiskReadCount:      10,
			DiskWriteCount:     20,
			NetworkBytesSent:   1_000,
			NetworkBytesRecv:   2_000,
			NetworkPacketsSent: 10,
			NetworkPacketsRecv: 20,
			NetworkErrin:       1,
			NetworkErrout:      2,
			NetworkDropin:      3,
			NetworkDropout:     4,
		},
		{
			Timestamp:          time.Unix(2, 0).UTC(),
			CPUPercent:         35,
			MemoryUsedBytes:    4096,
			MemoryUsedPercent:  45,
			DiskReadBytes:      175,
			DiskWriteBytes:     350,
			DiskReadCount:      13,
			DiskWriteCount:     25,
			NetworkBytesSent:   1_750,
			NetworkBytesRecv:   3_250,
			NetworkPacketsSent: 18,
			NetworkPacketsRecv: 33,
			NetworkErrin:       2,
			NetworkErrout:      4,
			NetworkDropin:      6,
			NetworkDropout:     8,
		},
	})

	if summary.SampleCount != 2 {
		t.Fatalf("SampleCount = %d, want 2", summary.SampleCount)
	}
	if summary.CPUPercentMax != 35 {
		t.Fatalf("CPUPercentMax = %v, want 35", summary.CPUPercentMax)
	}
	if summary.MemoryUsedBytesMax != 4096 {
		t.Fatalf("MemoryUsedBytesMax = %d, want 4096", summary.MemoryUsedBytesMax)
	}
	if summary.MemoryUsedPercentMax != 45 {
		t.Fatalf("MemoryUsedPercentMax = %v, want 45", summary.MemoryUsedPercentMax)
	}
	if summary.DiskReadBytesDelta != 75 {
		t.Fatalf("DiskReadBytesDelta = %d, want 75", summary.DiskReadBytesDelta)
	}
	if summary.DiskWriteBytesDelta != 150 {
		t.Fatalf("DiskWriteBytesDelta = %d, want 150", summary.DiskWriteBytesDelta)
	}
	if summary.DiskReadCountDelta != 3 {
		t.Fatalf("DiskReadCountDelta = %d, want 3", summary.DiskReadCountDelta)
	}
	if summary.DiskWriteCountDelta != 5 {
		t.Fatalf("DiskWriteCountDelta = %d, want 5", summary.DiskWriteCountDelta)
	}
	if summary.NetworkBytesSentDelta != 750 {
		t.Fatalf("NetworkBytesSentDelta = %d, want 750", summary.NetworkBytesSentDelta)
	}
	if summary.NetworkBytesRecvDelta != 1250 {
		t.Fatalf("NetworkBytesRecvDelta = %d, want 1250", summary.NetworkBytesRecvDelta)
	}
	if summary.NetworkPacketsSentDelta != 8 {
		t.Fatalf("NetworkPacketsSentDelta = %d, want 8", summary.NetworkPacketsSentDelta)
	}
	if summary.NetworkPacketsRecvDelta != 13 {
		t.Fatalf("NetworkPacketsRecvDelta = %d, want 13", summary.NetworkPacketsRecvDelta)
	}
	if summary.NetworkErrinDelta != 1 {
		t.Fatalf("NetworkErrinDelta = %d, want 1", summary.NetworkErrinDelta)
	}
	if summary.NetworkErroutDelta != 2 {
		t.Fatalf("NetworkErroutDelta = %d, want 2", summary.NetworkErroutDelta)
	}
	if summary.NetworkDropinDelta != 3 {
		t.Fatalf("NetworkDropinDelta = %d, want 3", summary.NetworkDropinDelta)
	}
	if summary.NetworkDropoutDelta != 4 {
		t.Fatalf("NetworkDropoutDelta = %d, want 4", summary.NetworkDropoutDelta)
	}
	if summary.NetworkDropTotalDelta != 7 {
		t.Fatalf("NetworkDropTotalDelta = %d, want 7", summary.NetworkDropTotalDelta)
	}
	if summary.NetworkBytesSentPerSec != 750 {
		t.Fatalf("NetworkBytesSentPerSec = %v, want 750", summary.NetworkBytesSentPerSec)
	}
	if summary.NetworkBytesRecvPerSec != 1250 {
		t.Fatalf("NetworkBytesRecvPerSec = %v, want 1250", summary.NetworkBytesRecvPerSec)
	}
	if summary.NetworkPacketsSentPerSec != 8 {
		t.Fatalf("NetworkPacketsSentPerSec = %v, want 8", summary.NetworkPacketsSentPerSec)
	}
	if summary.NetworkPacketsRecvPerSec != 13 {
		t.Fatalf("NetworkPacketsRecvPerSec = %v, want 13", summary.NetworkPacketsRecvPerSec)
	}
	if summary.NetworkDropTotalPerSec != 7 {
		t.Fatalf("NetworkDropTotalPerSec = %v, want 7", summary.NetworkDropTotalPerSec)
	}
}

func TestSummarizeHostMetricsCounterResetClampsDelta(t *testing.T) {
	summary := summarizeHostMetrics([]HostMetricsSample{
		{DiskReadBytes: 200, DiskWriteBytes: 300, DiskReadCount: 10, DiskWriteCount: 11, NetworkBytesSent: 20, NetworkBytesRecv: 30},
		{DiskReadBytes: 100, DiskWriteBytes: 250, DiskReadCount: 8, DiskWriteCount: 10, NetworkBytesSent: 10, NetworkBytesRecv: 25},
	})

	if summary.DiskReadBytesDelta != 0 {
		t.Fatalf("DiskReadBytesDelta = %d, want 0", summary.DiskReadBytesDelta)
	}
	if summary.DiskWriteBytesDelta != 0 {
		t.Fatalf("DiskWriteBytesDelta = %d, want 0", summary.DiskWriteBytesDelta)
	}
	if summary.DiskReadCountDelta != 0 {
		t.Fatalf("DiskReadCountDelta = %d, want 0", summary.DiskReadCountDelta)
	}
	if summary.DiskWriteCountDelta != 0 {
		t.Fatalf("DiskWriteCountDelta = %d, want 0", summary.DiskWriteCountDelta)
	}
	if summary.NetworkBytesSentDelta != 0 {
		t.Fatalf("NetworkBytesSentDelta = %d, want 0", summary.NetworkBytesSentDelta)
	}
	if summary.NetworkBytesRecvDelta != 0 {
		t.Fatalf("NetworkBytesRecvDelta = %d, want 0", summary.NetworkBytesRecvDelta)
	}
}

func TestSummarizeHostMetricsComputesCPUFromJiffies(t *testing.T) {
	summary := summarizeHostMetrics([]HostMetricsSample{
		{
			Timestamp:       time.Unix(1, 0).UTC(),
			CPUTotalJiffies: 100,
			CPUIdleJiffies:  50,
		},
		{
			Timestamp:       time.Unix(2, 0).UTC(),
			CPUTotalJiffies: 200,
			CPUIdleJiffies:  75,
		},
	})

	if math.Abs(summary.CPUPercentMax-75) > 1e-9 {
		t.Fatalf("CPUPercentMax = %v, want 75", summary.CPUPercentMax)
	}
}

func TestParseProcHostMetrics(t *testing.T) {
	payload := `cpu  100 0 50 850 0 0 0 0 0 0
__KILT_MEMINFO__
MemTotal:        1000 kB
MemAvailable:     250 kB
__KILT_DISKSTATS__
   8       0 sda 10 0 20 0 30 0 40 0 0 0 0 0 0 0 0 0 0
   8       1 sda1 99 0 99 0 99 0 99 0 0 0 0 0 0 0 0 0 0
__KILT_NETDEV__
Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1000 10 1 2 0 0 0 0 2000 20 3 4 0 0 0 0
    lo: 9999 99 9 9 0 0 0 0 9999 99 9 9 0 0 0 0
`

	sample, err := parseProcHostMetrics(time.Unix(1, 0).UTC(), payload)
	if err != nil {
		t.Fatalf("parseProcHostMetrics returned error: %v", err)
	}
	if sample.CPUTotalJiffies != 1000 {
		t.Fatalf("CPUTotalJiffies = %d, want 1000", sample.CPUTotalJiffies)
	}
	if sample.CPUIdleJiffies != 850 {
		t.Fatalf("CPUIdleJiffies = %d, want 850", sample.CPUIdleJiffies)
	}
	if sample.MemoryUsedBytes != 750*1024 {
		t.Fatalf("MemoryUsedBytes = %d, want %d", sample.MemoryUsedBytes, uint64(750*1024))
	}
	if sample.DiskReadBytes != 20*512 {
		t.Fatalf("DiskReadBytes = %d, want %d", sample.DiskReadBytes, uint64(20*512))
	}
	if sample.DiskWriteBytes != 40*512 {
		t.Fatalf("DiskWriteBytes = %d, want %d", sample.DiskWriteBytes, uint64(40*512))
	}
	if sample.NetworkBytesRecv != 1000 || sample.NetworkBytesSent != 2000 {
		t.Fatalf("network bytes = recv %d sent %d, want recv 1000 sent 2000", sample.NetworkBytesRecv, sample.NetworkBytesSent)
	}
	if sample.NetworkPacketsRecv != 10 || sample.NetworkPacketsSent != 20 {
		t.Fatalf("network packets = recv %d sent %d, want recv 10 sent 20", sample.NetworkPacketsRecv, sample.NetworkPacketsSent)
	}
	if sample.NetworkDropin != 2 || sample.NetworkDropout != 4 {
		t.Fatalf("network drops = in %d out %d, want in 2 out 4", sample.NetworkDropin, sample.NetworkDropout)
	}
}

func TestNormalizedRemoteHostsSplitsDeduplicatesAndTrims(t *testing.T) {
	got := normalizedRemoteHosts([]string{" opc@node1,opc@node2 ", "opc@node1", ""})
	want := []string{"opc@node1", "opc@node2"}
	if len(got) != len(want) {
		t.Fatalf("normalizedRemoteHosts length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizedRemoteHosts[%d] = %q, want %q; got %v", i, got[i], want[i], got)
		}
	}
}

func float64p(v float64) *float64 {
	return &v
}

func uint64p(v uint64) *uint64 {
	return &v
}

func assertLatencyPercentiles(t *testing.T, got, want LatencyPercentiles) {
	t.Helper()
	if math.Abs(got.P50Seconds-want.P50Seconds) > 1e-12 {
		t.Fatalf("P50Seconds = %v, want %v", got.P50Seconds, want.P50Seconds)
	}
	if math.Abs(got.P90Seconds-want.P90Seconds) > 1e-12 {
		t.Fatalf("P90Seconds = %v, want %v", got.P90Seconds, want.P90Seconds)
	}
	if math.Abs(got.P99Seconds-want.P99Seconds) > 1e-12 {
		t.Fatalf("P99Seconds = %v, want %v", got.P99Seconds, want.P99Seconds)
	}
	if math.Abs(got.P999Seconds-want.P999Seconds) > 1e-12 {
		t.Fatalf("P999Seconds = %v, want %v", got.P999Seconds, want.P999Seconds)
	}
}
