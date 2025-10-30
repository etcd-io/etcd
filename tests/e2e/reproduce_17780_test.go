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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/stringutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestReproduce17780 reproduces the issue: https://github.com/etcd-io/etcd/issues/17780.
func TestReproduce17780(t *testing.T) {
	e2e.BeforeTest(t)

	compactionBatchLimit := 10

	ctx := t.Context()
	clus, cerr := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(3),
		e2e.WithGoFailEnabled(true),
		e2e.WithSnapshotCount(1000),
		e2e.WithCompactionBatchLimit(compactionBatchLimit),
		e2e.WithWatchProcessNotifyInterval(100*time.Millisecond),
	)
	require.NoError(t, cerr)

	t.Cleanup(func() { clus.Stop() })

	leaderIdx := clus.WaitLeader(t)
	targetIdx := (leaderIdx + 1) % clus.Cfg.ClusterSize

	cli := newClient(t, clus.Procs[targetIdx].EndpointsGRPC(), e2e.ClientConfig{})

	// Revision: 2 -> 8 for new keys
	n := compactionBatchLimit - 2
	valueSize := 16
	for i := 2; i <= n; i++ {
		_, err := cli.Put(ctx, fmt.Sprintf("%d", i), stringutil.RandString(uint(valueSize)))
		require.NoError(t, err)
	}

	// Revision: 9 -> 11 for delete keys with compared revision
	//
	// We need last compaction batch is no-op and all the tombstones should
	// be deleted in previous compaction batch. So that we just lost the
	// finishedCompactRev after panic.
	for i := 9; i <= compactionBatchLimit+1; i++ {
		rev := i - 5
		key := fmt.Sprintf("%d", rev)

		_, err := cli.Delete(ctx, key)
		require.NoError(t, err)
	}

	require.NoError(t, clus.Procs[targetIdx].Failpoints().SetupHTTP(ctx, "compactBeforeSetFinishedCompact", `panic`))

	_, err := cli.Compact(ctx, 11, clientv3.WithCompactPhysical())
	require.Error(t, err)

	require.NoError(t, clus.Procs[targetIdx].Restart(ctx))

	// NOTE: We should not decrease the revision if there is no record
	// about finished compact operation.
	resp, err := cli.Get(ctx, fmt.Sprintf("%d", n))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Header.Revision, int64(11))

	// Revision 4 should be deleted by compaction.
	resp, err = cli.Get(ctx, fmt.Sprintf("%d", 4))
	require.NoError(t, err)
	require.Equal(t, int64(0), resp.Count)

	next := 20
	for i := 12; i <= next; i++ {
		_, err := cli.Put(ctx, fmt.Sprintf("%d", i), stringutil.RandString(uint(valueSize)))
		require.NoError(t, err)
	}

	expectedRevision := next
	for procIdx, proc := range clus.Procs {
		cli = newClient(t, proc.EndpointsGRPC(), e2e.ClientConfig{})
		resp, err := cli.Get(ctx, fmt.Sprintf("%d", next))
		require.NoError(t, err)

		assert.GreaterOrEqualf(t, resp.Header.Revision, int64(expectedRevision),
			"LeaderIdx: %d, Current: %d", leaderIdx, procIdx)
	}
}
