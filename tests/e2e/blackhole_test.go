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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestBlackholeByMockingPartitionLeader(t *testing.T) {
	blackholeTestByMockingPartition(t, 3, true)
}

func TestBlackholeByMockingPartitionFollower(t *testing.T) {
	blackholeTestByMockingPartition(t, 3, false)
}

func blackholeTestByMockingPartition(t *testing.T, clusterSize int, partitionLeader bool) {
	e2e.BeforeTest(t)

	t.Logf("Create an etcd cluster with %d member\n", clusterSize)
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithClusterSize(clusterSize),
		e2e.WithSnapshotCount(10),
		e2e.WithSnapshotCatchUpEntries(10),
		e2e.WithIsPeerTLS(true),
		e2e.WithSingleHTTPProxy(true),
	)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		require.NoError(t, epc.Close(), "failed to close etcd cluster")
	}()

	leaderID := epc.WaitLeader(t)
	mockPartitionNodeIndex := leaderID
	if !partitionLeader {
		mockPartitionNodeIndex = (leaderID + 1) % (clusterSize)
	}
	partitionedMember := epc.Procs[mockPartitionNodeIndex]
	// Mock partition
	t.Logf("Blackholing traffic from and to member %q", partitionedMember.Config().Name)
	epc.SingleHTTPProxyInstance.BlackholePeer(partitionedMember.Config().PeerURL)

	t.Logf("Wait 5s for any open connections to expire")
	time.Sleep(5 * time.Second)

	t.Logf("Wait for new leader election with remaining members")
	leaderEPC := epc.Procs[waitLeader(t, epc, mockPartitionNodeIndex)]
	t.Log("Writing 20 keys to the cluster (more than SnapshotCount entries to trigger at least a snapshot.)")
	writeKVs(t, leaderEPC.Etcdctl(), 0, 20)
	e2e.AssertProcessLogs(t, leaderEPC, "saved snapshot")

	t.Log("Verifying the partitionedMember is missing new writes")
	assertRevision(t, leaderEPC, 21)
	assertRevision(t, partitionedMember, 1)

	// Wait for some time to restore the network
	time.Sleep(1 * time.Second)
	t.Logf("Unblackholing traffic from and to member %q", partitionedMember.Config().Name)
	epc.SingleHTTPProxyInstance.UnblackholePeer(partitionedMember.Config().PeerURL)

	leaderEPC = epc.Procs[epc.WaitLeader(t)]
	time.Sleep(5 * time.Second)
	assertRevision(t, leaderEPC, 21)
	assertRevision(t, partitionedMember, 21)
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
