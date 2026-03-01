// Copyright 2026 The etcd Authors
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

//go:build !cluster_proxy

package e2e

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestIssue20732 reproduces etcd-io/etcd#20732 where repeated crashes during
// snapshot creation could accumulate *.snap.broken files and exhaust disk space.
func TestIssue20732(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := t.Context()

	cfg := e2e.NewConfig(
		e2e.WithClusterSize(1),
		e2e.WithSnapshotCount(1), // snapshot very frequently
		e2e.WithKeepDataDir(true),
		e2e.WithLogLevel("debug"),
	)

	epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithConfig(cfg))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, epc.Close())
	}()

	proc := epc.Procs[0]

	t.Log("Step 1: Repeatedly trigger snapshot and crash etcd")

	for round := 0; round < 5; round++ {
		t.Logf("Round %d: write data to trigger snapshot", round)

		for i := 0; i < 10; i++ {
			_, err := proc.Etcdctl().Put(
				ctx,
				fmt.Sprintf("key-%d-%d", round, i),
				"value",
				config.PutOptions{},
			)
			require.NoError(t, err)
		}

		t.Log("Killing etcd process abruptly")
		require.NoError(t, proc.Kill())

		t.Log("Restarting etcd with same data-dir")
		require.NoError(t, proc.Restart(ctx))
	}

	t.Log("Step 2: Verify broken snapshots do not accumulate")

	snapDir := filepath.Join(
		proc.Config().DataDirPath,
		"member",
		"snap",
	)

	broken, err := filepath.Glob(filepath.Join(snapDir, "*.snap.broken"))
	require.NoError(t, err)

	require.LessOrEqual(
		t,
		len(broken),
		1,
		"expected at most one broken snapshot, found %d: %v",
		len(broken),
		broken,
	)
}
