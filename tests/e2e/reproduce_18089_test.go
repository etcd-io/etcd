// Copyright 2025 The etcd Authors
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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestDeleteEventDrop_Issue18089 is an e2e test to reproduce the issue reported in: https://github.com/etcd-io/etcd/issues/18089
//
// The goal is to reproduce a DELETE event being dropped in a watch after a compaction
// occurs on the revision where the deletion took place. In order to reproduce this, we
// perform the following sequence (steps for reproduction thanks to @ahrtr):
//   - PUT k v2 (assume returned revision = r2)
//   - PUT k v3 (assume returned revision = r3)
//   - PUT k v4 (assume returned revision = r4)
//   - DELETE k (assume returned revision = r5)
//   - PUT k v6 (assume returned revision = r6)
//   - COMPACT r5
//   - WATCH rev=r5
//
// We should get the DELETE event (r5) followed by the PUT event (r6).

func TestDeleteEventDrop_Issue18089(t *testing.T) {
	e2e.BeforeTest(t)
	cfg := e2e.DefaultConfig()
	cfg.ClusterSize = 1
	cfg.Client = e2e.ClientConfig{ConnectionType: e2e.ClientTLS}
	clus, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(cfg))
	require.NoError(t, err)
	defer clus.Close()

	c := newClient(t, clus.EndpointsGRPC(), cfg.Client)
	defer c.Close()

	ctx := t.Context()
	const (
		key = "k"
		v2  = "v2"
		v3  = "v3"
		v4  = "v4"
		v6  = "v6"
	)

	t.Logf("PUT key=%s, val=%s", key, v2)
	_, err = c.KV.Put(ctx, key, v2)
	require.NoError(t, err)

	t.Logf("PUT key=%s, val=%s", key, v3)
	_, err = c.KV.Put(ctx, key, v3)
	require.NoError(t, err)

	t.Logf("PUT key=%s, val=%s", key, v4)
	_, err = c.KV.Put(ctx, key, v4)
	require.NoError(t, err)

	t.Logf("DELETE key=%s", key)
	deleteResp, err := c.KV.Delete(ctx, key)
	require.NoError(t, err)

	t.Logf("PUT key=%s, val=%s", key, v6)
	_, err = c.KV.Put(ctx, key, v6)
	require.NoError(t, err)

	t.Logf("COMPACT rev=%d", deleteResp.Header.Revision)
	_, err = c.KV.Compact(ctx, deleteResp.Header.Revision, clientv3.WithCompactPhysical())
	require.NoError(t, err)

	watchChan := c.Watch(ctx, key, clientv3.WithRev(deleteResp.Header.Revision))
	select {
	case watchResp := <-watchChan:
		require.Len(t, watchResp.Events, 2)

		require.Equal(t, mvccpb.DELETE, watchResp.Events[0].Type)
		deletedKey := string(watchResp.Events[0].Kv.Key)
		require.Equal(t, key, deletedKey)

		require.Equal(t, mvccpb.PUT, watchResp.Events[1].Type)

		updatedKey := string(watchResp.Events[1].Kv.Key)
		require.Equal(t, key, updatedKey)

		require.Equal(t, v6, string(watchResp.Events[1].Kv.Value))
	case <-time.After(100 * time.Millisecond):
		// we care only about the first response, but have an
		// escape hatch in case the watch response is delayed.
		t.Fatal("timed out getting watch response")
	}
}
