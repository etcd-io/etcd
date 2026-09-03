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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestReproduce22376 reproduces the issue: https://github.com/etcd-io/etcd/issues/22376
//
// A key that is deleted and re-created within a single main revision must keep
// its value when the store is compacted at exactly that revision. Before the
// fix the live value was deleted from the backend while the index still
// referenced it, so the next range hit the "range failed to find revision pair"
// fatal and the restarted member served an empty keyspace.
func TestReproduce22376(t *testing.T) {
	e2e.BeforeTest(t)
	// bound the context: if the member dies on the fatal below, the client
	// would otherwise retry until the test binary times out
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	clus, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithClusterSize(1))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, clus.Stop()) })

	cli := newClient(t, clus.EndpointsGRPC(), e2e.ClientConfig{})

	keys := []string{"cfg/a", "cfg/b", "cfg/c"}
	for _, k := range keys {
		_, err = cli.Put(ctx, k, "v1")
		require.NoError(t, err)
	}

	// Rewrite the whole keyspace in one transaction, so that every key gets a
	// tombstone and its re-creation in the same main revision.
	ops := []clientv3.Op{clientv3.OpDelete("\x00", clientv3.WithFromKey())}
	for _, k := range keys {
		ops = append(ops, clientv3.OpPut(k, "v2"))
	}
	txnResp, err := cli.Txn(ctx).Then(ops...).Commit()
	require.NoError(t, err)

	_, err = cli.Compact(ctx, txnResp.Header.Revision, clientv3.WithCompactPhysical())
	require.NoError(t, err)

	assertKeys := func(t *testing.T) {
		t.Helper()
		resp, gerr := cli.Get(ctx, "cfg/", clientv3.WithPrefix())
		require.NoError(t, gerr)
		require.Len(t, resp.Kvs, len(keys))
		for _, kv := range resp.Kvs {
			require.Equal(t, []byte("v2"), kv.Value)
		}
	}

	assertKeys(t)

	// the backend must still hold enough to rebuild the index on restart
	require.NoError(t, clus.Procs[0].Restart(ctx))
	assertKeys(t)
}
