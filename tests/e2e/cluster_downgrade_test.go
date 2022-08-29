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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestDowngradeUpgradeClusterOf1(t *testing.T) {
	testDowngradeUpgrade(t, 1)
}

func TestDowngradeUpgradeClusterOf3(t *testing.T) {
	testDowngradeUpgrade(t, 3)
}

func testDowngradeUpgrade(t *testing.T, clusterSize int) {
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

	t.Logf("Create cluster with version %s", currentVersionStr)
	epc := newCluster(t, currentEtcdBinary, clusterSize)
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{Cluster: currentVersionStr, Server: currentVersionStr})
	}
	t.Logf("Cluster created")

	t.Logf("etcdctl downgrade enable %s", lastVersionStr)
	downgradeEnable(t, epc, lastVersion)

	t.Log("Downgrade enabled, validating if cluster is ready for downgrade")
	for i := 0; i < len(epc.Procs); i++ {
		e2e.AssertProcessLogs(t, epc.Procs[i], "The server is ready to downgrade")
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{Cluster: lastVersionStr, Server: currentVersionStr})
	}
	t.Log("Cluster is ready for downgrade")

	t.Logf("Starting downgrade process to %q", lastVersionStr)
	for i := 0; i < len(epc.Procs); i++ {
		t.Logf("Downgrading member %d by running %s binary", i, lastReleaseBinary)
		stopEtcd(t, epc.Procs[i])
		startEtcd(t, epc.Procs[i], lastReleaseBinary)
	}
	t.Log("All members downgraded, validating downgrade")
	e2e.AssertProcessLogs(t, leader(t, epc), "the cluster has been downgraded")
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{Cluster: lastVersionStr, Server: lastVersionStr})
	}
	t.Log("Downgrade complete")

	t.Logf("Starting upgrade process to %q", currentVersionStr)
	for i := 0; i < len(epc.Procs); i++ {
		t.Logf("Upgrading member %d", i)
		stopEtcd(t, epc.Procs[i])
		startEtcd(t, epc.Procs[i], currentEtcdBinary)
		if i+1 < len(epc.Procs) {
			validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{Cluster: lastVersionStr, Server: currentVersionStr})
		}
	}
	t.Log("All members upgraded, validating upgrade")
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{Cluster: currentVersionStr, Server: currentVersionStr})
	}
	t.Log("Upgrade complete")
}

func newCluster(t *testing.T, execPath string, clusterSize int) *e2e.EtcdProcessCluster {
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t, &e2e.EtcdProcessClusterConfig{
		ExecPath:     execPath,
		ClusterSize:  clusterSize,
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

func startEtcd(t *testing.T, ep e2e.EtcdProcess, execPath string) {
	ep.Config().ExecPath = execPath
	err := ep.Restart(context.TODO())
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
}

func downgradeEnable(t *testing.T, epc *e2e.EtcdProcessCluster, ver semver.Version) {
	c := e2e.NewEtcdctl(epc.Cfg, epc.EndpointsV3())
	testutils.ExecuteWithTimeout(t, 20*time.Second, func() {
		err := c.DowngradeEnable(context.TODO(), ver.String())
		if err != nil {
			t.Fatal(err)
		}
	})
}

func stopEtcd(t *testing.T, ep e2e.EtcdProcess) {
	if err := ep.Stop(); err != nil {
		t.Fatal(err)
	}
}

func validateVersion(t *testing.T, cfg *e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess, expect version.Versions) {
	// Two separate calls to expect as it doesn't support multiple matches on the same line
	var err error
	testutils.ExecuteWithTimeout(t, 20*time.Second, func() {
		for {
			if expect.Server != "" {
				err = e2e.SpawnWithExpects(e2e.CURLPrefixArgs(cfg, member, "GET", e2e.CURLReq{Endpoint: "/version"}), nil, `"etcdserver":"`+expect.Server)
				if err != nil {
					time.Sleep(time.Second)
					continue
				}
			}
			if expect.Cluster != "" {
				err = e2e.SpawnWithExpects(e2e.CURLPrefixArgs(cfg, member, "GET", e2e.CURLReq{Endpoint: "/version"}), nil, `"etcdcluster":"`+expect.Cluster)
				if err != nil {
					time.Sleep(time.Second)
					continue
				}
			}
			break
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func leader(t *testing.T, epc *e2e.EtcdProcessCluster) e2e.EtcdProcess {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	for i := 0; i < len(epc.Procs); i++ {
		endpoints := epc.Procs[i].EndpointsV3()
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   endpoints,
			DialTimeout: 3 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer cli.Close()
		resp, err := cli.Status(ctx, endpoints[0])
		if err != nil {
			t.Fatal(err)
		}
		if resp.Header.GetMemberId() == resp.Leader {
			return epc.Procs[i]
		}
	}
	t.Fatal("Leader not found")
	return nil
}
