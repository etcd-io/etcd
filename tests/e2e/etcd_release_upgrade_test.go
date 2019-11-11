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
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/pkg/testutil"
	"go.etcd.io/etcd/version"
)

var (
	configUpgrade = etcdProcessClusterConfig{
		clusterSize:   3,
		initialToken:  "new",
		snapshotCount: 10,
		baseScheme:    "unix", // to avoid port conflict
	}
	lastReleasePath = "/etcd-last-release"
)

// TestReleaseUpgrade ensures that changes to master branch does not affect
// upgrade from latest etcd releases.
func TestReleaseUpgrade(t *testing.T) {
	defer testutil.AfterTest(t)
	t.Skip("remove skip after release new 3.4 binary")

	copiedCfg := configUpgrade
	epc := startCluster(t, &copiedCfg, lastReleasePath)

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         configNoTLS,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	// 3.0 boots as 2.3 then negotiates up to 3.0
	// so there's a window at boot time where it doesn't have V3rpcCapability enabled
	// poll /version until etcdcluster is >2.3.x before making v3 requests
	clusterVersionTest(cx, `"etcdcluster":"3.`)

	var kvs []kv
	for i := 0; i < 5; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("#%d: ctlV3Put error (%v)", i, err)
		}
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + "/etcd"
		epc.procs[i].Config().keepDataDir = true

		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)", err)
		}

		for j := range kvs {
			if err := ctlV3Get(cx, []string{kvs[j].key}, []kv{kvs[j]}...); err != nil {
				cx.t.Fatalf("#%d-%d: ctlV3Get error (%v)", i, j, err)
			}
		}
	}

	// TODO: update after release candidate
	// expect upgraded cluster version
	// new cluster version needs more time to upgrade
	ver := version.Cluster(version.Version)
	clusterVersionTest(cx, `"etcdcluster":"`+ver)
}

// TestReleaseUpgradeWithRestart ensures that upgraded cluster can restart
// with zero consistent index in the backend.
func TestReleaseUpgradeWithRestart(t *testing.T) {
	defer testutil.AfterTest(t)
	t.Skip("remove skip after release new 3.4 binary")

	copiedCfg := configUpgrade
	epc := startCluster(t, &copiedCfg, lastReleasePath)

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         configNoTLS,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("#%d: ctlV3Put error (%v)", i, err)
		}
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(epc.procs))
	for i := range epc.procs {
		go func(i int) {
			epc.procs[i].Config().execPath = binDir + "/etcd"
			epc.procs[i].Config().keepDataDir = true
			if err := epc.procs[i].Restart(); err != nil {
				t.Errorf("error restarting etcd process (%v)", err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	if err := ctlV3Get(cx, []string{kvs[0].key}, []kv{kvs[0]}...); err != nil {
		t.Fatal(err)
	}
}

// TestReleaseUpgradeFromSnapshot ensures that cluster can be upgraded from
// the snapshot of old version.
func TestReleaseUpgradeFromSnapshot(t *testing.T) {
	defer testutil.AfterTest(t)
	t.Skip("remove skip after release new 3.4 binary")

	copiedCfg := configUpgrade
	epc := startCluster(t, &copiedCfg, lastReleasePath)

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         configNoTLS,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("#%d: ctlV3Put error (%v)", i, err)
		}
	}

	// etcdctl save snapshot
	fpath := "upgrade.snapshot"
	defer os.RemoveAll(fpath)
	dialTimeout := 7 * time.Second
	cmdArgs := []string{
		ctlBinPath, "--endpoints", epc.EndpointsV3()[0],
		"--dial-timeout", dialTimeout.String(),
		"snapshot", "save", fpath}

	if err := spawnWithExpect(cmdArgs, fmt.Sprintf("Snapshot saved at %s", fpath)); err != nil {
		t.Fatal(err)
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
	}

	// etcdctl restore the snapshot
	for i := range epc.procs {
		cmdArgs := []string{
			ctlBinPath, "snapshot", "restore", fpath,
			"--name", epc.procs[i].Config().name,
			"--initial-cluster", epc.procs[i].Config().initialCluster,
			"--initial-cluster-token", epc.procs[i].Config().initialToken,
			"--initial-advertise-peer-urls", epc.procs[i].Config().purl.String(),
			"--data-dir", fmt.Sprintf("test%d.etcd", i)}
		err := spawnWithExpect(cmdArgs, "added member")
		if err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(epc.procs))
	for i := range epc.procs {
		go func(i int) {
			epc.procs[i].Config().execPath = binDir + "/etcd"
			epc.procs[i].Config().dataDirPath = fmt.Sprintf("test%d.etcd", i)
			if err := epc.procs[i].Restart(); err != nil {
				t.Errorf("error restarting etcd process (%v)", err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	if err := ctlV3Get(cx, []string{kvs[0].key}, []kv{kvs[0]}...); err != nil {
		t.Fatal(err)
	}
}

// TestReleaseUpgradeHealthStatus ensures all nodes in the cluster are healthy
// during the process of upgrade.
func TestReleaseUpgradeHealthStatus(t *testing.T) {
	defer testutil.AfterTest(t)
	t.Skip("remove skip after release new 3.4 binary")

	copiedCfg := configUpgrade
	epc := startCluster(t, &copiedCfg, lastReleasePath)

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         configNoTLS,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + "/etcd"
		epc.procs[i].Config().keepDataDir = true

		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)", err)
		}
		if err := ctlV3EndpointHealth(cx); err != nil {
			t.Fatalf("error check endpoints healthy (%v)", err)
		}
	}
}

func startCluster(t *testing.T, cfg *etcdProcessClusterConfig, path string) *etcdProcessCluster {
	binary := binDir + path
	mustFileExist(t, binary)
	cfg.execPath = binary
	epc, err := newEtcdProcessCluster(cfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	return epc
}

func mustFileExist(t *testing.T, path string) {
	if !fileutil.Exist(path) {
		t.Skipf("%q does not exist", path)
	}
}
