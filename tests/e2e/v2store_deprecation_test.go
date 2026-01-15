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
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestV2DeprecationNotYet(t *testing.T) {
	e2e.BeforeTest(t)
	t.Log("Verify its infeasible to start etcd with --v2-deprecation=not-yet mode")
	proc, err := e2e.SpawnCmd([]string{e2e.BinPath.Etcd, "--v2-deprecation=not-yet"}, nil)
	require.NoError(t, err)

	_, err = proc.Expect(`invalid value "not-yet" for flag -v2-deprecation: invalid value "not-yet"`)
	assert.NoError(t, err)
}

// TestV2DeprecationSnapshotMatches ensures that etcd v3.7 still commits v2 store
// changes to the snapshot for backwards compatibility.
func TestV2DeprecationSnapshotMatches(t *testing.T) {
	e2e.BeforeTest(t)
	lastReleaseData := t.TempDir()
	currentReleaseData := t.TempDir()
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var snapshotCount uint64 = 10

	epc := runEtcdAndCreateSnapshot(t, e2e.LastVersion, lastReleaseData, snapshotCount)
	oldMemberDataDir := epc.Procs[0].Config().DataDirPath
	cc1 := epc.Etcdctl()
	addAndRemoveKeysAndMembers(ctx, t, cc1, snapshotCount)
	require.NoError(t, epc.Close())

	epc = runEtcdAndCreateSnapshot(t, e2e.CurrentVersion, currentReleaseData, snapshotCount)
	newMemberDataDir := epc.Procs[0].Config().DataDirPath
	cc2 := epc.Etcdctl()
	addAndRemoveKeysAndMembers(ctx, t, cc2, snapshotCount)
	require.NoError(t, epc.Close())

	assertSnapshotsMatch(t, oldMemberDataDir, newMemberDataDir)
}

func addAndRemoveKeysAndMembers(ctx context.Context, tb testing.TB, cc *e2e.EtcdctlV3, snapshotCount uint64) {
	// Execute some non-trivial key&member operation
	var i uint64
	for i = 0; i < snapshotCount*3; i++ {
		_, err := cc.Put(ctx, fmt.Sprintf("%d", i), "1", config.PutOptions{})
		require.NoError(tb, err)
	}
	member1, err := cc.MemberAddAsLearner(ctx, "member1", []string{"http://127.0.0.1:2000"})
	require.NoError(tb, err)

	for i = 0; i < snapshotCount*2; i++ {
		_, err = cc.Delete(ctx, fmt.Sprintf("%d", i), config.DeleteOptions{})
		require.NoError(tb, err)
	}
	_, err = cc.MemberRemove(ctx, member1.Member.ID)
	require.NoError(tb, err)

	for i = 0; i < snapshotCount; i++ {
		_, err = cc.Put(ctx, fmt.Sprintf("%d", i), "2", config.PutOptions{})
		require.NoError(tb, err)
	}
	_, err = cc.MemberAddAsLearner(ctx, "member2", []string{"http://127.0.0.1:2001"})
	require.NoError(tb, err)

	for i = 0; i < snapshotCount/2; i++ {
		_, err = cc.Put(ctx, fmt.Sprintf("%d", i), "3", config.PutOptions{})
		assert.NoError(tb, err)
	}
}

func TestV2DeprecationSnapshotRecover(t *testing.T) {
	e2e.BeforeTest(t)
	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}
	epc := runEtcdAndCreateSnapshot(t, e2e.LastVersion, dataDir, 10)

	cc := epc.Etcdctl()
	lastReleaseGetResponse, err := cc.Get(ctx, "", config.GetOptions{Prefix: true})
	require.NoError(t, err)

	lastReleaseMemberListResponse, err := cc.MemberList(ctx, false)
	assert.NoError(t, err)

	assert.NoError(t, epc.Close())
	cfg := e2e.ConfigStandalone(*e2e.NewConfig(
		e2e.WithVersion(e2e.CurrentVersion),
		e2e.WithDataDirPath(dataDir),
	))
	epc, err = e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
	require.NoError(t, err)

	cc = epc.Etcdctl()
	currentReleaseGetResponse, err := cc.Get(ctx, "", config.GetOptions{Prefix: true})
	require.NoError(t, err)

	currentReleaseMemberListResponse, err := cc.MemberList(ctx, false)
	require.NoError(t, err)

	assert.Equal(t, lastReleaseGetResponse.Kvs, currentReleaseGetResponse.Kvs)
	assert.Equal(t, lastReleaseMemberListResponse.Members, currentReleaseMemberListResponse.Members)
	assert.NoError(t, epc.Close())
}

func runEtcdAndCreateSnapshot(tb testing.TB, serverVersion e2e.ClusterVersion, dataDir string, snapshotCount uint64) *e2e.EtcdProcessCluster {
	cfg := e2e.ConfigStandalone(*e2e.NewConfig(
		e2e.WithVersion(serverVersion),
		e2e.WithDataDirPath(dataDir),
		e2e.WithSnapshotCount(snapshotCount),
		e2e.WithKeepDataDir(true),
	))
	epc, err := e2e.NewEtcdProcessCluster(tb.Context(), tb, e2e.WithConfig(cfg))
	assert.NoError(tb, err)
	return epc
}

func filterSnapshotFiles(path string) bool {
	return strings.HasSuffix(path, ".snap")
}

func assertSnapshotsMatch(tb testing.TB, firstDataDir, secondDataDir string) {
	lg := zaptest.NewLogger(tb)

	firstFiles, err := fileutil.ListFiles(firstDataDir, filterSnapshotFiles)
	require.NoError(tb, err)

	secondFiles, err := fileutil.ListFiles(secondDataDir, filterSnapshotFiles)
	require.NoError(tb, err)

	assert.NotEmpty(tb, firstFiles)
	assert.NotEmpty(tb, secondFiles)
	assert.Len(tb, secondFiles, len(firstFiles))

	sort.Strings(firstFiles)
	sort.Strings(secondFiles)
	for i := 0; i < len(firstFiles); i++ {
		assertV2StoreMembershipEqual(tb, lg, firstFiles[i], secondFiles[i])
	}
}

func assertV2StoreMembershipEqual(tb testing.TB, lg *zap.Logger, firstSnapPath, secondSnapPath string) {
	st1 := loadV2StoreData(tb, lg, firstSnapPath)
	st2 := loadV2StoreData(tb, lg, secondSnapPath)

	st1Members, st1Deleted := membership.MembersFromStore(lg, st1)
	st2Members, st2Deleted := membership.MembersFromStore(lg, st2)

	require.Lenf(tb, st1Members, len(st2Members), "number of members in v2 store do not match")
	require.NotEmptyf(tb, st1Members, "no members found in v2 store")
	require.Lenf(tb, st1Deleted, len(st2Deleted), "number of deleted members in v2 store do not match")

	// remove ID because original ID was generated from hash of peerURLs + clusterName + time
	require.Equal(tb, rebuildMembers(tb, st1Members), rebuildMembers(tb, st2Members))
}

// loadV2StoreData reads v2 store from the snapshot file at fpath.
func loadV2StoreData(tb testing.TB, lg *zap.Logger, fpath string) v2store.Store {
	sn, err := snap.Read(lg, fpath)
	require.NoError(tb, err)

	v2data := v2store.New(etcdserver.StoreClusterPrefix, etcdserver.StoreKeysPrefix)
	v2data.Recovery(sn.Data)
	return v2data
}

// rebuildMembers rebuilds the members map with zeroed IDs and peerURLs as keys.
func rebuildMembers(tb testing.TB, members map[types.ID]*membership.Member) map[string]*membership.Member {
	newMembers := make(map[string]*membership.Member)
	for _, m := range members {
		peerURLs, err := types.NewURLs(m.PeerURLs)
		require.NoError(tb, err)

		m.ID = 0
		newMembers[peerURLs.String()] = m
	}
	return newMembers
}
