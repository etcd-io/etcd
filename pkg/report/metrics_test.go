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

package report

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestExtractMetricValues(t *testing.T) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(`
# HELP process_resident_memory_bytes Resident memory size in bytes.
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 123
# HELP go_memstats_heap_alloc_bytes Number of heap bytes allocated and still in use.
# TYPE go_memstats_heap_alloc_bytes gauge
go_memstats_heap_alloc_bytes 456
# HELP go_memstats_heap_inuse_bytes Number of heap bytes that are in use.
# TYPE go_memstats_heap_inuse_bytes gauge
go_memstats_heap_inuse_bytes 789
# HELP ignored_metric Ignored.
# TYPE ignored_metric gauge
ignored_metric 999
`))
	require.NoError(t, err)

	values := extractMetricValues(families, []string{
		"process_resident_memory_bytes",
		"go_memstats_heap_alloc_bytes",
		"go_memstats_heap_inuse_bytes",
		"missing_metric",
	})

	require.Equal(t, map[string]float64{
		"process_resident_memory_bytes": 123,
		"go_memstats_heap_alloc_bytes":  456,
		"go_memstats_heap_inuse_bytes":  789,
	}, values)
}

func TestMetricSamplerSummaries(t *testing.T) {
	sampler := newMetricSampler("http://127.0.0.1:2379/metrics")
	sampler.samples["process_resident_memory_bytes"] = []float64{10, 20, 15}
	sampler.samples["go_memstats_heap_alloc_bytes"] = []float64{30, 40, 35}
	sampler.samples["go_memstats_heap_inuse_bytes"] = []float64{50, 45, 60}

	summaries := sampler.summaries()
	require.Len(t, summaries, 3)

	maxByMetric := make(map[string]float64, len(summaries))
	for _, summary := range summaries {
		maxByMetric[summary.Name] = summary.Max
	}
	require.Equal(t, map[string]float64{
		"process_resident_memory_bytes": 20,
		"go_memstats_heap_alloc_bytes":  40,
		"go_memstats_heap_inuse_bytes":  60,
	}, maxByMetric)
}

func TestMetricsURLWritesTimeSeriesWithoutPerfDash(t *testing.T) {
	artifactsDir := t.TempDir()
	t.Setenv("ARTIFACTS", artifactsDir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`# HELP process_resident_memory_bytes Resident memory size in bytes.
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 123
# HELP go_memstats_heap_alloc_bytes Number of heap bytes allocated and still in use.
# TYPE go_memstats_heap_alloc_bytes gauge
go_memstats_heap_alloc_bytes 456
# HELP go_memstats_heap_inuse_bytes Number of heap bytes that are in use.
# TYPE go_memstats_heap_inuse_bytes gauge
go_memstats_heap_inuse_bytes 789
`))
	}))
	defer server.Close()

	r := NewReportWithOptions("%4.4f", "put", Options{MetricsURL: server.URL})
	go func() {
		start := time.Now()
		r.Results() <- Result{Start: start, End: start.Add(time.Millisecond)}
		close(r.Results())
	}()

	output := <-r.Run()
	require.Contains(t, output, "Resource metrics:")
	require.Contains(t, output, "process_resident_memory_bytes max:")
	require.Contains(t, output, "go_memstats_heap_alloc_bytes max:")
	require.Contains(t, output, "go_memstats_heap_inuse_bytes max:")

	resourceReports, err := filepath.Glob(filepath.Join(artifactsDir, "EtcdResourceMetrics_benchmark_put_*.json"))
	require.NoError(t, err)
	require.Len(t, resourceReports, 1)

	data, err := os.ReadFile(resourceReports[0])
	require.NoError(t, err)
	var resourceReport metricTimeSeriesReport
	require.NoError(t, json.Unmarshal(data, &resourceReport))
	require.Equal(t, "PUT", resourceReport.Operation)
	require.NotEmpty(t, resourceReport.Samples)
	require.Equal(t, map[string]float64{
		"process_resident_memory_bytes": 123,
		"go_memstats_heap_alloc_bytes":  456,
		"go_memstats_heap_inuse_bytes":  789,
	}, resourceReport.Samples[0].Values)

	perfReports, err := filepath.Glob(filepath.Join(artifactsDir, "EtcdAPI_benchmark_put_*.json"))
	require.NoError(t, err)
	require.Empty(t, perfReports)
}
