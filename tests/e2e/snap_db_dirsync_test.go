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
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/robustness/failpoint"
)

// These tests cover etcd's control flow around the snap-directory fsync in
// Snapshotter.SaveDBFrom. They do not simulate loss of an un-fsynced directory
// entry; the durability requirement comes from the operating-system fsync
// contract.
//
// The tests verify that a directory-sync failure aborts snapshot handling and
// is retried, and that the member recovers through restart and resend after a
// process crash before SaveDBFrom returns.

// newSnapDirSyncCluster builds a 3-member cluster with peer proxies (required
// for blackholing) and aggressive snapshotting.
func newSnapDirSyncCluster(t *testing.T) *e2e.EtcdProcessCluster {
	t.Helper()
	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(e2e.NewConfig(
		e2e.WithClusterSize(3),
		e2e.WithKeepDataDir(true),
		e2e.WithPeerProxy(true),
		e2e.WithIsPeerTLS(true),
		e2e.WithSnapshotCount(10),
		e2e.WithSnapshotCatchUpEntries(10),
		e2e.WithGoFailEnabled(true),
	)))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, epc.Close())
	})
	return epc
}

// driveTrafficUntilSnapshot writes against the healthy members while member is
// blackholed, returning only after member is far enough behind that the leader
// must stream it a snapshot. Traffic stops before the return.
func driveTrafficUntilSnapshot(t *testing.T, epc *e2e.EtcdProcessCluster, member e2e.EtcdProcess) {
	t.Helper()
	ctx := t.Context()

	c, err := clientv3.New(clientv3.Config{
		Endpoints:            []string{epc.Procs[0].EndpointsGRPC()[0]},
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	require.NoError(t, err)
	defer c.Close()

	trafficCtx, trafficCancel := context.WithCancel(ctx)
	defer trafficCancel()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-trafficCtx.Done():
				return
			default:
			}
			putCtx, putCancel := context.WithTimeout(trafficCtx, 50*time.Millisecond)
			_, _ = c.Put(putCtx, "a", "b")
			putCancel()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Blackhole returns after the member is behind by more than
	// snapshot-count + snapshot-catchup-entries, with traffic restored.
	require.NoError(t, failpoint.Blackhole(ctx, t, member, epc, true))

	trafficCancel()
	wg.Wait()
}

// TestSnapDBDirSyncErrorRecovery injects a failure into the snap-directory
// fsync and verifies that SaveDBFrom reports it instead of silently
// acknowledging an undurable snapshot db, and that the member recovers once
// the failure is removed.
func TestSnapDBDirSyncErrorRecovery(t *testing.T) {
	epc := newSnapDirSyncCluster(t)
	ctx := t.Context()
	member := epc.Procs[2]

	t.Log("inject a snap-directory fsync failure into the third member")
	require.NoError(t, member.Failpoints().SetupHTTP(ctx, "snapDBDirSyncError", `return("injected snap dir fsync failure")`))

	driveTrafficUntilSnapshot(t, epc, member)

	t.Log("the leader streams a snapshot, and the injected fsync failure must surface loudly")
	e2e.AssertProcessLogs(t, member, "failed to save incoming database snapshot")
	e2e.AssertProcessLogs(t, member, "injected snap dir fsync failure")

	t.Log("remove the failure; the leader resends the snapshot and the member catches up")
	require.NoError(t, member.Failpoints().DeactivateHTTP(ctx, "snapDBDirSyncError"))
	assertKVHash(t, epc)
}

// TestSnapDBReceiveCrashWindow kills a member after the snap.db rename but
// before SaveDBFrom returns, then verifies restart and snapshot resend recover
// the member. SIGKILL does not test directory-entry durability.
func TestSnapDBReceiveCrashWindow(t *testing.T) {
	epc := newSnapDirSyncCluster(t)
	ctx := t.Context()
	member := epc.Procs[2]

	t.Log("widen the crash window: pause the member between the snap.db rename and the directory fsync")
	require.NoError(t, member.Failpoints().SetupHTTP(ctx, "snapDBRenameBeforeDirSync", `sleep("30s")`))

	driveTrafficUntilSnapshot(t, epc, member)

	t.Log("wait until the snap.db rename has happened (the member is inside the crash window)")
	snapDir := filepath.Join(member.Config().DataDirPath, "member", "snap")
	require.Eventuallyf(t, func() bool {
		matches, _ := filepath.Glob(filepath.Join(snapDir, "*.snap.db"))
		return len(matches) > 0
	}, 60*time.Second, 100*time.Millisecond, "member never received a snapshot db")

	t.Log("SIGKILL the member inside the window; the failpoint config dies with the process")
	require.NoError(t, member.Kill())
	require.NoError(t, member.Wait(ctx))

	t.Log("restart the member; it must boot cleanly, receive the snapshot again, and catch up")
	require.NoError(t, member.Start(ctx))
	assertKVHash(t, epc)
}
