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

//go:build !cluster_proxy

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestIssue20271 reproduces the issue: https://github.com/etcd-io/etcd/issues/20271.
func TestIssue20271(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := t.Context()

	snapCount := 10

	cfg := e2e.NewConfig(
		e2e.WithSnapshotCount(uint64(snapCount)),
		e2e.WithSnapshotCatchUpEntries(uint64(snapCount)),
		e2e.WithClusterSize(3),
		e2e.WithKeepDataDir(true),
		e2e.WithGoFailEnabled(true),
		e2e.WithLogLevel("debug"),
		e2e.WithRollingStart(true),
		e2e.WithInitialLeaderIndex(0),
	)
	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, epc.Close())
	}()

	t.Log("Step 1: Write some data to the cluster")
	for i := 0; i < snapCount*5; i++ {
		require.NoError(t, epc.Procs[0].Etcdctl().Put(ctx,
			fmt.Sprintf("foo%d", i),
			strings.Repeat("Oops", 1024),
			config.PutOptions{}))
	}

	t.Log(`Step 2: Config the third member to sleep 15s after OpenSnapshotBackend and use SIGSTOP to pause it.`)
	require.NoError(t, epc.Procs[2].Failpoints().SetupHTTP(ctx, "applyAfterOpenSnapshot", `sleep("15s")`))
	epc.Procs[2].Pause()

	t.Log("Step 3: Delete some key values to trigger new snapshot on the first two members")
	for i := 0; i < snapCount+20; i++ {
		_, err = epc.Procs[0].Etcdctl().Delete(ctx, fmt.Sprintf("foo%d", i), config.DeleteOptions{})
		require.NoError(t, err)
	}

	t.Log("Step 4: Restarting the first two members to change term and trigger new leader to send snapshot file to the third member.")
	for _, proc := range epc.Procs[:2] {
		require.NoError(t, proc.Restart(ctx))
	}
	epc.Procs[2].Resume()

	t.Log(`Step 5: After opening snapshot file from new leader, invoke defragment\n
to override boltdb file. So, for the following changes, the third member will commit them into deleted boltdb file.`)
	e2e.AssertProcessLogs(t, epc.Procs[2], "applySnapshot: opened snapshot backend")
	err = epc.Procs[2].Etcdctl().Defragment(ctx, config.DefragOption{Timeout: 30 * time.Second})
	require.NoError(t, err)
	e2e.AssertProcessLogs(t, epc.Procs[2], "applied snapshot")
	require.NoError(t, epc.Procs[2].Failpoints().DeactivateHTTP(ctx, "applyAfterOpenSnapshot"))

	t.Log("Step 6: Write some data to the cluster")
	for i := 0; i < snapCount/2; i++ {
		err = epc.Procs[0].Etcdctl().Put(ctx, fmt.Sprintf("foo%d", i), strings.Repeat("Oops", 1), config.PutOptions{})
		require.NoError(t, err)
	}

	t.Log("Step 7: Restart the third member. It recovers from the new boltdb file. Therefore, data writen in Step 6 is lost.")
	require.NoError(t, epc.Procs[2].Restart(ctx))

	t.Log("Step 8: Check hashkv of each member")
	assertKVHash(t, epc)
}
