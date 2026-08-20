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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
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
