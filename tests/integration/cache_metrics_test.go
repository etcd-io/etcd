// Copyright 2025 The etcd Authors
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

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	cache "go.etcd.io/etcd/cache/v3"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestCacheMetricsStoreAfterPutsAndGets(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)
	ctx := t.Context()

	mux := http.NewServeMux()
	c, err := cache.New(client, "/m/",
		cache.WithHistoryWindowSize(32),
		cache.WithMetrics(mux),
		cache.WithMetricsPath("/test/metrics"),
	)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	require.NoError(t, c.WaitReady(ctx))

	var lastRev int64
	for i := 0; i < 5; i++ {
		resp, putErr := client.Put(ctx, fmt.Sprintf("/m/key%c", 'a'+i), "val")
		require.NoError(t, putErr)
		lastRev = resp.Header.Revision
	}
	require.NoError(t, c.WaitForRevision(ctx, lastRev))

	_, err = c.Get(ctx, "/m/", clientv3.WithPrefix(), clientv3.WithSerializable())
	require.NoError(t, err)
	_, err = c.Get(ctx, "/m/keya", clientv3.WithSerializable())
	require.NoError(t, err)

	families := scrapeHTTPMetrics(t, mux, "/test/metrics")

	for _, name := range []string{
		"etcd_cache_store_keys_total",
		"etcd_cache_store_latest_revision",
		"etcd_cache_get_duration_seconds",
		"etcd_cache_get_total",
		"etcd_cache_demux_active_watchers",
		"etcd_cache_demux_history_size",
	} {
		require.Contains(t, families, name, "expected metric family %q", name)
	}

	keysTotal := gaugeValue(t, families, "etcd_cache_store_keys_total")
	require.Equal(t, keysTotal, 5.0, "expected store_keys_total to be 5")

	latestRev := gaugeValue(t, families, "etcd_cache_store_latest_revision")
	require.Equal(t, latestRev, float64(lastRev), "expected store_latest_revision to be %d", lastRev)

	getSuccessCount := counterValue(t, families, "etcd_cache_get_total", "result", "success")
	require.Equal(t, getSuccessCount, 2.0, "expected get_total{success} to be 2")
}

func TestCacheMetricsWatchRegistration(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)
	ctx := t.Context()

	mux := http.NewServeMux()
	c, err := cache.New(client, "/w/",
		cache.WithHistoryWindowSize(32),
		cache.WithMetrics(mux),
	)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	require.NoError(t, c.WaitReady(ctx))

	watchCtx1, cancel1 := context.WithCancel(ctx)
	defer cancel1()
	ch1 := c.Watch(watchCtx1, "/w/", clientv3.WithPrefix())
	require.NotNil(t, ch1)

	watchCtx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	ch2 := c.Watch(watchCtx2, "/w/key", clientv3.WithPrefix())
	require.NotNil(t, ch2)

	time.Sleep(50 * time.Millisecond)

	families := scrapeHTTPMetrics(t, mux, cache.DefaultMetricsPath)

	watchSuccess := counterValue(t, families, "etcd_cache_watch_total", "result", "success")
	require.Equal(t, watchSuccess, 2.0, "expected watch_total{success} to be 2")
	require.Contains(t, families, "etcd_cache_watch_register_duration_seconds",
		"expected watch_register_duration_seconds metric")

	cancel1()
	cancel2()
}

func TestCacheMetricsDeleteUpdatesKeyCount(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)
	ctx := t.Context()

	mux := http.NewServeMux()
	c, err := cache.New(client, "/d/",
		cache.WithHistoryWindowSize(32),
		cache.WithMetrics(mux),
	)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	require.NoError(t, c.WaitReady(ctx))

	var rev int64
	for _, key := range []string{"/d/x", "/d/y", "/d/z"} {
		resp, putErr := client.Put(ctx, key, "v")
		require.NoError(t, putErr)
		rev = resp.Header.Revision
	}
	require.NoError(t, c.WaitForRevision(ctx, rev))

	families := scrapeHTTPMetrics(t, mux, cache.DefaultMetricsPath)
	keysAfterPuts := gaugeValue(t, families, "etcd_cache_store_keys_total")
	require.Equal(t, keysAfterPuts, 3.0)

	// Update operation should not increase key count
	resp, putErr := client.Put(ctx, "/d/x", "newVal")
	require.NoError(t, putErr)
	require.NoError(t, c.WaitForRevision(ctx, resp.Header.Revision))

	families = scrapeHTTPMetrics(t, mux, cache.DefaultMetricsPath)
	keysAfterPuts = gaugeValue(t, families, "etcd_cache_store_keys_total")
	require.Equal(t, keysAfterPuts, 3.0)

	delResp, err := client.Delete(ctx, "/d/z")
	require.NoError(t, err)
	require.NoError(t, c.WaitForRevision(ctx, delResp.Header.Revision))

	families = scrapeHTTPMetrics(t, mux, cache.DefaultMetricsPath)
	keysAfterDelete := gaugeValue(t, families, "etcd_cache_store_keys_total")
	require.Equal(t, keysAfterDelete, 2.0)
}

func TestCacheMetricsGetErrors(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)
	ctx := t.Context()

	mux := http.NewServeMux()
	c, err := cache.New(client, "/g/",
		cache.WithHistoryWindowSize(32),
		cache.WithMetrics(mux),
	)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	require.NoError(t, c.WaitReady(ctx))

	_, err = c.Get(ctx, "/outside/prefix", clientv3.WithSerializable())
	require.Error(t, err)

	families := scrapeHTTPMetrics(t, mux, cache.DefaultMetricsPath)
	errorCount := counterValue(t, families, "etcd_cache_get_total", "result", "error")
	require.Equal(t, errorCount, 1.0, "expected get_total{error} >= 1 after out-of-scope Get")
}

func TestCacheMetricsDefaultPath(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	mux := http.NewServeMux()
	c, err := cache.New(client, "",
		cache.WithHistoryWindowSize(8),
		cache.WithMetrics(mux),
	)
	require.NoError(t, err)
	t.Cleanup(c.Close)

	families := scrapeHTTPMetrics(t, mux, cache.DefaultMetricsPath)
	require.Contains(t, families, "etcd_cache_store_keys_total",
		"expected metrics served on default path %s", cache.DefaultMetricsPath)
}

func TestCacheMetricsNilMuxNoEndpoint(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	t.Cleanup(func() { clus.Terminate(t) })
	client := clus.Client(0)

	c, err := cache.New(client, "",
		cache.WithHistoryWindowSize(8),
	)
	require.NoError(t, err)
	t.Cleanup(c.Close)
	require.NoError(t, c.WaitReady(t.Context()))
}

// scrapeHTTPMetrics does an in-process HTTP GET against the mux and parses the
// Prometheus text exposition format into metric families.
func scrapeHTTPMetrics(t *testing.T, mux *http.ServeMux, path string) map[string]*dto.MetricFamily {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	b, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(b))
	require.NoError(t, err)
	return families
}

// gaugeValue returns the value of a gauge metric with no labels.
func gaugeValue(t *testing.T, families map[string]*dto.MetricFamily, name string) float64 {
	t.Helper()
	fam, ok := families[name]
	require.Truef(t, ok, "metric family %q not found", name)
	require.NotEmpty(t, fam.GetMetric(), "metric family %q has no samples", name)
	return fam.GetMetric()[0].GetGauge().GetValue()
}

// counterValue returns the value of a counter metric matching the given label key/value.
func counterValue(t *testing.T, families map[string]*dto.MetricFamily, name, labelKey, labelVal string) float64 {
	t.Helper()
	fam, ok := families[name]
	require.Truef(t, ok, "metric family %q not found", name)
	for _, m := range fam.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == labelKey && lp.GetValue() == labelVal {
				return m.GetCounter().GetValue()
			}
		}
	}
	t.Fatalf("metric %s{%s=%q} not found", name, labelKey, labelVal)
	return 0
}
