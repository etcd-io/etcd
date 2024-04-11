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

//go:build !cluster_proxy

package e2e

import (
	"context"
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
			name:   "NoTLS",
			config: e2e.NewConfig(e2e.WithClusterSize(size)),
		},
		{
			name:   "PeerTLS",
			config: e2e.NewConfigPeerTLS().With(e2e.WithClusterSize(size)),
		},
		{
			name:   "PeerAutoTLS",
			config: e2e.NewConfigAutoTLS().With(e2e.WithClusterSize(size)),
		},
		{
			name:   "ClientTLS",
			config: e2e.NewConfigClientTLS().With(e2e.WithClusterSize(size)),
		},
		{
			name:   "ClientAutoTLS",
			config: e2e.NewConfigClientAutoTLS().With(e2e.WithClusterSize(size)),
		},
	}

	if fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		tcs = append(tcs,
			clusterTestCase{
				name:   "LastVersion",
				config: e2e.NewConfig(e2e.WithClusterSize(size), e2e.WithVersion(e2e.LastVersion)),
			}, clusterTestCase{
				name:   "LastVersionPeerTLS",
				config: e2e.NewConfigPeerTLS().With(e2e.WithClusterSize(size), e2e.WithVersion(e2e.LastVersion)),
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
				clusterTestCase{
					name:   "MinorityLastVersionPeerTLS",
					config: e2e.NewConfigPeerTLS().With(e2e.WithClusterSize(size), e2e.WithVersion(e2e.MinorityLastVersion)),
				}, clusterTestCase{
					name:   "QuorumLastVersionPeerTLS",
					config: e2e.NewConfigPeerTLS().With(e2e.WithClusterSize(size), e2e.WithVersion(e2e.QuorumLastVersion)),
				},
			)
		}
	}
	return tcs
}

// TestSnapshotByAddingMember tests the mix version send snapshots by adding member
func TestSnapshotByAddingMember(t *testing.T) {
	for _, tc := range clusterTestCases(1) {
		t.Run(tc.name+"-adding-new-member-of-current-version", func(t *testing.T) {
			snapshotTestByAddingMember(t, tc.config, e2e.CurrentVersion)
		})
		// etcd doesn't support adding a new member of old version into
		// a cluster with higher version. For example, etcd cluster
		// version is 3.6.x, then a new member of 3.5.x can't join the
		// cluster. Please refer to link below,
		// https://github.com/etcd-io/etcd/blob/3e903d0b12e399519a4013c52d4635ec8bdd6863/server/etcdserver/cluster_util.go#L222-L230
		// TODO: Uncomment the following tests when support is added.
		// t.Run(tc.name+"-adding-new-member-of-current-version", func(t *testing.T) {
		// 	snapshotTestByAddingMember(t, tc.config, e2e.LastVersion)
		// })
	}
}

func snapshotTestByAddingMember(t *testing.T, clusterConfig *e2e.EtcdProcessClusterConfig, newInstanceVersion e2e.ClusterVersion) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	t.Logf("Create an etcd cluster with %d member\n", clusterConfig.ClusterSize)
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithConfig(clusterConfig),
		e2e.WithSnapshotCount(10),
	)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		derr := epc.Close()
		require.NoError(t, derr, "failed to close etcd cluster: %v", derr)
	}()

	t.Log("Writing 20 keys to the cluster (more than SnapshotCount entries to trigger at least a snapshot)")
	writeKVs(t, epc.Etcdctl(), 0, 20)

	t.Log("Start a new etcd instance, which will receive a snapshot from the leader.")
	newCfg := *epc.Cfg
	newCfg.Version = newInstanceVersion
	newCfg.ServerConfig.SnapshotCatchUpEntries = 10
	t.Log("Starting a new etcd instance")
	_, err = epc.StartNewProc(context.TODO(), &newCfg, t, false /* addAsLearner */)
	require.NoError(t, err, "failed to start the new etcd instance: %v", err)
	defer epc.CloseProc(context.TODO(), nil)

	assertKVHash(t, 2, epc)
}

func TestSnapshotByRestartingMember(t *testing.T) {
	restartNodeIndex := 2
	for _, tc := range clusterTestCases(3) {
		t.Run(tc.name, func(t *testing.T) {
			snapshotTestByRestartingMember(t, tc.config, restartNodeIndex)
		})
	}
}

func snapshotTestByRestartingMember(t *testing.T, clusterConfig *e2e.EtcdProcessClusterConfig, restartNodeIndex int) {
	e2e.BeforeTest(t)

	t.Logf("Create an etcd cluster with %d member\n", clusterConfig.ClusterSize)
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithConfig(clusterConfig),
		e2e.WithSnapshotCount(10),
		e2e.WithSnapshotCatchUpEntries(10),
	)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		derr := epc.Close()
		require.NoError(t, derr, "failed to close etcd cluster: %v", derr)
	}()
	toRestartedMember := epc.Procs[restartNodeIndex]

	t.Log("Stop and restart the selected member")
	err = toRestartedMember.Stop()
	require.NoError(t, err)

	t.Log("Writing 20 keys to the cluster (more than SnapshotCount entries to trigger at least a snapshot.)")
	writeKVs(t, epc.Etcdctl(), 0, 20)

	t.Log("Verify logs to check leader has saved snapshot")
	leaderEPC := epc.Procs[epc.WaitLeader(t)]
	e2e.AssertProcessLogs(t, leaderEPC, "saved snapshot")

	t.Log("Restart the selected member")
	err = toRestartedMember.Restart(context.TODO())
	require.NoError(t, err)

	assertKVHash(t, clusterConfig.ClusterSize, epc)

	// assert process logs to check snapshot be sent
	leaderEPC = epc.Procs[epc.WaitLeader(t)]
	if leaderEPC.Config().ExecPath == e2e.BinPath.Etcd {
		t.Log("Verify logs to check snapshot be sent from leader to follower")
		e2e.AssertProcessLogs(t, leaderEPC, "sent database snapshot")
	}
}

func TestSnapshotByMockingPartition(t *testing.T) {
	mockPartitionNodeIndex := 2
	for _, tc := range clusterTestCases(3) {
		if !tc.config.IsPeerTLS {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			snapshotTestByMockingPartition(t, tc.config, mockPartitionNodeIndex)
		})
	}
}

func snapshotTestByMockingPartition(t *testing.T, clusterConfig *e2e.EtcdProcessClusterConfig, mockPartitionNodeIndex int) {
	e2e.BeforeTest(t)

	t.Logf("Create an etcd cluster with %d member\n", clusterConfig.ClusterSize)
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithConfig(clusterConfig),
		e2e.WithSnapshotCount(10),
		e2e.WithSnapshotCatchUpEntries(10),
		e2e.WithPeerProxy(true),
	)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		require.NoError(t, epc.Close(), "failed to close etcd cluster")
	}()

	leaderId := epc.WaitLeader(t)
	partitionedMember := epc.Procs[mockPartitionNodeIndex]
	if leaderId != mockPartitionNodeIndex {
		partitionedMemberId, err := epc.MemberId(mockPartitionNodeIndex)
		require.NoError(t, err)
		// If the partitioned member is not the original leader, Blackhole would not block all its communication with other members.
		t.Logf("Move leader to Proc[%d]: %d\n", mockPartitionNodeIndex, partitionedMemberId)
		require.NoError(t, epc.Etcdctl().MoveLeader(context.TODO(), partitionedMemberId))
		epc.WaitLeader(t)
	}
	// Mock partition
	proxy := partitionedMember.PeerProxy()
	t.Logf("Blackholing traffic from and to member %q", partitionedMember.Config().Name)
	proxy.BlackholeTx()
	proxy.BlackholeRx()
	time.Sleep(2 * time.Second)

	t.Logf("Wait for new leader election with remaining members")
	leaderEPC := epc.Procs[waitLeader(t, epc, mockPartitionNodeIndex)]
	t.Log("Writing 20 keys to the cluster (more than SnapshotCount entries to trigger at least a snapshot.)")
	writeKVs(t, leaderEPC.Etcdctl(), 0, 20)
	e2e.AssertProcessLogs(t, leaderEPC, "saved snapshot")
	assertRevision(t, leaderEPC, 21)
	assertRevision(t, partitionedMember, 1)

	// Wait for some time to restore the network
	time.Sleep(1 * time.Second)
	t.Logf("Unblackholing traffic from and to member %q", partitionedMember.Config().Name)
	proxy.UnblackholeTx()
	proxy.UnblackholeRx()

	assertKVHash(t, clusterConfig.ClusterSize, epc)

	// assert process logs to check snapshot be sent
	leaderEPC = epc.Procs[epc.WaitLeader(t)]
	if leaderEPC.Config().ExecPath == e2e.BinPath.Etcd {
		t.Log("Verify logs to check snapshot be sent from leader to follower")
		e2e.AssertProcessLogs(t, leaderEPC, "sent database snapshot")
	}
}

func writeKVs(t *testing.T, etcdctl *e2e.EtcdctlV3, startIdx, endIdx int) {
	for i := startIdx; i < endIdx; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err := etcdctl.Put(context.TODO(), key, value, config.PutOptions{})
		require.NoError(t, err, "failed to put %q, error: %v", key, err)
	}
}

func assertKVHash(t *testing.T, clusterSize int, epc *e2e.EtcdProcessCluster) {
	if clusterSize < 2 {
		return
	}
	t.Log("Verify all nodes have exact same revision and hash")
	assert.Eventually(t, func() bool {
		hashKvs, err := epc.Etcdctl().HashKV(context.TODO(), 0)
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

func waitLeader(t testing.TB, epc *e2e.EtcdProcessCluster, excludeNode int) int {
	var membs []e2e.EtcdProcess
	for i := 0; i < len(epc.Procs); i++ {
		if i == excludeNode {
			continue
		}
		membs = append(membs, epc.Procs[i])
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return epc.WaitMembersForLeader(ctx, t, membs)
}

func assertRevision(t testing.TB, member e2e.EtcdProcess, expectedRevision int64) {
	responses, err := member.Etcdctl().Status(context.TODO())
	require.NoError(t, err)
	assert.Equal(t, expectedRevision, responses[0].Header.Revision, "revision mismatch")
}
