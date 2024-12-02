// Copyright 2017 The etcd Authors
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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestV3MetricsSecure(t *testing.T) {
	cfg := e2e.NewConfigTLS()
	cfg.ClusterSize = 1
	cfg.MetricsURLScheme = "https"
	testCtl(t, metricsTest)
}

func TestV3MetricsInsecure(t *testing.T) {
	cfg := e2e.NewConfigTLS()
	cfg.ClusterSize = 1
	cfg.MetricsURLScheme = "http"
	testCtl(t, metricsTest)
}

func TestV3LearnerMetricRecover(t *testing.T) {
	cfg := e2e.NewConfigTLS()
	cfg.ServerConfig.SnapshotCount = 10
	testCtl(t, learnerMetricRecoverTest, withCfg(*cfg))
}

func TestV3LearnerMetricApplyFromSnapshotTest(t *testing.T) {
	cfg := e2e.NewConfigTLS()
	cfg.ServerConfig.SnapshotCount = 10
	testCtl(t, learnerMetricApplyFromSnapshotTest, withCfg(*cfg))
}

func metricsTest(cx ctlCtx) {
	require.NoError(cx.t, ctlV3Put(cx, "k", "v", ""))

	i := 0
	for _, test := range []struct {
		endpoint, expected string
	}{
		{"/metrics", "etcd_mvcc_put_total 2"},
		{"/metrics", "etcd_debugging_mvcc_keys_total 1"},
		{"/metrics", "etcd_mvcc_delete_total 3"},
		{"/metrics", fmt.Sprintf(`etcd_server_version{server_version="%s"} 1`, version.Version)},
		{"/metrics", fmt.Sprintf(`etcd_cluster_version{cluster_version="%s"} 1`, version.Cluster(version.Version))},
		{"/metrics", `grpc_server_handled_total{grpc_code="Canceled",grpc_method="Watch",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"} 6`},
		{"/health", `{"health":"true","reason":""}`},
	} {
		i++
		require.NoError(cx.t, ctlV3Put(cx, fmt.Sprintf("%d", i), "v", ""))
		require.NoError(cx.t, ctlV3Del(cx, []string{fmt.Sprintf("%d", i)}, 1))
		require.NoError(cx.t, ctlV3Watch(cx, []string{"k", "--rev", "1"}, []kvExec{{key: "k", val: "v"}}...))
		require.NoErrorf(cx.t, e2e.CURLGet(cx.epc, e2e.CURLReq{Endpoint: test.endpoint, Expected: expect.ExpectedResponse{Value: test.expected}}), "failed get with curl")
	}
}

func learnerMetricRecoverTest(cx ctlCtx) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := cx.epc.StartNewProc(ctx, nil, cx.t, true /* addAsLearner */)
	require.NoError(cx.t, err)
	expectLearnerMetrics(cx)

	triggerSnapshot(ctx, cx)

	// Restart cluster
	require.NoError(cx.t, cx.epc.Restart(ctx))
	expectLearnerMetrics(cx)
}

func learnerMetricApplyFromSnapshotTest(cx ctlCtx) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Add learner but do not start it
	_, learnerCfg, err := cx.epc.AddMember(ctx, nil, cx.t, true /* addAsLearner */)
	require.NoError(cx.t, err)

	triggerSnapshot(ctx, cx)

	// Start the learner
	require.NoError(cx.t, cx.epc.StartNewProcFromConfig(ctx, cx.t, learnerCfg))
	expectLearnerMetrics(cx)
}

func triggerSnapshot(ctx context.Context, cx ctlCtx) {
	etcdctl := cx.epc.Procs[0].Etcdctl()
	for i := 0; i < int(cx.epc.Cfg.ServerConfig.SnapshotCount); i++ {
		require.NoError(cx.t, etcdctl.Put(ctx, "k", "v", config.PutOptions{}))
	}
}

func expectLearnerMetrics(cx ctlCtx) {
	expectLearnerMetric(cx, 0, "etcd_server_is_learner 0")
	expectLearnerMetric(cx, 1, "etcd_server_is_learner 1")
}

func expectLearnerMetric(cx ctlCtx, procIdx int, expectMetric string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := e2e.CURLPrefixArgsCluster(cx.epc.Cfg, cx.epc.Procs[procIdx], "GET", e2e.CURLReq{Endpoint: "/metrics"})
	require.NoError(cx.t, e2e.SpawnWithExpectsContext(ctx, args, nil, expect.ExpectedResponse{Value: expectMetric}))
}
