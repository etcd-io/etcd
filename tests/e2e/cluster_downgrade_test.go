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
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestDowngradeUpgradeClusterOf1(t *testing.T) {
	testDowngradeUpgrade(t, 1, false)
}

func TestDowngradeUpgradeClusterOf3(t *testing.T) {
	testDowngradeUpgrade(t, 3, false)
}

func TestDowngradeUpgradeClusterOf1WithSnapshot(t *testing.T) {
	testDowngradeUpgrade(t, 1, true)
}

func TestDowngradeUpgradeClusterOf3WithSnapshot(t *testing.T) {
	testDowngradeUpgrade(t, 3, true)
}

func testDowngradeUpgrade(t *testing.T, clusterSize int, triggerSnapshot bool) {
	currentEtcdBinary := e2e.BinPath.Etcd
	lastReleaseBinary := e2e.BinPath.EtcdLastRelease
	if !fileutil.Exist(lastReleaseBinary) {
		t.Skipf("%q does not exist", lastReleaseBinary)
	}

	currentVersion, err := e2e.GetVersionFromBinary(currentEtcdBinary)
	require.NoError(t, err)
	// wipe any pre-release suffix like -alpha.0 we see commonly in builds
	currentVersion.PreRelease = ""

	lastVersion, err := e2e.GetVersionFromBinary(lastReleaseBinary)
	require.NoError(t, err)

	require.Equalf(t, lastVersion.Minor, currentVersion.Minor-1, "unexpected minor version difference")
	currentVersionStr := currentVersion.String()
	lastVersionStr := lastVersion.String()

	lastClusterVersion := semver.New(lastVersionStr)
	lastClusterVersion.Patch = 0
	lastClusterVersionStr := lastClusterVersion.String()

	e2e.BeforeTest(t)

	t.Logf("Create cluster with version %s", currentVersionStr)
	var snapshotCount uint64 = 10
	epc := newCluster(t, clusterSize, snapshotCount)
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{
			Cluster: currentVersionStr,
			Server:  version.Version,
			Storage: currentVersionStr,
		})
	}
	t.Logf("Cluster created")
	if triggerSnapshot {
		t.Logf("Generating snapshot")
		generateSnapshot(t, snapshotCount, epc)
	}
	bm, bkv := getMembersAndKeys(t, epc)

	t.Logf("etcdctl downgrade enable %s", lastVersionStr)
	downgradeEnable(t, epc, lastVersion)

	t.Log("Downgrade enabled, validating if cluster is ready for downgrade")
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{
			Cluster: lastClusterVersionStr,
			Server:  version.Version,
			Storage: lastClusterVersionStr,
		})
		e2e.AssertProcessLogs(t, epc.Procs[i], "The server is ready to downgrade")
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
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{
			Cluster: lastClusterVersionStr,
			Server:  lastVersionStr,
		})
	}

	t.Log("Downgrade complete")
	am, akv := getMembersAndKeys(t, epc)
	assert.Equal(t, bkv.Kvs, akv.Kvs)
	assert.Equal(t, bm.Members, am.Members)

	t.Logf("Starting upgrade process to %q", currentVersionStr)
	for i := 0; i < len(epc.Procs); i++ {
		t.Logf("Upgrading member %d", i)
		stopEtcd(t, epc.Procs[i])
		startEtcd(t, epc.Procs[i], currentEtcdBinary)
		// NOTE: The leader has monitor to the cluster version, which will
		// update cluster version. We don't need to check the transient
		// version just in case that it might be flaky.
	}

	t.Log("All members upgraded, validating upgrade")
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{
			Cluster: currentVersionStr,
			Server:  version.Version,
			Storage: currentVersionStr,
		})
	}
	t.Log("Upgrade complete")
}

func newCluster(t *testing.T, clusterSize int, snapshotCount uint64) *e2e.EtcdProcessCluster {
	epc, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
		e2e.WithClusterSize(clusterSize),
		e2e.WithSnapshotCount(snapshotCount),
		e2e.WithKeepDataDir(true),
	)
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

func downgradeEnable(t *testing.T, epc *e2e.EtcdProcessCluster, ver *semver.Version) {
	c := epc.Etcdctl()
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
	testutils.ExecuteWithTimeout(t, 30*time.Second, func() {
		for {
			result, err := getMemberVersionByCurl(cfg, member)
			if err != nil {
				cfg.Logger.Warn("failed to get member version and retrying", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}

			if err := compareMemberVersion(expect, result); err != nil {
				cfg.Logger.Warn("failed to validate and retrying", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}
			break
		}
	})
}

func leader(t *testing.T, epc *e2e.EtcdProcessCluster) e2e.EtcdProcess {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	for i := 0; i < len(epc.Procs); i++ {
		endpoints := epc.Procs[i].EndpointsGRPC()
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

func compareMemberVersion(expect version.Versions, target version.Versions) error {
	if expect.Server != "" && expect.Server != target.Server {
		return fmt.Errorf("expect etcdserver version %v, but got %v", expect.Server, target.Server)
	}

	if expect.Cluster != "" && expect.Cluster != target.Cluster {
		return fmt.Errorf("expect etcdcluster version %v, but got %v", expect.Cluster, target.Cluster)
	}

	if expect.Storage != "" && expect.Storage != target.Storage {
		return fmt.Errorf("expect storage version %v, but got %v", expect.Storage, target.Storage)
	}
	return nil
}

func getMemberVersionByCurl(cfg *e2e.EtcdProcessClusterConfig, member e2e.EtcdProcess) (version.Versions, error) {
	args := e2e.CURLPrefixArgsCluster(cfg, member, "GET", e2e.CURLReq{Endpoint: "/version"})
	lines, err := e2e.RunUtilCompletion(args, nil)
	if err != nil {
		return version.Versions{}, err
	}

	data := strings.Join(lines, "\n")
	result := version.Versions{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return version.Versions{}, fmt.Errorf("failed to unmarshal (%v): %w", data, err)
	}
	return result, nil
}

func generateSnapshot(t *testing.T, snapshotCount uint64, epc *e2e.EtcdProcessCluster) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cc, err := e2e.NewEtcdctl(epc.Cfg.Client, epc.EndpointsGRPC())
	assert.NoError(t, err)

	var i uint64
	t.Logf("Adding keys")
	for i = 0; i < snapshotCount*3; i++ {
		err := cc.Put(ctx, fmt.Sprintf("%d", i), "1", config.PutOptions{})
		assert.NoError(t, err)
	}
	verifySnapshot(t, epc)
}

func verifySnapshot(t *testing.T, epc *e2e.EtcdProcessCluster) {
	for i := range epc.Procs {
		t.Logf("Verifying snapshot for member %d", i)
		ss := snap.New(epc.Cfg.Logger, datadir.ToSnapDir(epc.Procs[i].Config().DataDirPath))
		_, err := ss.Load()
		assert.NoError(t, err)
	}
	t.Logf("All members have a valid snapshot")
}

func getMembersAndKeys(t *testing.T, epc *e2e.EtcdProcessCluster) (*clientv3.MemberListResponse, *clientv3.GetResponse) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cc, err := e2e.NewEtcdctl(epc.Cfg.Client, epc.EndpointsGRPC())
	assert.NoError(t, err)

	kvs, err := cc.Get(ctx, "", config.GetOptions{Prefix: true})
	assert.NoError(t, err)

	members, err := cc.MemberList(ctx, false)
	assert.NoError(t, err)

	return members, kvs
}
