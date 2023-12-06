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
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	healthCheckTimeout = 2 * time.Second
	putCommandTimeout  = 200 * time.Millisecond
)

type healthCheckConfig struct {
	url                    string
	expectedStatusCode     int
	expectedTimeoutError   bool
	expectedRespSubStrings []string
}

type injectFailure func(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, duration time.Duration)

func TestHTTPHealthHandler(t *testing.T) {
	e2e.BeforeTest(t)
	client := &http.Client{}
	tcs := []struct {
		name          string
		injectFailure injectFailure
		clusterConfig e2e.EtcdProcessClusterConfig
		healthChecks  []healthCheckConfig
	}{
		{
			name:          "no failures", // happy case
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1},
			healthChecks: []healthCheckConfig{
				{
					url:                "/health",
					expectedStatusCode: http.StatusOK,
				},
			},
		},
		{
			name:          "activated no space alarm",
			injectFailure: triggerNoSpaceAlarm,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1, QuotaBackendBytes: int64(13 * os.Getpagesize())},
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
			name:          "overloaded server slow apply",
			injectFailure: triggerSlowApply,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 3, GoFailEnabled: true},
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
			name:          "network partitioned",
			injectFailure: blackhole,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 3, IsPeerTLS: true, PeerProxy: true},
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
			name:          "raft loop deadlock",
			injectFailure: triggerRaftLoopDeadLock,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1, GoFailEnabled: true},
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
			name:          "slow buffer write back with auth enabled",
			injectFailure: triggerSlowBufferWriteBackWithAuth,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1, GoFailEnabled: true},
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
			clus, err := e2e.NewEtcdProcessCluster(t, &tc.clusterConfig)
			require.NoError(t, err)
			defer clus.Close()
			e2e.ExecuteUntil(ctx, t, func() {
				if tc.injectFailure != nil {
					// guaranteed that failure point is active until all the health checks timeout.
					duration := time.Duration(len(tc.healthChecks)+10) * healthCheckTimeout
					tc.injectFailure(ctx, t, clus, duration)
				}

				for _, hc := range tc.healthChecks {
					requestURL := clus.Procs[0].EndpointsHTTP()[0] + hc.url
					t.Logf("health check URL is %s", requestURL)
					doHealthCheckAndVerify(t, client, requestURL, hc.expectedTimeoutError, hc.expectedStatusCode, hc.expectedRespSubStrings)
				}
			})
		})
	}
}

var (
	defaultHealthCheckConfigs = []healthCheckConfig{
		{
			url:                    "/livez",
			expectedStatusCode:     http.StatusOK,
			expectedRespSubStrings: []string{`ok`},
		},
		{
			url:                    "/readyz",
			expectedStatusCode:     http.StatusOK,
			expectedRespSubStrings: []string{`ok`},
		},
		{
			url:                    "/livez?verbose=true",
			expectedStatusCode:     http.StatusOK,
			expectedRespSubStrings: []string{`[+]serializable_read ok`},
		},
		{
			url:                "/readyz?verbose=true",
			expectedStatusCode: http.StatusOK,
			expectedRespSubStrings: []string{
				`[+]serializable_read ok`,
				`[+]data_corruption ok`,
			},
		},
	}
)

func TestHTTPLivezReadyzHandler(t *testing.T) {
	e2e.BeforeTest(t)
	client := &http.Client{}
	tcs := []struct {
		name          string
		injectFailure injectFailure
		clusterConfig e2e.EtcdProcessClusterConfig
		healthChecks  []healthCheckConfig
	}{
		{
			name:          "no failures", // happy case
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1},
			healthChecks:  defaultHealthCheckConfigs,
		},
		{
			name:          "activated no space alarm",
			injectFailure: triggerNoSpaceAlarm,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1, QuotaBackendBytes: int64(13 * os.Getpagesize())},
			healthChecks:  defaultHealthCheckConfigs,
		},
		// Readiness is not an indicator of performance. Slow response is not covered by readiness.
		// refer to https://tinyurl.com/livez-readyz-design-doc or https://github.com/etcd-io/etcd/issues/16007#issuecomment-1726541091 in case tinyurl is down.
		{
			name:          "overloaded server slow apply",
			injectFailure: triggerSlowApply,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 3, GoFailEnabled: true},
			// TODO expected behavior of readyz check should be 200 after ReadIndex check is implemented to replace linearizable read.
			healthChecks: []healthCheckConfig{
				{
					url:                "/livez",
					expectedStatusCode: http.StatusOK,
				},
				{
					url:                  "/readyz",
					expectedTimeoutError: true,
					expectedStatusCode:   http.StatusServiceUnavailable,
				},
			},
		},
		{
			name:          "network partitioned",
			injectFailure: blackhole,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 3, IsPeerTLS: true, PeerProxy: true},
			healthChecks: []healthCheckConfig{
				{
					url:                "/livez",
					expectedStatusCode: http.StatusOK,
				},
				{
					url:                  "/readyz",
					expectedTimeoutError: true,
					expectedStatusCode:   http.StatusServiceUnavailable,
					expectedRespSubStrings: []string{
						`[-]linearizable_read failed: etcdserver: leader changed`,
					},
				},
			},
		},
		{
			name:          "raft loop deadlock",
			injectFailure: triggerRaftLoopDeadLock,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1, GoFailEnabled: true},
			// TODO expected behavior of livez check should be 503 or timeout after RaftLoopDeadLock check is implemented.
			healthChecks: []healthCheckConfig{
				{
					url:                "/livez",
					expectedStatusCode: http.StatusOK,
				},
				{
					url:                  "/readyz",
					expectedTimeoutError: true,
					expectedStatusCode:   http.StatusServiceUnavailable,
				},
			},
		},
		// verify that auth enabled serializable read must go through mvcc
		{
			name:          "slow buffer write back with auth enabled",
			injectFailure: triggerSlowBufferWriteBackWithAuth,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 1, GoFailEnabled: true},
			healthChecks: []healthCheckConfig{
				{
					url:                  "/livez",
					expectedTimeoutError: true,
				},
				{
					url:                  "/readyz",
					expectedTimeoutError: true,
				},
			},
		},
		{
			name:          "corrupt",
			injectFailure: triggerCorrupt,
			clusterConfig: e2e.EtcdProcessClusterConfig{ClusterSize: 3, CorruptCheckTime: time.Second},
			healthChecks: []healthCheckConfig{
				{
					url:                    "/livez?verbose=true",
					expectedStatusCode:     http.StatusOK,
					expectedRespSubStrings: []string{`[+]serializable_read ok`},
				},
				{
					url:                "/readyz",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedRespSubStrings: []string{
						`[+]serializable_read ok`,
						`[-]data_corruption failed: alarm activated: CORRUPT`,
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			clus, err := e2e.NewEtcdProcessCluster(t, &tc.clusterConfig)
			require.NoError(t, err)
			defer clus.Close()
			e2e.ExecuteUntil(ctx, t, func() {
				if tc.injectFailure != nil {
					// guaranteed that failure point is active until all the health checks timeout.
					duration := time.Duration(len(tc.healthChecks)+10) * healthCheckTimeout
					tc.injectFailure(ctx, t, clus, duration)
				}

				for _, hc := range tc.healthChecks {
					requestURL := clus.Procs[0].EndpointsHTTP()[0] + hc.url
					t.Logf("health check URL is %s", requestURL)
					doHealthCheckAndVerify(t, client, requestURL, hc.expectedTimeoutError, hc.expectedStatusCode, hc.expectedRespSubStrings)
				}
			})
		})
	}
}

func doHealthCheckAndVerify(t *testing.T, client *http.Client, url string, expectTimeoutError bool, expectStatusCode int, expectRespSubStrings []string) {
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
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

	t.Logf("health check response body is:\n%s", body)
	require.Equal(t, expectStatusCode, resp.StatusCode)
	for _, expectRespSubString := range expectRespSubStrings {
		require.Contains(t, string(body), expectRespSubString)
	}
}

func triggerNoSpaceAlarm(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, _ time.Duration) {
	buf := strings.Repeat("b", os.Getpagesize())
	etcdctl := NewEtcdctl(clus.Procs[0].EndpointsV3(), e2e.ClientNonTLS, false, false)
	for {
		if err := etcdctl.Put("foo", buf); err != nil {
			if !strings.Contains(err.Error(), "etcdserver: mvcc: database space exceeded") {
				t.Fatal(err)
			}
			break
		}
	}
}

func triggerSlowApply(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, duration time.Duration) {
	// the following proposal will be blocked at applying stage
	// because when apply index < committed index, linearizable read would time out.
	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "beforeApplyOneEntryNormal", fmt.Sprintf(`sleep("%s")`, duration)))
	etcdctl := NewEtcdctl(clus.Procs[1].EndpointsV3(), e2e.ClientNonTLS, false, false)
	etcdctl.Put("foo", "bar")
}

func blackhole(_ context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, _ time.Duration) {
	member := clus.Procs[0]
	proxy := member.PeerProxy()
	t.Logf("Blackholing traffic from and to member %q", member.Config().Name)
	proxy.BlackholeTx()
	proxy.BlackholeRx()
}

func triggerRaftLoopDeadLock(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, duration time.Duration) {
	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "raftBeforeSaveWaitWalSync", fmt.Sprintf(`sleep("%s")`, duration)))
	etcdctl := NewEtcdctl(clus.Procs[0].EndpointsV3(), e2e.ClientNonTLS, false, false)
	etcdctl.Put("foo", "bar")
}

func triggerSlowBufferWriteBackWithAuth(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, duration time.Duration) {
	etcdctl := NewEtcdctl(clus.Procs[0].EndpointsV3(), e2e.ClientNonTLS, false, false)

	_, err := etcdctl.UserAdd("root", "root")
	require.NoError(t, err)
	_, err = etcdctl.UserGrantRole("root", "root")
	require.NoError(t, err)
	require.NoError(t, etcdctl.AuthEnable())

	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "beforeWritebackBuf", fmt.Sprintf(`sleep("%s")`, duration)))
	etcdctl.PutWithAuth("foo", "bar", "root", "root")
}

func triggerCorrupt(ctx context.Context, t *testing.T, clus *e2e.EtcdProcessCluster, _ time.Duration) {
	etcdctl := NewEtcdctl(clus.Procs[0].EndpointsV3(), e2e.ClientNonTLS, false, false)
	for i := 0; i < 10; i++ {
		require.NoError(t, etcdctl.Put("foo", "bar"))
	}
	err := clus.Procs[0].Stop()
	require.NoError(t, err)
	err = testutil.CorruptBBolt(path.Join(clus.Procs[0].Config().DataDirPath, "member", "snap", "db"))
	require.NoError(t, err)
	err = clus.Procs[0].Start()
	for {
		time.Sleep(time.Second)
		select {
		case <-ctx.Done():
			require.NoError(t, err)
		default:
		}
		response, err := etcdctl.AlarmList()
		if err != nil {
			continue
		}
		if len(response.Alarms) == 0 {
			continue
		}
		require.Len(t, response.Alarms, 1)
		if response.Alarms[0].Alarm == etcdserverpb.AlarmType_CORRUPT {
			break
		}
	}
}
