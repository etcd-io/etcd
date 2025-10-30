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
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestReproduce18667 reproduces the issue: https://github.com/etcd-io/etcd/issues/18667.
func TestReproduce18667(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := t.Context()
	clus, cerr := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(3),
		e2e.WithSnapshotCount(1000),
	)
	require.NoError(t, cerr)

	t.Cleanup(func() { clus.Stop() })

	targetIdx := rand.Intn(3)
	cli := newClient(t, clus.Procs[targetIdx].EndpointsGRPC(), e2e.ClientConfig{})

	t.Log("Put key-value [k1...k20]")
	var revision int64
	for i := 1; i <= 20; i++ {
		resp, err := cli.Put(ctx, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
		require.NoError(t, err)
		revision = resp.Header.Revision
	}

	t.Logf("Compact on the latest revision %d", revision)
	_, err := cli.Compact(ctx, revision)
	require.NoError(t, err)

	t.Log("Send txn to make data inconsistent among etcdservers")
	cmp := clientv3.Compare(clientv3.Value("k1"), "=", "v1")
	then := []clientv3.Op{
		clientv3.OpPut("k2", "foo"),
		clientv3.OpGet("k1", clientv3.WithRev(revision-1)),
	}
	_, err = cli.Txn(ctx).If(cmp).Then(then...).Commit()
	require.Error(t, err)
	require.Contains(t, err.Error(), rpctypes.ErrCompacted.Error())

	t.Log("Verify all members have consistent data on key 'k2'")
	for i := 0; i < clus.Cfg.ClusterSize; i++ {
		idx := (targetIdx + i) % clus.Cfg.ClusterSize
		cli = newClient(t, clus.Procs[idx].EndpointsGRPC(), e2e.ClientConfig{})
		resp, err := cli.Get(ctx, "k2")
		require.NoError(t, err)
		// TODO: All members are supposed to have the same key/value pair k2:v2.
		// However, the issue https://github.com/etcd-io/etcd/issues/18667
		// hasn't been resolved yet, so some members hold the wrong data
		// "k2:foo". Once the issue is fixed, we should remove the if-else
		// branch below, and all member should have consistent data k2:v2.
		if i == 0 {
			assert.Equal(t, "v2", string(resp.Kvs[0].Value))
		} else {
			assert.Equal(t, "foo", string(resp.Kvs[0].Value))
		}
	}
}
