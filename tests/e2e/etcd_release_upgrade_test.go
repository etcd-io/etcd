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
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/version"
)

// TestReleaseUpgrade ensures that changes to master branch does not affect
// upgrade from latest etcd releases.
func TestReleaseUpgrade(t *testing.T) {
	lastReleaseBinary := binDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	defer testutil.AfterTest(t)

	copiedCfg := configNoTLS
	copiedCfg.execPath = lastReleaseBinary
	copiedCfg.snapshotCount = 3
	copiedCfg.baseScheme = "unix" // to avoid port conflict

	epc, err := newEtcdProcessCluster(&copiedCfg)
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()
	// 3.0 boots as 2.3 then negotiates up to 3.0
	// so there's a window at boot time where it doesn't have V3rpcCapability enabled
	// poll /version until etcdcluster is >2.3.x before making v3 requests
	for i := 0; i < 7; i++ {
		if err = cURLGet(epc, cURLReq{endpoint: "/version", expected: `"etcdcluster":"` + version.Cluster(version.Version)}); err != nil {
			t.Logf("#%d: v3 is not ready yet (%v)", i, err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	if err != nil {
		t.Fatalf("cannot pull version (%v)", err)
	}

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

	// expect upgraded cluster version
	if err := cURLGet(cx.epc, cURLReq{endpoint: "/metrics", expected: fmt.Sprintf(`etcd_cluster_version{cluster_version="%s"} 1`, version.Cluster(version.Version)), metricsURLScheme: cx.cfg.metricsURLScheme}); err != nil {
		cx.t.Fatalf("failed get with curl (%v)", err)
	}
}

func TestReleaseUpgradeWithRestart(t *testing.T) {
	lastReleaseBinary := binDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	defer testutil.AfterTest(t)

	copiedCfg := configNoTLS
	copiedCfg.execPath = lastReleaseBinary
	copiedCfg.snapshotCount = 10
	copiedCfg.baseScheme = "unix"

	epc, err := newEtcdProcessCluster(&copiedCfg)
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
				t.Fatalf("error restarting etcd process (%v)", err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	if err := ctlV3Get(cx, []string{kvs[0].key}, []kv{kvs[0]}...); err != nil {
		t.Fatal(err)
	}
}

type cURLReq struct {
	username string
	password string

	isTLS   bool
	timeout int

	endpoint string

	value    string
	expected string
	header   string

	metricsURLScheme string

	ciphers string
}

// cURLPrefixArgs builds the beginning of a curl command for a given key
// addressed to a random URL in the given cluster.
func cURLPrefixArgs(clus *etcdProcessCluster, method string, req cURLReq) []string {
	var (
		cmdArgs = []string{"curl"}
		acurl   = clus.procs[rand.Intn(clus.cfg.clusterSize)].Config().acurl
	)
	if req.metricsURLScheme != "https" {
		if req.isTLS {
			if clus.cfg.clientTLS != clientTLSAndNonTLS {
				panic("should not use cURLPrefixArgsUseTLS when serving only TLS or non-TLS")
			}
			cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
			acurl = toTLS(clus.procs[rand.Intn(clus.cfg.clusterSize)].Config().acurl)
		} else if clus.cfg.clientTLS == clientTLS {
			if !clus.cfg.noCN {
				cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath, "--key", privateKeyPath)
			} else {
				cmdArgs = append(cmdArgs, "--cacert", caPath, "--cert", certPath3, "--key", privateKeyPath3)
			}
		}
	}
	if req.metricsURLScheme != "" {
		acurl = clus.procs[rand.Intn(clus.cfg.clusterSize)].EndpointsMetrics()[0]
	}
	ep := acurl + req.endpoint

	if req.username != "" || req.password != "" {
		cmdArgs = append(cmdArgs, "-L", "-u", fmt.Sprintf("%s:%s", req.username, req.password), ep)
	} else {
		cmdArgs = append(cmdArgs, "-L", ep)
	}
	if req.timeout != 0 {
		cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%d", req.timeout))
	}

	if req.header != "" {
		cmdArgs = append(cmdArgs, "-H", req.header)
	}

	if req.ciphers != "" {
		cmdArgs = append(cmdArgs, "--ciphers", req.ciphers)
	}

	switch method {
	case "POST", "PUT":
		dt := req.value
		if !strings.HasPrefix(dt, "{") { // for non-JSON value
			dt = "value=" + dt
		}
		cmdArgs = append(cmdArgs, "-X", method, "-d", dt)
	}
	return cmdArgs
}

func cURLPost(clus *etcdProcessCluster, req cURLReq) error {
	return spawnWithExpect(cURLPrefixArgs(clus, "POST", req), req.expected)
}

func cURLPut(clus *etcdProcessCluster, req cURLReq) error {
	return spawnWithExpect(cURLPrefixArgs(clus, "PUT", req), req.expected)
}

func cURLGet(clus *etcdProcessCluster, req cURLReq) error {
	return spawnWithExpect(cURLPrefixArgs(clus, "GET", req), req.expected)
}
