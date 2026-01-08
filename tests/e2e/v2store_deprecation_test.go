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
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
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

func TestV2DeprecationSnapshotMatches(t *testing.T) {
	e2e.BeforeTest(t)
	lastReleaseData := t.TempDir()
	currentReleaseData := t.TempDir()
	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}
	var snapshotCount uint64 = 10
	epc := runEtcdAndCreateSnapshot(t, e2e.LastVersion, lastReleaseData, snapshotCount)
	oldMemberDataDir := epc.Procs[0].Config().DataDirPath
	require.NoError(t, epc.Close())
	epc = runEtcdAndCreateSnapshot(t, e2e.CurrentVersion, currentReleaseData, snapshotCount)
	newMemberDataDir := epc.Procs[0].Config().DataDirPath
	require.NoError(t, epc.Close())

	assertSnapshotsMatch(t, oldMemberDataDir, newMemberDataDir)
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
		assertMembershipEqual(tb, lg)
	}
}

func assertMembershipEqual(tb testing.TB, lg *zap.Logger) {
	rc1 := membership.NewCluster(zaptest.NewLogger(tb))
	be1, _ := betesting.NewDefaultTmpBackend(tb)
	defer betesting.Close(tb, be1)
	rc1.SetBackend(schema.NewMembershipBackend(lg, be1))
	rc1.Recover(func(lg *zap.Logger, v *semver.Version) {})

	rc2 := membership.NewCluster(zaptest.NewLogger(tb))
	be2, _ := betesting.NewDefaultTmpBackend(tb)
	defer betesting.Close(tb, be2)
	rc2.SetBackend(schema.NewMembershipBackend(lg, be2))
	rc2.Recover(func(lg *zap.Logger, v *semver.Version) {})

	// membership should match
	if !reflect.DeepEqual(rc1.Members(), rc2.Members()) {
		tb.Logf("memberids_from_last_version = %+v, member_ids_from_current_version = %+v", rc1.MemberIDs(), rc2.MemberIDs())
		tb.Errorf("members_from_last_version_snapshot = %+v, members_from_current_version_snapshot %+v", rc1.Members(), rc2.Members())
	}
}
