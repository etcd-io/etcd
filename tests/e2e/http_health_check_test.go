// Copyright 2023 The etcd Authors
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

//go:build !cluster_proxy

package e2e

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

type healthCheckConfig struct {
	url                  string
	expectedStatusCode   int
	expectedTimeoutError bool
}

func TestHTTPHealthHandler(t *testing.T) {
	e2e.BeforeTest(t)
	client := &http.Client{}
	tcs := []struct {
		name           string
		injectFailure  func(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster)
		clusterOptions []e2e.EPClusterOption
		healthChecks   []healthCheckConfig
	}{
		{
			name:           "no failures", // happy case
			clusterOptions: []e2e.EPClusterOption{e2e.WithClusterSize(1)},
			healthChecks: []healthCheckConfig{
				{
					url:                "/health",
					expectedStatusCode: http.StatusOK,
				},
			},
		},
		{
			name:           "activated no space alarm",
			injectFailure:  triggerNoSpaceAlarm,
			clusterOptions: []e2e.EPClusterOption{e2e.WithClusterSize(1), e2e.WithQuotaBackendBytes(int64(13 * os.Getpagesize()))},
			healthChecks: []healthCheckConfig{
				{
					url:                "/health",
					expectedStatusCode: http.StatusServiceUnavailable,
				},
				{
					url:                "/health?exclude=NOSPACE",
					expectedStatusCode: http.StatusOK,
				},
			},
		},
		{
			name:           "overloaded server slow apply",
			injectFailure:  triggerSlowApply,
			clusterOptions: []e2e.EPClusterOption{e2e.WithClusterSize(3), e2e.WithGoFailEnabled(true)},
			healthChecks: []healthCheckConfig{
				{
					url:                "/health?serializable=true",
					expectedStatusCode: http.StatusOK,
				},
				{
					url:                  "/health?serializable=false",
					expectedTimeoutError: true,
				},
			},
		},
		{
			name:           "network partitioned",
			injectFailure:  blackhole,
			clusterOptions: []e2e.EPClusterOption{e2e.WithClusterSize(3), e2e.WithIsPeerTLS(true), e2e.WithPeerProxy(true)},
			healthChecks: []healthCheckConfig{
				{
					url:                "/health?serializable=true",
					expectedStatusCode: http.StatusOK,
				},
				{
					url:                  "/health?serializable=false",
					expectedTimeoutError: true,
					expectedStatusCode:   http.StatusServiceUnavailable,
					// old leader may return "etcdserver: leader changed" error with 503 in ReadIndex leaderChangedNotifier
				},
			},
		},
		{
			name:           "raft loop deadlock",
			injectFailure:  triggerRaftLoopDeadLock,
			clusterOptions: []e2e.EPClusterOption{e2e.WithClusterSize(1), e2e.WithGoFailEnabled(true)},
			healthChecks: []healthCheckConfig{
				{
					// current kubeadm etcd liveness check failed to detect raft loop deadlock in steady state
					// ref. https://github.com/kubernetes/kubernetes/blob/master/cmd/kubeadm/app/phases/etcd/local.go#L225-L226
					// current liveness probe depends on the etcd /health check has a flaw that new /livez check should resolve.
					url:                "/health?serializable=true",
					expectedStatusCode: http.StatusOK,
				},
				{
					url:                  "/health?serializable=false",
					expectedTimeoutError: true,
				},
			},
		},
		// verify that auth enabled serializable read must go through mvcc
		{
			name:           "slow buffer write back with auth enabled",
			injectFailure:  triggerSlowBufferWriteBackWithAuth,
			clusterOptions: []e2e.EPClusterOption{e2e.WithClusterSize(1), e2e.WithGoFailEnabled(true)},
			healthChecks: []healthCheckConfig{
				{
					url:                  "/health?serializable=true",
					expectedTimeoutError: true,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			clus, err := e2e.NewEtcdProcessCluster(ctx, t, tc.clusterOptions...)
			require.NoError(t, err)
			defer clus.Close()
			testutils.ExecuteUntil(ctx, t, func() {
				if tc.injectFailure != nil {
					tc.injectFailure(ctx, t, clus)
				}

				for _, hc := range tc.healthChecks {
					requestURL := clus.Procs[0].EndpointsHTTP()[0] + hc.url
					t.Logf("health check URL is %s", requestURL)
					doHealthCheckAndVerify(t, client, requestURL, hc.expectedStatusCode, hc.expectedTimeoutError)
				}
			})
		})
	}
}

func doHealthCheckAndVerify(t *testing.T, client *http.Client, url string, expectStatusCode int, expectTimeoutError bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	require.NoErrorf(t, err, "failed to creat request %+v", err)
	resp, herr := client.Do(req)
	cancel()
	if expectTimeoutError {
		if herr != nil && strings.Contains(herr.Error(), context.DeadlineExceeded.Error()) {
			return
		}
	}
	require.NoErrorf(t, herr, "failed to get response %+v", err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoErrorf(t, err, "failed to read response %+v", err)

	t.Logf("health check response body is: %s", body)
	require.Equal(t, expectStatusCode, resp.StatusCode)
}

func triggerNoSpaceAlarm(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster) {
	buf := strings.Repeat("b", os.Getpagesize())
	etcdctl := clus.Etcdctl()
	for {
		if err := etcdctl.Put(ctx, "foo", buf, config.PutOptions{}); err != nil {
			if !strings.Contains(err.Error(), "etcdserver: mvcc: database space exceeded") {
				t.Fatal(err)
			}
			break
		}
	}
}

func triggerSlowApply(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster) {
	// the following proposal will be blocked at applying stage
	// because when apply index < committed index, linearizable read would time out.
	err := clus.Procs[0].Failpoints().SetupHTTP(ctx, "beforeApplyOneEntryNormal", `sleep("3s")`)
	if err != nil {
		t.Skip("failpoint beforeApplyOneEntryNormal is not registered; please run `make gofail-enable` and compile etcd binary")
	}
	require.NoError(t, clus.Procs[1].Etcdctl().Put(ctx, "foo", "bar", config.PutOptions{}))
}

func blackhole(_ context.Context, t *testing.T, clus *e2e.EtcdProcessCluster) {
	member := clus.Procs[0]
	proxy := member.PeerProxy()
	t.Logf("Blackholing traffic from and to member %q", member.Config().Name)
	proxy.BlackholeTx()
	proxy.BlackholeRx()
}

func triggerRaftLoopDeadLock(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster) {
	err := clus.Procs[0].Failpoints().SetupHTTP(ctx, "raftBeforeSave", `sleep("3s")`)
	if err != nil {
		t.Skip("failpoint raftBeforeSave is not registered; please run `make gofail-enable` and compile etcd binary")
	}
	clus.Procs[0].Etcdctl().Put(context.Background(), "foo", "bar", config.PutOptions{})
}

func triggerSlowBufferWriteBackWithAuth(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster) {
	etcdctl := clus.Etcdctl()
	_, err := etcdctl.UserAdd(ctx, "root", "root", config.UserAddOptions{})
	require.NoError(t, err)
	_, err = etcdctl.UserGrantRole(ctx, "root", "root")
	require.NoError(t, err)
	require.NoError(t, etcdctl.AuthEnable(ctx))

	err = clus.Procs[0].Failpoints().SetupHTTP(ctx, "beforeWritebackBuf", `sleep("3s")`)
	if err != nil {
		t.Skip("failpoint beforeWritebackBuf is not registered; please run `make gofail-enable` and compile etcd binary")
	}
	clus.Procs[0].Etcdctl(e2e.WithAuth("root", "root")).Put(context.Background(), "foo", "bar", config.PutOptions{Timeout: 200 * time.Millisecond})
}
