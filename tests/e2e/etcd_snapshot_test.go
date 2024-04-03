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
		tcs = append(tcs, clusterTestCase{
			name:   "LastVersion",
			config: e2e.NewConfig(e2e.WithClusterSize(size), e2e.WithVersion(e2e.LastVersion)),
		})
		if size > 2 {
			tcs = append(tcs, clusterTestCase{
				name:   "MinorityLastVersion",
				config: e2e.NewConfig(e2e.WithClusterSize(size), e2e.WithVersion(e2e.MinorityLastVersion)),
			}, clusterTestCase{
				name:   "QuorumLastVersion",
				config: e2e.NewConfig(e2e.WithClusterSize(size), e2e.WithVersion(e2e.QuorumLastVersion)),
			})
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
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err = epc.Etcdctl().Put(context.TODO(), key, value, config.PutOptions{})
		require.NoError(t, err, "failed to put %q, error: %v", key, err)
	}

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
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		err = epc.Etcdctl().Put(context.TODO(), key, value, config.PutOptions{})
		require.NoError(t, err, "failed to put %q, error: %v", key, err)
	}

	t.Log("Verify logs to check leader has saved snapshot")
	leaderEPC := epc.Procs[epc.WaitLeader(t)]
	e2e.AssertProcessLogs(t, leaderEPC, "saved snapshot")

	t.Log("Restart the selected member")
	err = toRestartedMember.Restart(context.TODO())
	require.NoError(t, err)

	assertKVHash(t, clusterConfig.ClusterSize, epc)

	// assert process logs to check snapshot be sent
	if clusterConfig.Version == e2e.CurrentVersion || clusterConfig.Version == e2e.MinorityLastVersion {
		t.Log("Verify logs to check snapshot be sent from leader to follower")
		leaderEPC = epc.Procs[epc.WaitLeader(t)]
		e2e.AssertProcessLogs(t, leaderEPC, "sent database snapshot")
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
