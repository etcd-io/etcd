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
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// These tests cover the receiver's error and recovery paths around
// Snapshotter.SaveDBFrom's directory fsync. They do not simulate loss of an
// unsynced directory entry; that behavior follows the operating-system fsync
// contract.

func newSnapDirSyncCluster(t *testing.T) *e2e.EtcdProcessCluster {
	t.Helper()
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ClusterSize:            3,
		KeepDataDir:            true,
		PeerProxy:              true,
		IsPeerTLS:              true,
		SnapshotCount:          10,
		SnapshotCatchUpEntries: 10,
		GoFailEnabled:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, epc.Close())
	})
	return epc
}

// driveTrafficUntilSnapshot blackholes a follower, writes until it is far
// enough behind to require a snapshot, and then restores its peer traffic.
func driveTrafficUntilSnapshot(t *testing.T, epc *e2e.EtcdProcessCluster, member e2e.EtcdProcess) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	leader := epc.Procs[epc.WaitLeader(t)]
	clusterClient, err := clientv3.New(clientv3.Config{
		Endpoints:            leader.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	defer clusterClient.Close()

	memberClient, err := clientv3.New(clientv3.Config{
		Endpoints:            member.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	defer memberClient.Close()

	proxy := member.PeerProxy()
	require.NotNil(t, proxy)
	proxy.BlackholeTx()
	proxy.BlackholeRx()
	defer func() {
		proxy.UnblackholeTx()
		proxy.UnblackholeRx()
	}()

	minEntriesToGuaranteeSnapshot := int64(epc.Cfg.SnapshotCount + epc.Cfg.SnapshotCatchUpEntries)

	for i := 0; ; i++ {
		_, err = clusterClient.Put(ctx, fmt.Sprintf("snap-dir-sync-%d", i), "value")
		require.NoError(t, err)
		if i%5 != 4 {
			continue
		}
		clusterStatus, statusErr := clusterClient.Status(ctx, leader.EndpointsGRPC()[0])
		require.NoError(t, statusErr)
		memberStatus, statusErr := memberClient.Status(ctx, member.EndpointsGRPC()[0])
		require.NoError(t, statusErr)
		if clusterStatus.Header.Revision-memberStatus.Header.Revision > minEntriesToGuaranteeSnapshot {
			return
		}
	}
}

// TestSnapDBDirSyncErrorRecovery verifies that a directory-sync failure fails
// the receive request, then the member receives and applies the retried
// snapshot after the failure is removed.
func TestSnapDBDirSyncErrorRecovery(t *testing.T) {
	epc := newSnapDirSyncCluster(t)
	ctx := context.Background()
	leaderIdx := epc.WaitLeader(t)
	member := epc.Procs[(leaderIdx+1)%len(epc.Procs)]

	t.Log("inject a snap-directory fsync failure")
	require.NoError(t, member.Failpoints().SetupHTTP(ctx, "snapDBDirSyncError", `return("injected snap dir fsync failure")`))

	driveTrafficUntilSnapshot(t, epc, member)

	t.Log("wait for the snapshot receive to fail")
	e2e.AssertProcessLogs(t, member, "failed to save incoming database snapshot")
	e2e.AssertProcessLogs(t, member, "injected snap dir fsync failure")

	t.Log("remove the failure and verify recovery")
	require.NoError(t, member.Failpoints().DeactivateHTTP(ctx, "snapDBDirSyncError"))
	assertKVHash(t, epc)
}

// TestSnapDBReceiveCrashWindow kills a member after the snap.db rename and
// before the directory sync. After restart, it must receive another snapshot
// and catch up. SIGKILL does not test directory-entry durability.
func TestSnapDBReceiveCrashWindow(t *testing.T) {
	epc := newSnapDirSyncCluster(t)
	ctx := context.Background()
	leaderIdx := epc.WaitLeader(t)
	member := epc.Procs[(leaderIdx+1)%len(epc.Procs)]

	t.Log("pause the member after the snap.db rename")
	require.NoError(t, member.Failpoints().SetupHTTP(ctx, "snapDBRenameBeforeDirSync", `sleep("30s")`))

	driveTrafficUntilSnapshot(t, epc, member)

	t.Log("wait for the snap.db rename")
	snapDir := filepath.Join(member.Config().DataDirPath, "member", "snap")
	require.Eventuallyf(t, func() bool {
		matches, _ := filepath.Glob(filepath.Join(snapDir, "*.snap.db"))
		return len(matches) > 0
	}, 60*time.Second, 100*time.Millisecond, "member never received a snapshot db")

	t.Log("kill the member before the directory sync")
	require.NoError(t, member.Kill())
	require.NoError(t, member.Wait(ctx))

	t.Log("restart the member and verify snapshot recovery")
	require.NoError(t, member.Start())
	assertKVHash(t, epc)
}
