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

//go:build !cluster_proxy

package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestVerifyHashKVAfterCompact(t *testing.T) {
	scenarios := []struct {
		clusterVersion e2e.ClusterVersion
		keys           []string // used for data generators
	}{
		{
			clusterVersion: e2e.CurrentVersion,
			keys:           []string{"key0"},
		},
		{
			clusterVersion: e2e.CurrentVersion,
			keys:           []string{"key0", "key1"},
		},
		{
			clusterVersion: e2e.QuorumLastVersion,
			keys:           []string{"key0"},
		},
		{
			clusterVersion: e2e.QuorumLastVersion,
			keys:           []string{"key0", "key1"},
		},
	}

	for _, compactedOnTombstoneRev := range []bool{false, true} {
		for _, scenario := range scenarios {
			t.Run(fmt.Sprintf("compactedOnTombstone=%v - %s - Keys=%v", compactedOnTombstoneRev, scenario.clusterVersion, scenario.keys), func(t *testing.T) {
				e2e.BeforeTest(t)

				if scenario.clusterVersion != e2e.CurrentVersion {
					if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
						t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
					}
				}

				ctx := t.Context()

				cfg := e2e.NewConfigClientTLS()
				clus, err := e2e.NewEtcdProcessCluster(ctx, t,
					e2e.WithConfig(cfg),
					e2e.WithClusterSize(3),
					e2e.WithVersion(scenario.clusterVersion))
				require.NoError(t, err)

				t.Cleanup(func() { clus.Close() })

				tombstoneRevs, lastestRev := populateDataForHashKV(t, clus, cfg.Client, scenario.keys)

				compactedOnRev := tombstoneRevs[0]

				// If compaction revision isn't a tombstone, select a revision in the middle of two tombstones.
				if !compactedOnTombstoneRev {
					compactedOnRev = (tombstoneRevs[0] + tombstoneRevs[1]) / 2
					require.Greater(t, compactedOnRev, tombstoneRevs[0])
					require.Greater(t, tombstoneRevs[1], compactedOnRev)
				}

				cli, err := e2e.NewEtcdctl(cfg.Client, clus.EndpointsGRPC())
				require.NoError(t, err)

				t.Logf("COMPACT on rev=%d", compactedOnRev)
				_, err = cli.Compact(ctx, compactedOnRev, config.CompactOption{Physical: true})
				require.NoError(t, err)

				for rev := compactedOnRev; rev <= lastestRev; rev++ {
					verifyConsistentHashKVAcrossAllMembers(t, cli, rev)
				}
			})
		}
	}
}

func TestVerifyHashKVAfterTwoCompactionsOnTombstone_MixVersions(t *testing.T) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	ctx := t.Context()

	cfg := e2e.NewConfigClientTLS()
	clus, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithConfig(cfg),
		e2e.WithClusterSize(3),
		e2e.WithVersion(e2e.QuorumLastVersion))
	require.NoError(t, err)
	t.Cleanup(func() { clus.Close() })

	tombstoneRevs, lastestRev := populateDataForHashKV(t, clus, cfg.Client, []string{"key0"})

	cli, err := e2e.NewEtcdctl(cfg.Client, clus.EndpointsGRPC())
	require.NoError(t, err)

	firstCompactOnRev := tombstoneRevs[0]
	t.Logf("COMPACT rev=%d", firstCompactOnRev)
	_, err = cli.Compact(ctx, firstCompactOnRev, config.CompactOption{Physical: true})
	require.NoError(t, err)

	secondCompactOnRev := tombstoneRevs[1]
	t.Logf("COMPACT rev=%d", secondCompactOnRev)
	_, err = cli.Compact(ctx, secondCompactOnRev, config.CompactOption{Physical: true})
	require.NoError(t, err)

	for rev := secondCompactOnRev; rev <= lastestRev; rev++ {
		verifyConsistentHashKVAcrossAllMembers(t, cli, rev)
	}
}

func TestVerifyHashKVAfterCompactionOnLastTombstone_MixVersions(t *testing.T) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPath.EtcdLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPath.EtcdLastRelease)
	}

	for _, keys := range [][]string{
		{"key0"},
		{"key0", "key1"},
	} {
		t.Run(fmt.Sprintf("#%v", keys), func(t *testing.T) {
			ctx := t.Context()

			cfg := e2e.NewConfigClientTLS()
			clus, err := e2e.NewEtcdProcessCluster(ctx, t,
				e2e.WithConfig(cfg),
				e2e.WithClusterSize(3),
				e2e.WithVersion(e2e.QuorumLastVersion))
			require.NoError(t, err)
			t.Cleanup(func() { clus.Close() })

			tombstoneRevs, lastestRev := populateDataForHashKV(t, clus, cfg.Client, keys)

			cli, err := e2e.NewEtcdctl(cfg.Client, clus.EndpointsGRPC())
			require.NoError(t, err)

			compactOnRev := tombstoneRevs[len(tombstoneRevs)-1]
			t.Logf("COMPACT rev=%d", compactOnRev)
			_, err = cli.Compact(ctx, compactOnRev, config.CompactOption{Physical: true})
			require.NoError(t, err)

			for rev := compactOnRev; rev <= lastestRev; rev++ {
				verifyConsistentHashKVAcrossAllMembers(t, cli, rev)
			}
		})
	}
}

// populateDataForHashKV populates some sample data, and return a slice of tombstone
// revisions and the latest revision
func populateDataForHashKV(t *testing.T, clus *e2e.EtcdProcessCluster, clientCfg e2e.ClientConfig, keys []string) ([]int64, int64) {
	c := newClient(t, clus.EndpointsGRPC(), clientCfg)
	defer c.Close()

	ctx := t.Context()
	totalOperations := 40

	var (
		tombStoneRevs []int64
		latestRev     int64
	)

	deleteStep := 10 // submit a delete operation on every 10 operations
	for i := 1; i <= totalOperations; i++ {
		if i%deleteStep == 0 {
			t.Logf("Deleting key=%s", keys[0]) // Only delete the first key for simplicity
			resp, derr := c.Delete(ctx, keys[0])
			require.NoError(t, derr)
			latestRev = resp.Header.Revision
			tombStoneRevs = append(tombStoneRevs, resp.Header.Revision)
			continue
		}

		value := fmt.Sprintf("%d", i)
		var ops []clientv3.Op
		for _, key := range keys {
			ops = append(ops, clientv3.OpPut(key, value))
		}

		t.Logf("Writing keys: %v, value: %s", keys, value)
		resp, terr := c.Txn(ctx).Then(ops...).Commit()
		require.NoError(t, terr)
		require.True(t, resp.Succeeded)
		require.Len(t, resp.Responses, len(ops))
		latestRev = resp.Header.Revision
	}
	return tombStoneRevs, latestRev
}

func verifyConsistentHashKVAcrossAllMembers(t *testing.T, cli *e2e.EtcdctlV3, hashKVOnRev int64) {
	ctx := t.Context()

	t.Logf("HashKV on rev=%d", hashKVOnRev)
	resp, err := cli.HashKV(ctx, hashKVOnRev)
	require.NoError(t, err)

	require.Greater(t, len(resp), 1)
	require.NotEqual(t, 0, resp[0].Hash)
	t.Logf("One Hash value is %d", resp[0].Hash)

	for i := 1; i < len(resp); i++ {
		require.Equal(t, resp[0].Hash, resp[i].Hash)
	}
}
