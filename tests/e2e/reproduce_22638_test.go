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

package e2e

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

const (
	// peerConnReadTimeout mirrors rafthttp.DefaultConnReadTimeout, the read
	// deadline every stream reader should apply to its peer connection.
	peerConnReadTimeout = 5 * time.Second
	// silentPeerDetectionBudget leaves room for the writer's heartbeat
	// interval (ConnReadTimeout/3) and for scheduling noise on CI machines,
	// while staying far below the ~165s it takes TCP keepalive to give up.
	silentPeerDetectionBudget = 3 * peerConnReadTimeout
)

// TestReproduce22638 reproduces https://github.com/etcd-io/etcd/issues/22638.
//
// A rafthttp stream reader must notice a peer that stops sending within
// ConnReadTimeout, even when the TCP connection to that peer stays up. That
// happens when something between the members, such as a proxy or load
// balancer, keeps the TCP session alive but stops forwarding data. Since
// v3.5.0 the read deadline was not applied to dialed peer connections, so a
// reader waited until the TCP connection itself broke, which a live proxy
// never lets happen.
//
// The test blackholes one member's peer proxy, which keeps the TCP connections
// open but drops all data, and expects every other member to mark that member
// inactive (etcd_network_active_peers drops to 0) within a few read timeouts.
func TestReproduce22638(t *testing.T) {
	testCases := []struct {
		name    string
		version e2e.ClusterVersion
	}{
		{name: "CurrentVersion", version: e2e.CurrentVersion},
		{name: "LastVersion", version: e2e.LastVersion},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)
			if tc.version == e2e.LastVersion {
				skipIfLastReleaseHas22638(t)
			}
			testSilentPeerDetectedByStreamReaders(t, tc.version)
		})
	}
}

// skipIfLastReleaseHas22638 skips the test when the last release binary is
// missing or still has the regression. Releases before v3.5.0 apply the read
// deadline correctly.
func skipIfLastReleaseHas22638(t *testing.T) {
	t.Helper()
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}
	v, err := e2e.GetVersionFromBinary(e2e.BinPath.EtcdLastRelease)
	require.NoError(t, err)
	// TODO: also run for the release lines the fix gets backported to.
	if v.Major() > 3 || (v.Major() == 3 && v.Minor() >= 5) {
		t.Skipf("last release v%s is affected by https://github.com/etcd-io/etcd/issues/22638", v)
	}
}

func testSilentPeerDetectedByStreamReaders(t *testing.T, version e2e.ClusterVersion) {
	ctx := t.Context()

	clus, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(3),
		e2e.WithIsPeerTLS(true),
		e2e.WithPeerProxy(true),
		e2e.WithVersion(version),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clus.Stop()) })

	// Silence a follower so the cluster keeps its leader and quorum.
	leader := clus.WaitLeader(t)
	silentIdx := (leader + 1) % len(clus.Procs)
	silent := clus.Procs[silentIdx]
	silentID := localMemberID(ctx, t, silent)
	t.Logf("Silencing member %q (%s); leader is %q", silent.Config().Name, silentID, clus.Procs[leader].Config().Name)

	others := make([]e2e.EtcdProcess, 0, len(clus.Procs)-1)
	for i, p := range clus.Procs {
		if i != silentIdx {
			others = append(others, p)
		}
	}

	// Start from a healthy cluster: every member sees the silent member as active.
	for _, p := range others {
		waitActivePeerGauge(ctx, t, p, silentID, 1, 30*time.Second)
	}

	// The proxy in front of the silent member keeps accepting and holding TCP
	// connections but drops every byte in both directions. The other members'
	// stream readers for the silent member dial this proxy, so their TCP peer
	// stays alive and answers keepalives: only the read deadline can detect
	// that the stream went quiet.
	proxy := silent.PeerProxy()
	proxy.BlackholeTx()
	proxy.BlackholeRx()
	t.Cleanup(func() {
		proxy.UnblackholeTx()
		proxy.UnblackholeRx()
	})
	start := time.Now()

	for _, p := range others {
		waitActivePeerGauge(ctx, t, p, silentID, 0, silentPeerDetectionBudget-time.Since(start))
		t.Logf("Member %q marked %s inactive after %v", p.Config().Name, silentID, time.Since(start).Round(100*time.Millisecond))
	}
}

// localMemberID returns the hex member ID of p, read from the Local label of
// its own etcd_network_active_peers series. Metrics are used instead of
// etcdctl so the helper works with every supported release.
func localMemberID(ctx context.Context, t *testing.T, p e2e.EtcdProcess) string {
	t.Helper()
	var id string
	require.Eventuallyf(t, func() bool {
		series, err := activePeerSeries(ctx, p)
		if err != nil {
			return false
		}
		for _, s := range series {
			if s.local != "" {
				id = s.local
			}
		}
		return id != ""
	}, 30*time.Second, 200*time.Millisecond, "member %q never reported etcd_network_active_peers", p.Config().Name)
	return id
}

// waitActivePeerGauge waits until p reports etcd_network_active_peers{Remote=remoteID}
// equal to want, failing the test after timeout.
func waitActivePeerGauge(ctx context.Context, t *testing.T, p e2e.EtcdProcess, remoteID string, want float64, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	last := "no sample"
	for {
		series, err := activePeerSeries(ctx, p)
		switch {
		case err != nil:
			// Keep the last observed value: the final scrape can fail
			// only because ctx expired.
			if last == "no sample" {
				last = err.Error()
			}
		default:
			if s, ok := series[remoteID]; ok {
				if s.value == want {
					return
				}
				last = fmt.Sprintf("%v", s.value)
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("member %q: etcd_network_active_peers{Remote=%q} did not become %v within %v (last: %s)",
				p.Config().Name, remoteID, want, timeout, last)
		case <-ticker.C:
		}
	}
}

type activePeer struct {
	local string
	value float64
}

// activePeerSeries returns p's etcd_network_active_peers samples keyed by the Remote label.
func activePeerSeries(ctx context.Context, p e2e.EtcdProcess) (map[string]activePeer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	metricsURL, err := url.JoinPath(p.EndpointsHTTP()[0], "metrics")
	if err != nil {
		return nil, err
	}
	families, err := e2e.GetMetrics(metricsURL)
	if err != nil {
		return nil, err
	}
	mf, ok := families["etcd_network_active_peers"]
	if !ok {
		return nil, fmt.Errorf("etcd_network_active_peers not found")
	}
	series := make(map[string]activePeer)
	for _, m := range mf.GetMetric() {
		var local, remote string
		for _, l := range m.GetLabel() {
			switch l.GetName() {
			case "Local":
				local = l.GetValue()
			case "Remote":
				remote = l.GetValue()
			}
		}
		series[remote] = activePeer{local: local, value: m.GetGauge().GetValue()}
	}
	return series, nil
}
