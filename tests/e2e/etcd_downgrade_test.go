// Copyright 2019 The etcd Authors
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

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/pkg/testutil"
	"go.etcd.io/etcd/version"
)

var (
	configDowngrade = etcdProcessClusterConfig{
		clusterSize:   3,
		initialToken:  "new",
		snapshotCount: 10,
		baseScheme:    "unix", // to avoid port conflict
	}
)

// TestRollingDowngrade ensures that the servers in cluster
// can be downgraded one by one.
func TestRollingDowngrade(t *testing.T) {
	defer testutil.AfterTest(t)

	cv := semver.Must(semver.NewVersion(version.Version))
	tv := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	tvBinary := fmt.Sprintf("/downgrade%d%d", tv.Major, tv.Minor)
	mustFileExist(t, binDir+tvBinary)

	copiedCfg := configDowngrade
	// start a new cluster with current version
	epc := startCluster(t, &copiedCfg, "/etcd")

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         copiedCfg,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	clusterVersionTest(cx, `"etcdcluster":"`+version.Cluster(cv.String()))
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	putKVPairs(cx, kvs)

	if err := testCtlDowngradeEnable(cx, tv.String(),
		"The cluster is available to downgrade"); err != nil {
		cx.t.Fatalf("testCtlDowngradeEnable error (%v)", err)
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "true")
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + tvBinary
		epc.procs[i].Config().keepDataDir = true

		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)(%d)", err, i)
		}
		// check cluster health after restart
		testHealth(cx)
		testKVPairs(cx, kvs)
		// cluster version should be updated once there is a downgraded server
		clusterVersionTest(cx, `"etcdcluster":"`+tv.String())
	}
}

// TestRestartDowngrade ensures that the servers in cluster can be stopped
// together and then restarted with lower version.
func TestRestartDowngrade(t *testing.T) {
	defer testutil.AfterTest(t)

	cv := semver.Must(semver.NewVersion(version.Version))
	tv := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	tvBinary := fmt.Sprintf("/downgrade%d%d", tv.Major, tv.Minor)
	mustFileExist(t, binDir+tvBinary)

	copiedCfg := configDowngrade
	// start a new cluster with current version
	epc := startCluster(t, &copiedCfg, "/etcd")

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         copiedCfg,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	clusterVersionTest(cx, `"etcdcluster":"`+version.Cluster(cv.String()))
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	putKVPairs(cx, kvs)

	if err := testCtlDowngradeEnable(cx, tv.String(),
		"The cluster is available to downgrade"); err != nil {
		cx.t.Fatalf("testCtlDowngradeEnable error (%v)", err)
	}
	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "true")
	}
	//	stop cluster
	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
	}
	// restart cluster
	var wg sync.WaitGroup
	wg.Add(len(epc.procs))
	for i := range epc.procs {
		go func(i int) {
			epc.procs[i].Config().execPath = binDir + tvBinary
			epc.procs[i].Config().keepDataDir = true

			if err := epc.procs[i].Restart(); err != nil {
				t.Fatalf("error restarting etcd process (%v)(%d)", err, i)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	testHealth(cx)
	testKVPairs(cx, kvs)
	clusterVersionTest(cx, `"etcdcluster":"`+tv.String())
}

// TestDowngradeThenUpgrade ensures that the cluster can be upgraded after
// downgrade.
func TestDowngradeThenUpgrade(t *testing.T) {
	defer testutil.AfterTest(t)

	cv := semver.Must(semver.NewVersion(version.Version))
	tv := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	tvBinary := fmt.Sprintf("/downgrade%d%d", tv.Major, tv.Minor)
	mustFileExist(t, binDir+tvBinary)

	copiedCfg := configDowngrade
	// start a new cluster with current version
	epc := startCluster(t, &copiedCfg, "/etcd")

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         copiedCfg,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	clusterVersionTest(cx, `"etcdcluster":"`+version.Cluster(cv.String()))
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	putKVPairs(cx, kvs)

	if err := testCtlDowngradeEnable(cx, tv.String(),
		"The cluster is available to downgrade"); err != nil {
		cx.t.Fatalf("testCtlDowngradeEnable error (%v)", err)
	}
	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "true")
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + tvBinary
		epc.procs[i].Config().keepDataDir = true

		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)(%d)", err, i)
		}

		testHealth(cx)
		clusterVersionTest(cx, `"etcdcluster":"`+tv.String())
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "false")
	}

	// update kv pairs
	for i := 0; i < 50; i++ {
		kvs[i] = kv{key: fmt.Sprintf("foo%d", i), val: "bar_new"}
	}
	putKVPairs(cx, kvs)
	// upgrade the cluster
	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + "/etcd"
		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)", err)
		}
		testHealth(cx)
	}

	testKVPairs(cx, kvs)
	ver := version.Cluster(version.Version)
	clusterVersionTest(cx, `"etcdcluster":"`+ver)
}

// TestDowngradeThenRestart ensures that the server can be restarted
// after downgrade.
func TestDowngradeThenRestart(t *testing.T) {
	defer testutil.AfterTest(t)

	cv := semver.Must(semver.NewVersion(version.Version))
	tv := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	tvBinary := fmt.Sprintf("/downgrade%d%d", tv.Major, tv.Minor)
	mustFileExist(t, binDir+tvBinary)

	copiedCfg := configDowngrade
	// start a new cluster with current version
	epc := startCluster(t, &copiedCfg, "/etcd")

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
	clusterVersionTest(cx, `"etcdcluster":"`+version.Cluster(cv.String()))
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	putKVPairs(cx, kvs)

	if err := testCtlDowngradeEnable(cx, tv.String(),
		"The cluster is available to downgrade"); err != nil {
		cx.t.Fatalf("testCtlDowngradeEnable error (%v)", err)
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "true")
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + tvBinary
		epc.procs[i].Config().keepDataDir = true

		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)(%d)", err, i)
		}

		// check cluster health after restart
		if err := ctlV3EndpointHealth(cx); err != nil {
			t.Fatalf("error check endpoints healthy (%v)", err)
		}
		// cluster version should be updated once there is a downgraded server
		clusterVersionTest(cx, `"etcdcluster":"`+tv.String())
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "false")
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}

		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)(%d)", err, i)
		}

		// check cluster health after restart
		if err := ctlV3EndpointHealth(cx); err != nil {
			t.Fatalf("error check endpoints healthy (%v)", err)
		}
		testKVPairs(cx, kvs)
	}
}

// TestDowngradeThenDowngrade ensures that the cluster can be downgraded
// after downgrade.
func TestDowngradeThenDowngrade(t *testing.T) {
	defer testutil.AfterTest(t)

	cv := semver.Must(semver.NewVersion(version.Version))
	tv := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	lowerTv := semver.Version{Major: cv.Major, Minor: cv.Minor - 2}
	tvBinary := fmt.Sprintf("/downgrade%d%d", tv.Major, tv.Minor)
	lowerTvBinary := fmt.Sprintf("/downgrade%d%d", lowerTv.Major, lowerTv.Minor)
	mustFileExist(t, binDir+tvBinary)
	mustFileExist(t, binDir+lowerTvBinary)

	copiedCfg := configDowngrade
	// start a new cluster with current version
	epc := startCluster(t, &copiedCfg, "/etcd")

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         copiedCfg,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	clusterVersionTest(cx, `"etcdcluster":"`+version.Cluster(cv.String()))
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	putKVPairs(cx, kvs)

	if err := testCtlDowngradeEnable(cx, tv.String(),
		"The cluster is available to downgrade"); err != nil {
		cx.t.Fatalf("testCtlDowngradeEnable error (%v)", err)
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "true")
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + tvBinary
		epc.procs[i].Config().keepDataDir = true

		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)(%d)", err, i)
		}

		testHealth(cx)
		clusterVersionTest(cx, `"etcdcluster":"`+tv.String())
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "false")
	}
	// downgrade to lower version
	if err := testCtlDowngradeEnable(cx, lowerTv.String(),
		"The cluster is available to downgrade"); err != nil {
		cx.t.Fatalf("testCtlDowngradeEnable error (%v)", err)
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "true")
	}

	for i := range epc.procs {
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		epc.procs[i].Config().execPath = binDir + lowerTvBinary
		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)(%d)", err, i)
		}

		testHealth(cx)
		clusterVersionTest(cx, `"etcdcluster":"`+lowerTv.String())
		testKVPairs(cx, kvs)
	}
}

// TestReleaseUpgradeFromSnapshot ensures that cluster can be downgraded from
// the snapshot of previous version.
func TestDowngradeFromSnapshot(t *testing.T) {
	defer testutil.AfterTest(t)
	cv := semver.Must(semver.NewVersion(version.Version))
	tv := semver.Version{Major: cv.Major, Minor: cv.Minor - 1}
	tvBinary := fmt.Sprintf("/downgrade%d%d", tv.Major, tv.Minor)
	mustFileExist(t, binDir+tvBinary)

	copiedCfg := configUpgrade
	epc := startCluster(t, &copiedCfg, "/etcd")

	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         copiedCfg,
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	var kvs []kv
	for i := 0; i < 50; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	putKVPairs(cx, kvs)

	if err := testCtlDowngradeEnable(cx, tv.String(),
		"The cluster is available to downgrade"); err != nil {
		cx.t.Fatalf("testCtlDowngradeEnable error (%v)", err)
	}

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "true")
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
			epc.procs[i].Config().execPath = binDir + tvBinary
			epc.procs[i].Config().dataDirPath = fmt.Sprintf("test%d.etcd", i)
			if err := epc.procs[i].Restart(); err != nil {
				t.Errorf("error restarting etcd process (%v)", err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	for i := range epc.procs {
		testDowngradeEnabled(cx, epc.procs[i].Config().acurl, "false")
	}
	clusterVersionTest(cx, `"etcdcluster":"`+tv.String())
	testKVPairs(cx, kvs)
}

func testDowngradeEnabled(cx ctlCtx, endpoint string, expected string) {
	cmdArgs := []string{"curl", "-L", endpoint + "/downgrade/enabled"}
	fmt.Println(cmdArgs)
	var err error
	for i := 0; i < 7; i++ {
		time.Sleep(time.Second)
		if err = spawnWithExpect(cmdArgs, expected); err != nil {
			cx.t.Logf("#%d: downgrade enabled status does not match the expected. (%v)", i, err)
			continue
		}
		break
	}
	if err != nil {
		cx.t.Fatalf("downgrade enabled status(%v) does not match the expected(%v)", err, expected)
	}
}
func testHealth(cx ctlCtx) {
	if err := ctlV3EndpointHealth(cx); err != nil {
		cx.t.Fatalf("error check endpoints healthy (%v)", err)
	}
}
func testKVPairs(cx ctlCtx, kvs []kv) {
	for j := range kvs {
		if err := ctlV3Get(cx, []string{kvs[j].key}, []kv{kvs[j]}...); err != nil {
			cx.t.Fatalf("ctlV3Get error (%v)", err)
		}
	}
}

func putKVPairs(cx ctlCtx, kvs []kv) {
	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("#%d: ctlV3Put error (%v)", i, err)
		}
	}
}
func testCtlDowngradeEnable(cx ctlCtx, tv string, expected string) error {
	cmdArgs := append(cx.PrefixArgs(), "downgrade", "enable", tv)
	return spawnWithExpect(cmdArgs, expected)
}
