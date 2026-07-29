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

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestRaftAsyncStorageWrites(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := t.Context()
	clus, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(3),
		e2e.WithSnapshotCount(20),
		e2e.WithSnapshotCatchUpEntries(5),
		e2e.WithServerFeatureGate("RaftAsyncStorageWrites", true),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clus.Close()) })

	leader := clus.WaitLeader(t)
	lagging := (leader + 1) % len(clus.Procs)
	active := (leader + 2) % len(clus.Procs)
	require.NoError(t, clus.Procs[lagging].Stop())

	client := newClient(t, clus.Procs[active].EndpointsGRPC(), e2e.ClientConfig{})
	defer client.Close()
	for i := 0; i < 100; i++ {
		_, err = client.Put(ctx, fmt.Sprintf("async-key-%03d", i), fmt.Sprintf("value-%03d", i))
		require.NoError(t, err)
	}

	require.NoError(t, clus.Procs[lagging].Restart(ctx))
	require.Eventually(t, func() bool {
		healthCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return clus.Procs[lagging].Etcdctl().Health(healthCtx) == nil
	}, 30*time.Second, 100*time.Millisecond)

	laggingClient := newClient(t, clus.Procs[lagging].EndpointsGRPC(), e2e.ClientConfig{})
	defer laggingClient.Close()
	response, err := laggingClient.Get(ctx, "async-key-099")
	require.NoError(t, err)
	require.Len(t, response.Kvs, 1)
	require.Equal(t, "value-099", string(response.Kvs[0].Value))
}
