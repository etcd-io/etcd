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
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	clientv3 "go.etcd.io/etcd/client/v3"
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
					if !fileutil.Exist(e2e.BinPathLastRelease) {
						t.Skipf("%q does not exist", e2e.BinPathLastRelease)
					}
				}

				ctx := context.Background()

				cfg := e2e.EtcdProcessClusterConfig{
					ClusterSize:     3,
					IsClientAutoTLS: true,
					ClientTLS:       e2e.ClientTLS,
					Version:         scenario.clusterVersion,
				}

				clus, err := e2e.NewEtcdProcessCluster(t, &cfg)
				require.NoError(t, err)
				defer clus.Close()

				tombstoneRevs, lastestRev := populateDataForHashKV(t, clus, &cfg, scenario.keys)

				compactedOnRev := tombstoneRevs[0]

				// If compaction revision isn't a tombstone, select a revision in the middle of two tombstones.
				if !compactedOnTombstoneRev {
					compactedOnRev = (tombstoneRevs[0] + tombstoneRevs[1]) / 2
					require.Greater(t, tombstoneRevs[1], compactedOnRev)
				}

				cli := newClient(t, clus.EndpointsGRPC(), cfg.ClientTLS, cfg.IsClientAutoTLS)
				defer cli.Close()

				t.Logf("COMPACT on rev=%d", compactedOnRev)
				_, err = cli.Compact(ctx, compactedOnRev, clientv3.WithCompactPhysical())
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

	if !fileutil.Exist(e2e.BinPathLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPathLastRelease)
	}

	ctx := context.Background()

	cfg := e2e.EtcdProcessClusterConfig{
		ClusterSize:     3,
		IsClientAutoTLS: true,
		ClientTLS:       e2e.ClientTLS,
		Version:         e2e.QuorumLastVersion,
	}

	clus, err := e2e.NewEtcdProcessCluster(t, &cfg)
	require.NoError(t, err)
	defer clus.Close()

	t.Cleanup(func() { clus.Close() })

	tombstoneRevs, lastestRev := populateDataForHashKV(t, clus, &cfg, []string{"key0"})

	cli := newClient(t, clus.EndpointsGRPC(), cfg.ClientTLS, cfg.IsClientAutoTLS)
	defer cli.Close()

	firstCompactOnRev := tombstoneRevs[0]
	t.Logf("COMPACT rev=%d", firstCompactOnRev)
	_, err = cli.Compact(ctx, firstCompactOnRev, clientv3.WithCompactPhysical())
	require.NoError(t, err)

	secondCompactOnRev := tombstoneRevs[1]
	t.Logf("COMPACT rev=%d", secondCompactOnRev)
	_, err = cli.Compact(ctx, secondCompactOnRev, clientv3.WithCompactPhysical())
	require.NoError(t, err)

	for rev := secondCompactOnRev; rev <= lastestRev; rev++ {
		verifyConsistentHashKVAcrossAllMembers(t, cli, rev)
	}
}

func TestVerifyHashKVAfterCompactionOnLastTombstone_MixVersions(t *testing.T) {
	e2e.BeforeTest(t)

	if !fileutil.Exist(e2e.BinPathLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPathLastRelease)
	}

	for _, keys := range [][]string{
		[]string{"key0"},
		[]string{"key0", "key1"},
	} {
		t.Run(fmt.Sprintf("#%v", keys), func(t *testing.T) {
			ctx := context.Background()

			cfg := e2e.EtcdProcessClusterConfig{
				ClusterSize:     3,
				IsClientAutoTLS: true,
				ClientTLS:       e2e.ClientTLS,
				Version:         e2e.QuorumLastVersion,
			}

			clus, err := e2e.NewEtcdProcessCluster(t, &cfg)
			require.NoError(t, err)
			defer clus.Close()

			tombstoneRevs, lastestRev := populateDataForHashKV(t, clus, &cfg, keys)

			cli := newClient(t, clus.EndpointsGRPC(), cfg.ClientTLS, cfg.IsClientAutoTLS)
			defer cli.Close()

			compactOnRev := tombstoneRevs[len(tombstoneRevs)-1]
			t.Logf("COMPACT rev=%d", compactOnRev)
			_, err = cli.Compact(ctx, compactOnRev, clientv3.WithCompactPhysical())
			require.NoError(t, err)

			for rev := compactOnRev; rev <= lastestRev; rev++ {
				verifyConsistentHashKVAcrossAllMembers(t, cli, rev)
			}

		})
	}
}

// populateDataForHashKV populates some sample data, and return a slice of tombstone
// revisions and the latest revision
func populateDataForHashKV(t *testing.T, clus *e2e.EtcdProcessCluster, clusCfg *e2e.EtcdProcessClusterConfig, keys []string) ([]int64, int64) {
	c := newClient(t, clus.EndpointsGRPC(), clusCfg.ClientTLS, clusCfg.IsClientAutoTLS)
	defer c.Close()

	ctx := context.Background()
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

func verifyConsistentHashKVAcrossAllMembers(t *testing.T, cli *clientv3.Client, hashKVOnRev int64) {
	t.Logf("HashKV on rev=%d", hashKVOnRev)
	resp, err := hashKVs(cli.Endpoints(), cli)
	require.NoError(t, err)

	require.Greater(t, len(resp), 1)
	require.True(t, resp[0].Hash != 0)
	t.Logf("One Hash value is %d", resp[0].Hash)

	for i := 1; i < len(resp); i++ {
		require.Equal(t, resp[0].Hash, resp[i].Hash)
	}
}
