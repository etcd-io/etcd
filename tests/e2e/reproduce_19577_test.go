package e2e

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestReproduce19577(t *testing.T) {
	e2e.BeforeTest(t)

	ctx, cancel := context.WithCancel(context.Background())

	clus, cerr := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithGoFailEnabled(true),
	)
	require.NoError(t, cerr)
	t.Cleanup(func() { require.NoError(t, clus.Close()) })

	cli := newClient(t, clus.EndpointsGRPC(), e2e.ClientConfig{})

	// setup the failpoint to ensure there's a race between cancel and close
	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "beforeWatchStreamCancelWhileUnlocked",
		fmt.Sprintf(`sleep("%s")`, time.Second*3)))

	key := "19577"

	// create some revisions for the watcher to watch
	_, err := cli.Put(ctx, key, "value")
	require.NoError(t, err)

	rsp, err := cli.Put(ctx, key, "value")
	require.NoError(t, err)

	// compact the latest revision
	_, err = cli.Compact(ctx, rsp.Header.Revision)
	require.NoError(t, err)

	// We must create two watchers to ensure that when one watcher is cancelled, we send a specific
	// cancel request for that watch ID, rather than closing the entire stream.
	_ = cli.Watch(ctx, key, clientv3.WithRev(rsp.Header.Revision-1))
	_ = cli.Watch(context.WithoutCancel(ctx), key, clientv3.WithRev(rsp.Header.Revision-1))

	httpEndpoint := clus.EndpointsHTTP()[0]

	watcherCount := getEtcdDebuggingWatchCountGauge(t, httpEndpoint)
	require.Equal(t, 2.0, watcherCount)

	cancel()

	// ensure some delay between the cancel and the close, so that we know we initiate the cancel before
	// the close on the server end.
	time.Sleep(time.Second * 1)

	cli.Close()

	// We must wait longer than the failpoint to ensure we observe the state of the metric after
	// the cancel/close race has completed.
	time.Sleep(time.Second * 4)

	watcherCount = getEtcdDebuggingWatchCountGauge(t, httpEndpoint)
	require.Equal(t, 0.0, watcherCount)
}

func getEtcdDebuggingWatchCountGauge(t *testing.T, httpEndpoint string) (watchCount float64) {
	metricsURL, err := url.JoinPath(httpEndpoint, "metrics")
	require.NoError(t, err)

	// Fetch metrics from the endpoint
	metricFamilies, err := e2e.GetMetrics(metricsURL)
	require.NoError(t, err)

	name := "etcd_debugging_mvcc_watcher_total"
	mf, ok := metricFamilies[name]
	require.Truef(t, ok, "metric %q not found", name)
	require.NotEmptyf(t, mf.Metric, "metric %q has no data", name)

	gauge := mf.Metric[0].GetGauge()
	require.NotNilf(t, gauge, "metric %q is not a gauge", name)

	return gauge.GetValue()
}
