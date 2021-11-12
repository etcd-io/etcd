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
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestDowngradeUpgrade(t *testing.T) {
	currentEtcdBinary := ""
	lastReleaseBinary := e2e.BinDir + "/etcd-last-release"
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}
	currentVersion := semver.New(version.Version)
	lastVersion := semver.Version{Major: currentVersion.Major, Minor: currentVersion.Minor - 1}
	currentVersionStr := fmt.Sprintf("%d.%d", currentVersion.Major, currentVersion.Minor)
	lastVersionStr := fmt.Sprintf("%d.%d", lastVersion.Major, lastVersion.Minor)

	e2e.BeforeTest(t)
	dataDirPath := t.TempDir()

	epc := startEtcd(t, currentEtcdBinary, dataDirPath)
	validateVersion(t, epc, version.Versions{Cluster: currentVersionStr, Server: currentVersionStr})

	downgradeEnable(t, epc, lastVersion)
	validateVersion(t, epc, version.Versions{Cluster: lastVersionStr, Server: currentVersionStr})

	stopEtcd(t, epc)
	epc = startEtcd(t, lastReleaseBinary, dataDirPath)
	validateVersion(t, epc, version.Versions{Cluster: lastVersionStr, Server: lastVersionStr})
	expectLog(t, epc, "the cluster has been downgraded")

	stopEtcd(t, epc)
	epc = startEtcd(t, currentEtcdBinary, dataDirPath)
	// TODO: Verify cluster version after upgrade when we fix cluster version set timeout
	validateVersion(t, epc, version.Versions{Server: currentVersionStr})
}

func startEtcd(t *testing.T, execPath, dataDirPath string) *e2e.EtcdProcessCluster {
	epc, err := e2e.NewEtcdProcessCluster(t, &e2e.EtcdProcessClusterConfig{
		ExecPath:     execPath,
		DataDirPath:  dataDirPath,
		ClusterSize:  1,
		InitialToken: "new",
		KeepDataDir:  true,
		// TODO: REMOVE snapshot override when snapshotting is automated after lowering storage versiont l
		SnapshotCount: 5,
	})
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
	t.Cleanup(func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	})

	prefixArgs := []string{e2e.CtlBinPath, "--endpoints", strings.Join(epc.EndpointsV3(), ",")}
	t.Log("Write keys to ensure wal snapshot is created so cluster version set is snapshotted")
	e2e.ExecuteWithTimeout(t, 20*time.Second, func() {
		for i := 0; i < 10; i++ {
			if err := e2e.SpawnWithExpect(append(prefixArgs, "put", fmt.Sprintf("%d", i), "value"), "OK"); err != nil {
				t.Fatal(err)
			}
		}
	})
	return epc
}

func downgradeEnable(t *testing.T, epc *e2e.EtcdProcessCluster, ver semver.Version) {
	t.Log("etcdctl downgrade...")
	c, err := clientv3.New(clientv3.Config{
		Endpoints: epc.EndpointsV3(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	// TODO: Fix request always timing out even thou it succeeds
	c.Downgrade(ctx, 1, ver.String())
	cancel()

	expectLog(t, epc, "The server is ready to downgrade")
}

func stopEtcd(t *testing.T, epc *e2e.EtcdProcessCluster) {
	t.Log("Stopping the server...")
	if err := epc.Procs[0].Stop(); err != nil {
		t.Fatal(err)
	}
}

func validateVersion(t *testing.T, epc *e2e.EtcdProcessCluster, expect version.Versions) {
	t.Log("Validate version")
	// Two separate calls to expect as it doesn't support multiple matches on the same line
	e2e.ExecuteWithTimeout(t, 20*time.Second, func() {
		if expect.Server != "" {
			err := e2e.SpawnWithExpects(e2e.CURLPrefixArgs(epc, "GET", e2e.CURLReq{Endpoint: "/version"}), nil, `"etcdserver":"`+expect.Server)
			if err != nil {
				t.Fatal(err)
			}
		}
		if expect.Cluster != "" {
			err := e2e.SpawnWithExpects(e2e.CURLPrefixArgs(epc, "GET", e2e.CURLReq{Endpoint: "/version"}), nil, `"etcdcluster":"`+expect.Cluster)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

func expectLog(t *testing.T, epc *e2e.EtcdProcessCluster, expectLog string) {
	t.Helper()
	e2e.ExecuteWithTimeout(t, 30*time.Second, func() {
		_, err := epc.Procs[0].Logs().Expect(expectLog)
		if err != nil {
			t.Fatal(err)
		}
	})
}
