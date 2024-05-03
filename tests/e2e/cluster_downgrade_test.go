// Copyright 2024 The etcd Authors
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
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/testutils"
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
	currentEtcdBinary := e2e.BinPath
	lastReleaseBinary := e2e.BinPathLastRelease
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

	currentClusterVersion := semver.New(currentVersionStr)
	currentClusterVersion.Patch = 0
	currentClusterVersionStr := currentClusterVersion.String()

	lastClusterVersion := semver.New(lastVersionStr)
	lastClusterVersion.Patch = 0
	lastClusterVersionStr := lastClusterVersion.String()

	e2e.BeforeTest(t)

	t.Logf("Create cluster with version %s", currentVersionStr)
	snapshotCount := 10
	epc := newCluster(t, clusterSize, snapshotCount)
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{
			Cluster: currentClusterVersionStr,
			Server:  version.Version,
		})
	}
	cc := epc.Etcdctl()
	t.Logf("Cluster created")
	if len(epc.Procs) > 1 {
		t.Log("Waiting health interval to required to make membership changes")
		time.Sleep(etcdserver.HealthInterval)
	}

	t.Log("Adding member to test membership, but a learner avoid breaking quorum")
	resp, err := cc.MemberAddAsLearner("fake1", []string{"http://127.0.0.1:1001"})
	require.NoError(t, err)
	if triggerSnapshot {
		t.Logf("Generating snapshot")
		generateSnapshot(t, snapshotCount, cc)
		verifySnapshot(t, epc)
	}
	t.Log("Removing learner to test membership")
	_, err = cc.MemberRemove(resp.Member.ID)
	require.NoError(t, err)
	beforeMembers, beforeKV := getMembersAndKeys(t, cc)

	t.Logf("Starting downgrade process to %q", lastVersionStr)
	for i := 0; i < len(epc.Procs); i++ {
		t.Logf("Downgrading member %d by running %s binary", i, lastReleaseBinary)
		stopEtcd(t, epc.Procs[i])
		startEtcd(t, epc.Procs[i], lastReleaseBinary)
	}

	t.Log("All members downgraded, validating downgrade")
	for i := 0; i < len(epc.Procs); i++ {
		validateVersion(t, epc.Cfg, epc.Procs[i], version.Versions{
			Cluster: lastClusterVersionStr,
			Server:  lastVersionStr,
		})
	}

	t.Log("Downgrade complete")
	afterMembers, afterKV := getMembersAndKeys(t, cc)
	assert.Equal(t, beforeKV.Kvs, afterKV.Kvs)
	assert.Equal(t, beforeMembers.Members, afterMembers.Members)

	if len(epc.Procs) > 1 {
		t.Log("Waiting health interval to required to make membership changes")
		time.Sleep(etcdserver.HealthInterval)
	}
	t.Log("Adding learner to test membership, but avoid breaking quorum")
	resp, err = cc.MemberAddAsLearner("fake2", []string{"http://127.0.0.1:1002"})
	require.NoError(t, err)
	if triggerSnapshot {
		t.Logf("Generating snapshot")
		generateSnapshot(t, snapshotCount, cc)
		verifySnapshot(t, epc)
	}
	t.Log("Removing learner to test membership")
	_, err = cc.MemberRemove(resp.Member.ID)
	require.NoError(t, err)
	beforeMembers, beforeKV = getMembersAndKeys(t, cc)

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
			Cluster: currentClusterVersionStr,
			Server:  version.Version,
		})
	}
	t.Log("Upgrade complete")

	afterMembers, afterKV = getMembersAndKeys(t, cc)
	assert.Equal(t, beforeKV.Kvs, afterKV.Kvs)
	assert.Equal(t, beforeMembers.Members, afterMembers.Members)
}

func newCluster(t *testing.T, clusterSize int, snapshotCount int) *e2e.EtcdProcessCluster {
	copiedCfg := e2e.NewConfigNoTLS()
	copiedCfg.ClusterSize = clusterSize
	copiedCfg.SnapshotCount = snapshotCount
	copiedCfg.KeepDataDir = true
	copiedCfg.BasePeerScheme = "unix" // to avoid port conflict

	epc, err := e2e.NewEtcdProcessCluster(t, copiedCfg)
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
	if execPath == e2e.BinPathLastRelease {
		ep.Config().Args = addOrRemoveArg(ep.Config().Args, "--next-cluster-version-compatible", true)
	} else {
		ep.Config().Args = addOrRemoveArg(ep.Config().Args, "--next-cluster-version-compatible", false)
	}
	err := ep.Restart()
	if err != nil {
		t.Fatalf("could not start etcd process cluster (%v)", err)
	}
}

func addOrRemoveArg(args []string, arg string, isAdd bool) []string {
	newArgs := []string{}
	for _, s := range args {
		if s != arg {
			newArgs = append(newArgs, s)
		}
	}
	if isAdd {
		newArgs = append(newArgs, arg)
	}
	return newArgs
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
				cfg.Logger.Warn("failed to get member version and retrying", zap.Error(err), zap.String("member", member.Config().Name))
				time.Sleep(time.Second)
				continue
			}
			cfg.Logger.Info("Comparing versions", zap.String("member", member.Config().Name), zap.Any("got", result), zap.Any("want", expect))
			if err := compareMemberVersion(expect, result); err != nil {
				cfg.Logger.Warn("Versions didn't match retrying", zap.Error(err), zap.String("member", member.Config().Name))
				time.Sleep(time.Second)
				continue
			}
			cfg.Logger.Info("Versions match", zap.String("member", member.Config().Name))
			break
		}
	})
}

func compareMemberVersion(expect version.Versions, target version.Versions) error {
	if expect.Server != "" && expect.Server != target.Server {
		return fmt.Errorf("expect etcdserver version %v, but got %v", expect.Server, target.Server)
	}

	if expect.Cluster != "" && expect.Cluster != target.Cluster {
		return fmt.Errorf("expect etcdcluster version %v, but got %v", expect.Cluster, target.Cluster)
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

func generateSnapshot(t *testing.T, snapshotCount int, cc *e2e.Etcdctl) {
	t.Logf("Adding keys")
	for i := 0; i < snapshotCount*3; i++ {
		err := cc.Put(fmt.Sprintf("%d", i), "1")
		assert.NoError(t, err)
	}
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

func getMembersAndKeys(t *testing.T, cc *e2e.Etcdctl) (*clientv3.MemberListResponse, *clientv3.GetResponse) {
	kvs, err := cc.GetWithPrefix("")
	assert.NoError(t, err)

	members, err := cc.MemberList()
	assert.NoError(t, err)

	return members, kvs
}
