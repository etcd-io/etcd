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

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
)

// TestReleaseUpgrade ensures that changes to master branch does not affect
// upgrade from latest etcd releases.
func TestReleaseUpgrade(t *testing.T) {
	lastReleaseBinary := binDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	BeforeTest(t)

	copiedCfg := newConfigNoTLS()
	copiedCfg.execPath = lastReleaseBinary
	copiedCfg.snapshotCount = 3
	copiedCfg.baseScheme = "unix" // to avoid port conflict

	epc, err := newEtcdProcessCluster(t, copiedCfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         *newConfigNoTLS(),
		dialTimeout: 7 * time.Second,
		quorum:      true,
		epc:         epc,
	}
	var kvs []kv
	for i := 0; i < 5; i++ {
		kvs = append(kvs, kv{key: fmt.Sprintf("foo%d", i), val: "bar"})
	}
	for i := range kvs {
		if err := ctlV3Put(cx, kvs[i].key, kvs[i].val, ""); err != nil {
			cx.t.Fatalf("#%d: ctlV3Put error (%v)", i, err)
		}
	}

	t.Log("Cluster of etcd in old version running")

	for i := range epc.procs {
		t.Logf("Stopping node: %v", i)
		if err := epc.procs[i].Stop(); err != nil {
			t.Fatalf("#%d: error closing etcd process (%v)", i, err)
		}
		t.Logf("Stopped node: %v", i)
		epc.procs[i].Config().execPath = binDir + "/etcd"
		epc.procs[i].Config().keepDataDir = true

		t.Logf("Restarting node in the new version: %v", i)
		if err := epc.procs[i].Restart(); err != nil {
			t.Fatalf("error restarting etcd process (%v)", err)
		}

		t.Logf("Testing reads after node restarts: %v", i)
		for j := range kvs {
			if err := ctlV3Get(cx, []string{kvs[j].key}, []kv{kvs[j]}...); err != nil {
				cx.t.Fatalf("#%d-%d: ctlV3Get error (%v)", i, j, err)
			}
		}
		t.Logf("Tested reads after node restarts: %v", i)
	}

	t.Log("Waiting for full upgrade...")
	// TODO: update after release candidate
	// expect upgraded cluster version
	// new cluster version needs more time to upgrade
	ver := version.Cluster(version.Version)
	for i := 0; i < 7; i++ {
		if err = cURLGet(epc, cURLReq{endpoint: "/version", expected: `"etcdcluster":"` + ver}); err != nil {
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
	lastReleaseBinary := binDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	BeforeTest(t)

	copiedCfg := newConfigNoTLS()
	copiedCfg.execPath = lastReleaseBinary
	copiedCfg.snapshotCount = 10
	copiedCfg.baseScheme = "unix"

	epc, err := newEtcdProcessCluster(t, copiedCfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         *newConfigNoTLS(),
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
