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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"

	"go.etcd.io/etcd/pkg/testutil"
)

// TestIssue20271 reproduces the issue: https://github.com/etcd-io/etcd/issues/20271.
func TestIssue20271(t *testing.T) {
	defer testutil.AfterTest(t)

	ctx := t.Context()
	snapCount := 10

	epc, err := newEtcdProcessCluster(t, &etcdProcessClusterConfig{
		snapshotCount:       snapCount,
		clusterSize:         3,
		keepDataDir:         true,
		goFailEnabled:       true,
		goFailClientTimeout: 40 * time.Second,
		debug:               true,
	})
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, epc.Stop()) })

	t.Log("Step 0: Ensure the leader is not the third member")
	if leaderIdx := epc.WaitLeader(t); leaderIdx == 2 {
		var transferee uint64
		firstCli := newClient(t, epc.procs[0].EndpointsGRPC(), clientNonTLS, false)
		statusResp, err := firstCli.Status(ctx, epc.procs[0].EndpointsV3()[0])
		require.NoError(t, err)
		transferee = statusResp.Header.GetMemberId()

		cli := newClient(t, epc.procs[leaderIdx].EndpointsGRPC(), clientNonTLS, false)
		_, err = cli.MoveLeader(ctx, transferee)
		require.NoError(t, err)
	}

	t.Log("Step 1: Write some data to the cluster")
	for i := 0; i < snapCount*5; i++ {
		require.NoError(t, epc.procs[0].Etcdctl(clientNonTLS, false, false).
			Put(fmt.Sprintf("foo%d", i), strings.Repeat("Oops", 1024)))
	}

	t.Log(`Step 2: Config the third member to sleep 15s after OpenSnapshotBackend and use SIGSTOP to pause it.`)
	require.NoError(t, epc.procs[2].Failpoints().SetupHTTP(ctx, "applyAfterOpenSnapshot", `sleep("15s")`))
	epc.procs[2].Pause()

	t.Log("Step 3: Write some key values to trigger new snapshot on the first two members")
	for i := 0; i < snapCount+20; i++ {
		err = epc.procs[0].Etcdctl(clientNonTLS, false, false).Put(fmt.Sprintf("foo%d", i), strings.Repeat("Awoo", 10))
		require.NoError(t, err)
	}

	t.Log("Step 4: Restarting the first two members to re-connect to the paused member, so the inflight messages will be dropped. This will trigger new leader to send snapshot file to the third member.")
	for _, proc := range epc.procs[:2] {
		require.NoError(t, proc.Restart())
	}
	epc.procs[2].Resume()

	t.Log(`Step 5: After opening snapshot file from new leader, invoke defragment\n
to override boltdb file. So, for the following changes, the third member will commit them into deleted boltdb file.`)
	epc.procs[2].Logs().Expect("applySnapshot: opened snapshot backend")

	err = epc.procs[2].Etcdctl(clientNonTLS, false, false).Defragment(20 * time.Second)
	require.NoError(t, err)
	epc.procs[2].Logs().Expect("applied snapshot")
	require.NoError(t, epc.procs[2].Failpoints().DeactivateHTTP(ctx, "applyAfterOpenSnapshot"))

	t.Log("Step 6: Write some data to the cluster")
	for i := 0; i < snapCount/2; i++ {
		err = epc.procs[0].Etcdctl(clientNonTLS, false, false).Put(fmt.Sprintf("foo%d", i), strings.Repeat("Oops", 1))
		require.NoError(t, err)
	}

	t.Log("Step 7: Restart the third member. It recovers from the new boltdb file. Therefore, data writen in Step 6 is lost.")
	require.NoError(t, epc.procs[2].Restart())

	t.Log("Step 8: Check hashkv of each member")
	assertKVHash(t, epc)
}

func assertKVHash(t *testing.T, epc *etcdProcessCluster) {
	clusterSize := len(epc.procs)
	if clusterSize < 2 {
		return
	}
	t.Log("Verify all nodes have exact same revision and hash")
	assert.Eventually(t, func() bool {
		var hashKvs = make([]*clientv3.HashKVResponse, 0, len(epc.procs))
		for _, proc := range epc.procs {
			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   proc.EndpointsV3(),
				DialTimeout: 3 * time.Second,
			})
			assert.NoError(t, err)
			resp, err := cli.HashKV(t.Context(), proc.EndpointsV3()[0], 0)
			assert.NoError(t, err)
			hashKvs = append(hashKvs, resp)
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
