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

package e2e

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/stringutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"

	"github.com/stretchr/testify/require"
)

// TestReproduce19406 reproduces the issue: https://github.com/etcd-io/etcd/issues/19406
func TestReproduce19406(t *testing.T) {
	e2e.BeforeTest(t)

	compactionSleepInterval := 100 * time.Millisecond
	ctx := t.Context()

	clus, cerr := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithGoFailEnabled(true),
		e2e.WithCompactionBatchLimit(1),
		e2e.WithCompactionSleepInterval(compactionSleepInterval),
	)
	require.NoError(t, cerr)
	t.Cleanup(func() { require.NoError(t, clus.Stop()) })

	// Produce some data
	cli := newClient(t, clus.EndpointsGRPC(), e2e.ClientConfig{})
	valueSize := 10
	var latestRevision int64

	produceKeyNum := 20
	for i := 0; i <= produceKeyNum; i++ {
		resp, err := cli.Put(ctx, fmt.Sprintf("%d", i), stringutil.RandString(uint(valueSize)))
		require.NoError(t, err)
		latestRevision = resp.Header.Revision
	}

	// Sleep for PerCompactionInterationInterval to simulate a single iteration of compaction lasting at least this duration.
	PerCompactionInterationInterval := compactionSleepInterval
	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "compactAfterAcquiredBatchTxLock",
		fmt.Sprintf(`sleep("%s")`, PerCompactionInterationInterval)))

	// start compaction
	t.Log("start compaction...")
	_, err := cli.Compact(ctx, latestRevision, clientv3.WithCompactPhysical())
	require.NoError(t, err)
	t.Log("finished compaction...")

	// Validate that total compaction sleep interval
	// Compaction runs in batches. During each batch, it acquires a lock, releases it at the end,
	// and then waits for a compactionSleepInterval before starting the next batch. This pause
	// allows PUT requests to be processed.
	// Therefore, the total compaction sleep interval larger or equal to
	// (compaction iteration number - 1) * compactionSleepInterval
	httpEndpoint := clus.EndpointsHTTP()[0]
	totalKeys := produceKeyNum + 1
	pauseDuration, totalDuration := getEtcdCompactionMetrics(t, httpEndpoint)
	require.NoError(t, err)
	actualSleepInterval := time.Duration(totalDuration-pauseDuration) * time.Millisecond
	expectSleepInterval := compactionSleepInterval * time.Duration(totalKeys)
	t.Logf("db_compaction_pause_duration: %.2f db_compaction_total_duration: %.2f, totalKeys: %d",
		pauseDuration, totalDuration, totalKeys)
	require.GreaterOrEqualf(t, actualSleepInterval, expectSleepInterval,
		"expect total compact sleep interval larger than (%v) but got (%v)",
		expectSleepInterval, actualSleepInterval)
}

func getEtcdCompactionMetrics(t *testing.T, httpEndpoint string) (pauseDuration, totalDuration float64) {
	metricsURL, err := url.JoinPath(httpEndpoint, "metrics")
	require.NoError(t, err)

	// Fetch metrics from the endpoint
	metricFamilies, err := e2e.GetMetrics(metricsURL)
	require.NoError(t, err)

	// Extract sum from histogram metric
	getHistogramSum := func(name string) float64 {
		mf, ok := metricFamilies[name]
		require.Truef(t, ok, "metric %q not found", name)
		require.NotEmptyf(t, mf.Metric, "metric %q has no data", name)

		hist := mf.Metric[0].GetHistogram()
		require.NotEmptyf(t, hist, "metric %q is not a histogram", name)

		return hist.GetSampleSum()
	}

	pauseDuration = getHistogramSum("etcd_debugging_mvcc_db_compaction_pause_duration_milliseconds")
	totalDuration = getHistogramSum("etcd_debugging_mvcc_db_compaction_total_duration_milliseconds")

	return pauseDuration, totalDuration
}
