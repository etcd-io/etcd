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

//go:build !cluster_proxy

package e2e

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestPeerStreamReadTimeout verifies that a rafthttp stream reader marks a
// silent peer inactive within a few ConnReadTimeouts (5s default) when the
// peer proxy keeps the TCP connection open while dropping all data (#22638).
func TestPeerStreamReadTimeout(t *testing.T) {
	e2e.BeforeTest(t)

	clus, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithClusterSize(2),
		e2e.WithIsPeerTLS(true),
		e2e.WithPeerProxy(true),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clus.Close()) })

	leaderIdx := clus.WaitLeader(t)
	observer := clus.Procs[leaderIdx]
	silent := clus.Procs[(leaderIdx+1)%len(clus.Procs)]

	waitForActivePeerCount(t, observer, 1, 30*time.Second)
	waitForActivePeerCount(t, silent, 1, 30*time.Second)

	proxy := silent.PeerProxy()
	proxy.BlackholeTx()
	proxy.BlackholeRx()
	t.Cleanup(func() {
		proxy.UnblackholeTx()
		proxy.UnblackholeRx()
	})

	// 15s (3x the 5s default ConnReadTimeout) leaves margin for link-heartbeat
	// spacing and CI scheduling while staying well below ~165s TCP keepalive.
	waitForActivePeerCount(t, observer, 0, 15*time.Second)
}

func waitForActivePeerCount(t *testing.T, m e2e.EtcdProcess, want int, timeout time.Duration) {
	t.Helper()
	metricsURL, err := url.JoinPath(m.EndpointsHTTP()[0], "metrics")
	require.NoError(t, err)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		families, getErr := e2e.GetMetrics(metricsURL)
		if !assert.NoError(c, getErr) {
			return
		}
		mf, ok := families["etcd_network_active_peers"]
		if assert.Truef(c, ok, "etcd_network_active_peers metric not found") && assert.Len(c, mf.GetMetric(), 1) {
			assert.Equal(c, want, int(mf.GetMetric()[0].GetGauge().GetValue()))
		}
	}, timeout, 200*time.Millisecond)
}
