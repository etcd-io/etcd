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

// TestMixVersionsSnapshotByAddingMember tests the mix version send snapshots by adding member
func TestMixVersionsSnapshotByAddingMember(t *testing.T) {
	cases := []struct {
		name               string
		clusterVersion     e2e.ClusterVersion
		newInstanceVersion e2e.ClusterVersion
	}{
		// etcd doesn't support adding a new member of old version into
		// a cluster with higher version. For example, etcd cluster
		// version is 3.6.x, then a new member of 3.5.x can't join the
		// cluster. Please refer to link below,
		// https://github.com/etcd-io/etcd/blob/3e903d0b12e399519a4013c52d4635ec8bdd6863/server/etcdserver/cluster_util.go#L222-L230
		/*{
			name:              "etcd instance with last version receives snapshot from the leader with current version",
			clusterVersion:    e2e.CurrentVersion,
			newInstaceVersion: e2e.LastVersion,
		},*/
		{
			name:               "etcd instance with current version receives snapshot from the leader with last version",
			clusterVersion:     e2e.LastVersion,
			newInstanceVersion: e2e.CurrentVersion,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mixVersionsSnapshotTestByAddingMember(t, tc.clusterVersion, tc.newInstanceVersion)
		})
	}
}

func mixVersionsSnapshotTestByAddingMember(t *testing.T, clusterVersion, newInstanceVersion e2e.ClusterVersion) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	// Create an etcd cluster with 1 member
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithClusterSize(1),
		e2e.WithSnapshotCount(10),
		e2e.WithVersion(clusterVersion),
	)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		err := epc.Close()
		require.NoError(t, err, "failed to close etcd cluster: %v", err)
	}()

	// Write more than SnapshotCount entries to trigger at least a snapshot.
	t.Log("Writing 20 keys to the cluster")
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err := epc.Etcdctl().Put(context.TODO(), key, value, config.PutOptions{})
		require.NoError(t, err, "failed to put %q, error: %v", key, err)
	}

	// start a new etcd instance, which will receive a snapshot from the leader.
	newCfg := *epc.Cfg
	newCfg.Version = newInstanceVersion
	newCfg.SnapshotCatchUpEntries = 10
	t.Log("Starting a new etcd instance")
	_, err = epc.StartNewProc(context.TODO(), &newCfg, t, false /* addAsLearner */)
	require.NoError(t, err, "failed to start the new etcd instance: %v", err)
	defer epc.CloseProc(context.TODO(), nil)

	// verify all nodes have exact same revision and hash
	t.Log("Verify all nodes have exact same revision and hash")
	assert.Eventually(t, func() bool {
		hashKvs, err := epc.Etcdctl().HashKV(context.TODO(), 0)
		if err != nil {
			t.Logf("failed to get HashKV: %v", err)
			return false
		}
		if len(hashKvs) != 2 {
			t.Logf("expected 2 hashkv responses, but got: %d", len(hashKvs))
			return false
		}

		if hashKvs[0].Header.Revision != hashKvs[1].Header.Revision {
			t.Logf("Got different revisions, [%d, %d]", hashKvs[0].Header.Revision, hashKvs[1].Header.Revision)
			return false
		}

		assert.Equal(t, hashKvs[0].Hash, hashKvs[1].Hash)

		return true
	}, 10*time.Second, 500*time.Millisecond)
}

func TestMixVersionsSnapshotByMockingPartition(t *testing.T) {
	cases := []struct {
		name                   string
		clusterVersion         e2e.ClusterVersion
		mockPartitionNodeIndex int
	}{
		{
			name:                   "etcd instance with last version receives snapshot from the leader with current version",
			clusterVersion:         e2e.MinorityLastVersion,
			mockPartitionNodeIndex: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mixVersionsSnapshotTestByMockPartition(t, tc.clusterVersion, tc.mockPartitionNodeIndex)
		})
	}
}

func mixVersionsSnapshotTestByMockPartition(t *testing.T, clusterVersion e2e.ClusterVersion, mockPartitionNodeIndex int) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	// Create an etcd cluster with 3 member of MinorityLastVersion
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithClusterSize(3),
		e2e.WithSnapshotCount(10),
		e2e.WithVersion(clusterVersion),
		e2e.WithSnapshotCatchUpEntries(10),
	)
	require.NoError(t, err, "failed to start etcd cluster: %v", err)
	defer func() {
		err := epc.Close()
		require.NoError(t, err, "failed to close etcd cluster: %v", err)
	}()
	toPartitionedMember := epc.Procs[mockPartitionNodeIndex]

	// Stop and restart the partitioned member
	err = toPartitionedMember.Stop()
	require.NoError(t, err)

	// Write more than SnapshotCount entries to trigger at least a snapshot.
	t.Log("Writing 20 keys to the cluster")
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err := epc.Etcdctl().Put(context.TODO(), key, value, config.PutOptions{})
		require.NoError(t, err, "failed to put %q, error: %v", key, err)
	}

	t.Log("Verify logs to check leader has saved snapshot")
	leaderEPC := epc.Procs[epc.WaitLeader(t)]
	e2e.AssertProcessLogs(t, leaderEPC, "saved snapshot")

	// Restart the partitioned member
	err = toPartitionedMember.Restart(context.TODO())
	require.NoError(t, err)

	// verify all nodes have exact same revision and hash
	t.Log("Verify all nodes have exact same revision and hash")
	assert.Eventually(t, func() bool {
		hashKvs, err := epc.Etcdctl().HashKV(context.TODO(), 0)
		if err != nil {
			t.Logf("failed to get HashKV: %v", err)
			return false
		}
		if len(hashKvs) != 3 {
			t.Logf("expected 3 hashkv responses, but got: %d", len(hashKvs))
			return false
		}

		if hashKvs[0].Header.Revision != hashKvs[1].Header.Revision {
			t.Logf("Got different revisions, [%d, %d]", hashKvs[0].Header.Revision, hashKvs[1].Header.Revision)
			return false
		}
		if hashKvs[1].Header.Revision != hashKvs[2].Header.Revision {
			t.Logf("Got different revisions, [%d, %d]", hashKvs[1].Header.Revision, hashKvs[2].Header.Revision)
			return false
		}

		assert.Equal(t, hashKvs[0].Hash, hashKvs[1].Hash)
		assert.Equal(t, hashKvs[1].Hash, hashKvs[2].Hash)

		return true
	}, 10*time.Second, 500*time.Millisecond)

	// assert process logs to check snapshot be sent
	t.Log("Verify logs to check snapshot be sent from leader to follower")
	leaderEPC = epc.Procs[epc.WaitLeader(t)]
	e2e.AssertProcessLogs(t, leaderEPC, "sent database snapshot")
}
