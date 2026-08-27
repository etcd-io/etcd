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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

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

// TestCleanupOrphanedDefragFilesOnBootstrap verifies that etcd cleanup the
// orphaned defragmentation files on bootstrap.
func TestCleanupOrphanedDefragFilesOnBootstrap(t *testing.T) {
	testCleanupCertainFilesOnBootstrap(t, "db.tmp.defrag")
}

// TestCleanupV2SnapshotOnBootstrap verifies that etcd cleanup the legacy
// v2 snapshot files on bootstrap.
// TODO: we can remove this test in the next etcd release v3.9
func TestCleanupV2SnapshotOnBootstrap(t *testing.T) {
	testCleanupCertainFilesOnBootstrap(t, "10.snap")
}

func testCleanupCertainFilesOnBootstrap(t *testing.T, filename string) {
	e2e.BeforeTest(t)

	t.Log("Create a new single member etcd cluster")
	cfg := e2e.ConfigStandalone(*e2e.NewConfig(
		e2e.WithKeepDataDir(true),
	))
	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, epc.Close())
	}()

	t.Logf("Stop the etcd member, and create the given file %s under snapshot directory", filename)
	require.NoError(t, epc.Procs[0].Stop())

	snapshotDir := datadir.ToSnapDir(epc.Procs[0].Config().DataDirPath)
	require.NoError(t, os.WriteFile(filepath.Join(snapshotDir, filename), []byte{}, 0o644))

	fileExt := filepath.Ext(filename)
	names, rerr := fileutil.ReadDir(snapshotDir, fileutil.WithExt(fileExt))
	require.NoError(t, rerr)
	require.Len(t, names, 1)

	t.Logf("Start the etcd member again, and expect it removes the given file %s automatically", filename)
	require.NoError(t, epc.Procs[0].Start(t.Context()))

	names, rerr = fileutil.ReadDir(snapshotDir, fileutil.WithExt(fileExt))
	require.NoError(t, rerr)
	require.Empty(t, names)
}
