// Copyright 2021 The etcd Authors
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
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestDowngradeUpgrade(t *testing.T) {
	currentEtcdBinary := e2e.BinDir + "/etcd"
	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}
	currentVersion := semver.New(version.Version)
	lastVersion := semver.Version{Major: currentVersion.Major, Minor: currentVersion.Minor - 1}
	currentVersionStr := fmt.Sprintf("%d.%d", currentVersion.Major, currentVersion.Minor)
	lastVersionStr := fmt.Sprintf("%d.%d", lastVersion.Major, lastVersion.Minor)

	e2e.BeforeTest(t)

	epc := newCluster(t, currentEtcdBinary)
	validateVersion(t, epc, version.Versions{Cluster: currentVersionStr, Server: currentVersionStr})

	downgradeEnable(t, epc, lastVersion)
	expectLog(t, epc, "The server is ready to downgrade")
	validateVersion(t, epc, version.Versions{Cluster: lastVersionStr, Server: currentVersionStr})

	stopEtcd(t, epc.Procs[0])
	startEtcd(t, epc, lastReleaseBinary)
	expectLog(t, epc, "the cluster has been downgraded")
	validateVersion(t, epc, version.Versions{Cluster: lastVersionStr, Server: lastVersionStr})

	stopEtcd(t, epc.Procs[0])
	startEtcd(t, epc, currentEtcdBinary)
	validateVersion(t, epc, version.Versions{Cluster: currentVersionStr, Server: currentVersionStr})
}

func newCluster(t *testing.T, execPath string) *e2e.EtcdProcessCluster {
	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ExecPath:     execPath,
		ClusterSize:  1,
		InitialToken: "new",
		KeepDataDir:  true,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})
	return epc
}

func startEtcd(t *testing.T, epc *e2e.EtcdProcessCluster, execPath string) {
	epc.Procs[0].Config().ExecPath = execPath
	err := epc.Procs[0].Restart()
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
}

func downgradeEnable(t *testing.T, epc *e2e.EtcdProcessCluster, ver semver.Version) {
	t.Log("etcdctl downgrade...")
	c := e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())
	testutils.ExecuteWithTimeout(t, 20*time.Second, func() {
		err := c.DowngradeEnable(ver.String())
		if err != nil {
			t.Fatal(err)
		}
	})
}

func stopEtcd(t *testing.T, ep e2e.EtcdProcess) {
	t.Log("Stopping the server...")
	if err := ep.Stop(); err != nil {
		t.Fatal(err)
	}
}

func validateVersion(t *testing.T, epc *e2e.EtcdProcessCluster, expect version.Versions) {
	t.Log("Validate version")
	// Two separate calls to expect as it doesn't support multiple matches on the same line
	var err error
	testutils.ExecuteWithTimeout(t, 20*time.Second, func() {
		if expect.Server != "" {
			err = e2e.SpawnWithExpects(e2e.CURLPrefixArgs(epc.Cfg, epc.Procs[rand.Intn(epc.Cfg.ClusterSize)], "GET", e2e.CURLReq{Endpoint: "/version"}), nil, `"etcdserver":"`+expect.Server)
			if err != nil {
				return
			}
		}
		if expect.Cluster != "" {
			err = e2e.SpawnWithExpects(e2e.CURLPrefixArgs(epc.Cfg, epc.Procs[rand.Intn(epc.Cfg.ClusterSize)], "GET", e2e.CURLReq{Endpoint: "/version"}), nil, `"etcdcluster":"`+expect.Cluster)
			if err != nil {
				return
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func expectLog(t *testing.T, epc *e2e.EtcdProcessCluster, expectLog string) {
	t.Helper()
	var err error
	testutils.ExecuteWithTimeout(t, 30*time.Second, func() {
		_, err = epc.Procs[0].Logs().Expect(expectLog)
	})
	if err != nil {
		t.Fatal(err)
	}
}
