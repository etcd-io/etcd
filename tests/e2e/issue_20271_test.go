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

	t.Log("Step 1: Ensure that each member has about 1 MiB boltdb file.")
	for i := 0; i < 1024; i++ {
		require.NoError(t, epc.Procs[0].Etcdctl().Put(ctx,
			fmt.Sprintf("foo%d", i),
			strings.Repeat("Oops", 1024),
			config.PutOptions{}))
	}

	t.Log(`Step 2: Currently, it's unable to use PeerProxy to block specific\n
data path from one follower to leader. So here, I use SIGSTOP to pause the third member.`)
	require.NoError(t, epc.Procs[2].Failpoints().SetupHTTP(ctx, "applyAfterOpenSnapshot", `sleep("30s")`))
	epc.Procs[2].(*e2e.EtcdServerProcess).Pause()

	t.Log("Step 3: Delete some key values.")
	for i := 0; i < snapCount+20; i++ {
		_, err := epc.Procs[0].Etcdctl().Delete(ctx, fmt.Sprintf("foo%d", i), config.DeleteOptions{})
		require.NoError(t, err)
	}

	t.Log("Step 4: Restarting the first two members to change term and trigger new leader to send snapshot file to the third member.")
	for _, proc := range epc.Procs[:2] {
		require.NoError(t, proc.Restart(ctx))
	}
	epc.Procs[2].(*e2e.EtcdServerProcess).Resume()

	t.Log(`Step 5: After opening snapshot file from new leader, invoke defragment\n
to override boltdb file. So, for the following changes, the third member will commit them into deleted boltdb file.`)
	e2e.AssertProcessLogs(t, epc.Procs[2], "applySnapshot: opened snapshot backend")
	err = epc.Procs[2].Etcdctl().Defragment(ctx, config.DefragOption{Timeout: 10 * time.Second})
	require.NoError(t, err)
	e2e.AssertProcessLogs(t, epc.Procs[2], "applied snapshot")
	require.NoError(t, epc.Procs[2].Failpoints().DeactivateHTTP(ctx, "applyAfterOpenSnapshot"))

	t.Log("Step 6: Transfer leader to the third member.")
	leaderIndex := epc.WaitMembersForLeader(ctx, t, epc.Procs)
	newLeaderID, _, err := getMemberIDByName(ctx, epc.Procs[0].Etcdctl(), epc.Procs[2].Config().Name)
	require.NoError(t, err)
	require.NoError(t, epc.Procs[leaderIndex].Etcdctl().MoveLeader(ctx, newLeaderID))
	leaderIndex = epc.WaitMembersForLeader(ctx, t, epc.Procs)
	require.Equal(t, leaderIndex, 2)

	t.Log("Step 7: Stop the first member.")
	require.NoError(t, epc.Procs[0].Stop())

	t.Log("Step 8: The third member should be leader")
	leaderIndex = epc.WaitMembersForLeader(ctx, t, epc.Procs[1:])
	require.Equal(t, leaderIndex, 1)

	t.Log("Step 9: The third member will commit them into deleted boltdb file. The boltdb file will be different from deleted one.")
	for i := 0; i < snapCount+50; i++ {
		err := epc.Procs[2].Etcdctl().Put(ctx, fmt.Sprintf("foo%d", i), "bar", config.PutOptions{})
		require.NoError(t, err)
	}

	t.Log("Step 10: Restart the first member and it should be panic")
	require.NoError(t, epc.Procs[0].Start(ctx))
}
