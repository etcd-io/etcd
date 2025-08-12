// Copyright 2022 The etcd Authors
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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

type clusterTestCase struct {
	name   string
	config *e2e.EtcdProcessClusterConfig
}

func clusterTestCases(size int) []clusterTestCase {
	tcs := []clusterTestCase{
		{
			name:   "CurrentVersion",
			config: e2e.NewConfig(e2e.WithClusterSize(size)),
		},
	}
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		return tcs
	}

	tcs = append(tcs,
		clusterTestCase{
			name:   "LastVersion",
			config: e2e.NewConfig(e2e.WithClusterSize(size), e2e.WithVersion(e2e.LastVersion)),
		},
	)
	if size > 2 {
		tcs = append(tcs,
			clusterTestCase{
				name:   "MinorityLastVersion",
				config: e2e.NewConfig(e2e.WithClusterSize(size), e2e.WithVersion(e2e.MinorityLastVersion)),
			}, clusterTestCase{
				name:   "QuorumLastVersion",
				config: e2e.NewConfig(e2e.WithClusterSize(size), e2e.WithVersion(e2e.QuorumLastVersion)),
			},
		)
	}
	return tcs
}

// TestMixVersionsSnapshotByAddingMember tests the mix version send snapshots by adding member
func TestMixVersionsSnapshotByAddingMember(t *testing.T) {
	for _, tc := range clusterTestCases(1) {
		t.Run(tc.name+"-adding-new-member-of-current-version", func(t *testing.T) {
			mixVersionsSnapshotTestByAddingMember(t, tc.config, e2e.CurrentVersion)
		})
		// etcd doesn't support adding a new member of old version into
		// a cluster with higher version. For example, etcd cluster
		// version is 3.6.x, then a new member of 3.5.x can't join the
		// cluster. Please refer to link below,
		// https://github.com/etcd-io/etcd/blob/3e903d0b12e399519a4013c52d4635ec8bdd6863/server/etcdserver/cluster_util.go#L222-L230
		/*t.Run(tc.name+"-adding-new-member-of-last-version", func(t *testing.T) {
			mixVersionsSnapshotTestByAddingMember(t, tc.config, e2e.LastVersion)
		})*/
	}
}

func mixVersionsSnapshotTestByAddingMember(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, newInstanceVersion e2e.ClusterVersion) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	t.Logf("Create an etcd cluster with %d member", cfg.ClusterSize)
	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithConfig(cfg),
		e2e.WithSnapshotCount(10),
	)
	require.NoErrorf(t, err, "failed to start etcd cluster")
	defer func() {
		derr := epc.Close()
		require.NoErrorf(t, derr, "failed to close etcd cluster")
	}()

	t.Log("Writing 20 keys to the cluster (more than SnapshotCount entries to trigger at least a snapshot)")
	writeKVs(t, epc.Etcdctl(), 0, 20)

	t.Log("start a new etcd instance, which will receive a snapshot from the leader.")
	newCfg := *epc.Cfg
	newCfg.Version = newInstanceVersion
	newCfg.ServerConfig.SnapshotCatchUpEntries = 10
	t.Log("Starting a new etcd instance")
	_, err = epc.StartNewProc(t.Context(), &newCfg, t, false /* addAsLearner */)
	require.NoErrorf(t, err, "failed to start the new etcd instance")
	defer epc.CloseProc(t.Context(), nil)

	assertKVHash(t, epc)
}

func TestMixVersionsSnapshotByMockingPartition(t *testing.T) {
	mockPartitionNodeIndex := 2
	for _, tc := range clusterTestCases(3) {
		t.Run(tc.name, func(t *testing.T) {
			mixVersionsSnapshotTestByMockPartition(t, tc.config, mockPartitionNodeIndex)
		})
	}
}

func mixVersionsSnapshotTestByMockPartition(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, mockPartitionNodeIndex int) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	clusterOptions := []e2e.EPClusterOption{
		e2e.WithConfig(cfg),
		e2e.WithSnapshotCount(10),
		e2e.WithSnapshotCatchUpEntries(10),
	}
	t.Logf("Create an etcd cluster with %d member", cfg.ClusterSize)
	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, clusterOptions...)
	require.NoErrorf(t, err, "failed to start etcd cluster")
	defer func() {
		derr := epc.Close()
		require.NoErrorf(t, derr, "failed to close etcd cluster")
	}()
	toPartitionedMember := epc.Procs[mockPartitionNodeIndex]

	t.Log("Stop and restart the partitioned member")
	err = toPartitionedMember.Stop()
	require.NoError(t, err)

	t.Log("Writing 30 keys to the cluster (more than SnapshotCount entries to trigger at least a snapshot)")
	writeKVs(t, epc.Etcdctl(), 0, 30)

	t.Log("Verify logs to check leader has saved snapshot")
	leaderEPC := epc.Procs[epc.WaitLeader(t)]
	e2e.AssertProcessLogs(t, leaderEPC, "saved snapshot")

	t.Log("Restart the partitioned member")
	err = toPartitionedMember.Restart(t.Context())
	require.NoError(t, err)

	assertKVHash(t, epc)

	leaderEPC = epc.Procs[epc.WaitLeader(t)]
	t.Log("Verify logs to check snapshot be sent from leader to follower")
	e2e.AssertProcessLogs(t, leaderEPC, "sent database snapshot")
}

func writeKVs(t *testing.T, etcdctl *e2e.EtcdctlV3, startIdx, endIdx int) {
	for i := startIdx; i < endIdx; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err := etcdctl.Put(t.Context(), key, value, config.PutOptions{})
		require.NoErrorf(t, err, "failed to put %q", key)
	}
}

func assertKVHash(t *testing.T, epc *e2e.EtcdProcessCluster) {
	clusterSize := len(epc.Procs)
	if clusterSize < 2 {
		return
	}
	t.Log("Verify all nodes have exact same revision and hash")
	assert.Eventually(t, func() bool {
		hashKvs, err := epc.Etcdctl().HashKV(t.Context(), 0)
		if err != nil {
			t.Logf("failed to get HashKV: %v", err)
			return false
		}
		if len(hashKvs) != clusterSize {
			t.Logf("expected %d hashkv responses, but got: %d", clusterSize, len(hashKvs))
			return false
		}
		for i := 1; i < clusterSize; i++ {
			if hashKvs[0].Header.Revision != hashKvs[i].Header.Revision {
				t.Logf("Got different revisions, [%d, %d]", hashKvs[0].Header.Revision, hashKvs[1].Header.Revision)
				return false
			}

			assert.Equal(t, hashKvs[0].Hash, hashKvs[i].Hash)
		}
		return true
	}, 10*time.Second, 500*time.Millisecond)
}
