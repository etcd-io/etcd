// Copyright 2016 The etcd Authors
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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestReleaseUpgrade ensures that changes to master branch does not affect
// upgrade from latest etcd releases.
func TestReleaseUpgrade(t *testing.T) {
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithVersion(e2e.LastVersion),
		e2e.WithSnapshotCount(3),
		e2e.WithBasePeerScheme("unix"), // to avoid port conflict
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	cx := ctlCtx{
		t:           t,
		cfg:         *e2e.NewConfigNoTLS(),
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	var kvs []kv
	for i := 0; i < 5; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	for i := range kvs {
		if err = ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("#%d: ctlV3Put error (%v)", i, err)
		}
	}

	t.Log("Cluster of etcd in old version running")

	for i := range epc.Procs {
		t.Logf("Stopping node: %v", i)
		if err = epc.Procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		t.Logf("Stopped node: %v", i)
		epc.Procs[i].Config().ExecPath = e2e.BinPath.Etcd
		epc.Procs[i].Config().KeepDataDir = true

		t.Logf("Restarting node in the new version: %v", i)
		if err = epc.Procs[i].Restart(t.Context()); err != nil {
			t.Fatalf("error restarting etcd process (%v)", err)
		}

		t.Logf("Testing reads after node restarts: %v", i)
		for j := range kvs {
			require.NoErrorf(cx.t, ctlV3Get(cx, []string{kvs[j].key}, []kv{kvs[j]}...), "#%d-%d: ctlV3Get error", i, j)
		}
		t.Logf("Tested reads after node restarts: %v", i)
	}

	t.Log("Waiting for full upgrade...")
	// TODO: update after release candidate
	// expect upgraded cluster version
	// new cluster version needs more time to upgrade
	ver := version.Cluster(version.Version)
	for i := 0; i < 7; i++ {
		if err = e2e.CURLGet(epc, e2e.CURLReq{Endpoint: "/version", Expected: expect.ExpectedResponse{Value: `"etcdcluster":"` + ver}}); err != nil {
			t.Logf("#%d: %v is not ready yet (%v)", i, ver, err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if err != nil {
		t.Fatalf("cluster version is not upgraded (%v)", err)
	}
	t.Log("TestReleaseUpgrade businessLogic DONE")
}

func TestReleaseUpgradeWithRestart(t *testing.T) {
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	e2e.BeforeTest(t)

	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t,
		e2e.WithVersion(e2e.LastVersion),
		e2e.WithSnapshotCount(10),
		e2e.WithBasePeerScheme("unix"),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	cx := ctlCtx{
		t:           t,
		cfg:         *e2e.NewConfigNoTLS(),
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	for i := range kvs {
		require.NoErrorf(cx.t, ctlV3Put(cx, kvs[i].key, kvs[i].val, ""), "#%d: ctlV3Put error", i)
	}

	for i := range epc.Procs {
		require.NoErrorf(t, epc.Procs[i].Stop(), "#%d: error closing etcd process", i)
	}

	var wg sync.WaitGroup
	wg.Add(len(epc.Procs))
	for i := range epc.Procs {
		go func(i int) {
			epc.Procs[i].Config().ExecPath = e2e.BinPath.Etcd
			epc.Procs[i].Config().KeepDataDir = true
			assert.NoErrorf(t, epc.Procs[i].Restart(t.Context()), "error restarting etcd process")
			wg.Done()
		}(i)
	}
	wg.Wait()

	require.NoError(t, ctlV3Get(cx, []string{kvs[0].key}, []kv{kvs[0]}...))
}

func TestClusterUpgradeAfterPromotingMembers(t *testing.T) {
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	e2e.BeforeTest(t)

	currentVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.Etcd)
	require.NoErrorf(t, err, "failed to get version from binary")

	lastClusterVersion, err := e2e.GetVersionFromBinary(e2e.BinPath.EtcdLastRelease)
	require.NoErrorf(t, err, "failed to get version from last release binary")

	clusterSize := 3

	epc := createNewClusterByPromotingMembers(t, lastClusterVersion, clusterSize)
	defer func() {
		require.NoError(t, epc.Close())
	}()

	err = e2e.DowngradeUpgradeMembers(t, nil, epc, 3, false, lastClusterVersion, currentVersion)
	require.NoError(t, err)

	t.Logf("Checking all members' status after upgrading")
	ensureAllMembersAreVotingMembers(t, epc)

	t.Logf("Checking all members are ready to serve client requests")
	for i := 0; i < clusterSize; i++ {
		err = epc.Procs[i].Etcdctl().Put(context.Background(), "foo", "bar", config.PutOptions{})
		require.NoError(t, err)
	}
}

func createNewClusterByPromotingMembers(t *testing.T, clusterVersion *semver.Version, clusterSize int) *e2e.EtcdProcessCluster {
	require.GreaterOrEqualf(t, clusterSize, 1, "clusterSize >= 1")

	cv := e2e.CurrentVersion
	if clusterVersion.LessThan(*semver.New(version.Version)) {
		cv = e2e.LastVersion
	}
	ctx := context.Background()

	t.Logf("Creating new etcd cluster - version: %s, clusterSize: %v", clusterVersion.String(), clusterSize)

	t.Log("Creating first node")
	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithVersion(cv),
		e2e.WithClusterSize(1),
		e2e.WithSnapshotCount(10),
	)
	require.NoErrorf(t, err, "failed to start first etcd process")
	defer func() {
		if t.Failed() {
			epc.Close()
		}
	}()

	for i := 1; i < clusterSize; i++ {
		var nodeID uint64
		var aerr error

		// NOTE: New promoted member needs time to get connected.
		t.Logf("[%d] Adding new node as learner", i)
		testutils.ExecuteWithTimeout(t, 1*time.Minute, func() {
			for {
				nodeID, aerr = epc.StartNewProc(ctx, nil, t, true)
				if aerr != nil {
					if strings.Contains(aerr.Error(), "etcdserver: unhealthy cluster") {
						time.Sleep(1 * time.Second)
						continue
					}
				}
				break
			}
		})
		require.NoError(t, aerr)

		t.Logf("[%d] Promoting node(%d)", i, nodeID)
		etcdctl := epc.Procs[0].Etcdctl()
		_, err = etcdctl.MemberPromote(ctx, nodeID)
		require.NoError(t, err)
	}

	t.Log("Checking all members' status")
	ensureAllMembersAreVotingMembers(t, epc)

	t.Logf("Adding 10 key/value to trigger snapshot")
	for i := 0; i < 10; i++ {
		err = epc.Etcdctl().Put(ctx, "foo", "bar", config.PutOptions{})
		require.NoError(t, err)
	}
	return epc
}

func ensureAllMembersAreVotingMembers(t *testing.T, epc *e2e.EtcdProcessCluster) {
	resp, err := epc.Etcdctl().MemberList(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, resp.Members, len(epc.Procs))
	for _, m := range resp.Members {
		require.Falsef(t, m.IsLearner, "node(%x)", m.ID)
	}
}
