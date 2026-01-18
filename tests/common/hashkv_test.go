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

package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestVerifyHashKVAfterCompact tests that HashKV is consistent across all members after a physical compaction.
// It tests both cases where the compaction is on a tombstone revision and where it is not.
func TestVerifyHashKVAfterCompact(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, keys := range [][]string{
		{"key0"},
		{"key0", "key1"},
	} {
		for _, compactedOnTombstoneRev := range []bool{false, true} {
			t.Run(fmt.Sprintf("Keys=%v/CompactedOnTombstone=%v", keys, compactedOnTombstoneRev), func(t *testing.T) {
				for _, tc := range clusterTestCases() {
					t.Run(tc.name, func(t *testing.T) {
						if tc.config.ClusterSize < 2 {
							t.Skip("Skipping test for single-member cluster")
						}
						ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
						defer cancel()

						clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
						defer clus.Close()

						cc := testutils.MustClient(clus.Client())
						tombstoneRevs, latestRev := populateDataForHashKV(t, cc, keys)

						compactedOnRev := tombstoneRevs[0]

						// If compaction revision isn't a tombstone, select a revision in the middle of two tombstones.
						if !compactedOnTombstoneRev {
							require.Greater(t, len(tombstoneRevs), 1)
							compactedOnRev = (tombstoneRevs[0] + tombstoneRevs[1]) / 2
							require.Greater(t, compactedOnRev, tombstoneRevs[0])
							require.Greater(t, tombstoneRevs[1], compactedOnRev)
						}

						_, err := cc.Compact(ctx, compactedOnRev, config.CompactOption{Physical: true})
						require.NoError(t, err)

						for rev := compactedOnRev; rev <= latestRev; rev++ {
							verifyConsistentHashKVAcrossAllMembers(t, cc, rev)
						}
					})
				}
			})
		}
	}
}

// TestVerifyHashKVAfterTwoCompactsOnTombstone tests that HashKV is consistent
// across all members after two physical compactions on tombstone revisions.
func TestVerifyHashKVAfterTwoCompactsOnTombstone(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.config.ClusterSize < 2 {
				t.Skip("Skipping test for single-member cluster")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()

			cc := testutils.MustClient(clus.Client())
			tombstoneRevs, latestRev := populateDataForHashKV(t, cc, []string{"key0"})
			require.GreaterOrEqual(t, len(tombstoneRevs), 2)

			firstCompactOnRev := tombstoneRevs[0]
			t.Logf("COMPACT rev=%d", firstCompactOnRev)
			_, err := cc.Compact(ctx, firstCompactOnRev, config.CompactOption{Physical: true})
			require.NoError(t, err)

			secondCompactOnRev := tombstoneRevs[1]
			t.Logf("COMPACT rev=%d", secondCompactOnRev)
			_, err = cc.Compact(ctx, secondCompactOnRev, config.CompactOption{Physical: true})
			require.NoError(t, err)

			for rev := secondCompactOnRev; rev <= latestRev; rev++ {
				verifyConsistentHashKVAcrossAllMembers(t, cc, rev)
			}
		})
	}
}

// TestVerifyHashKVAfterCompactOnLastTombstone tests that HashKV is consistent
// across all members after a physical compaction on the last tombstone revision.
func TestVerifyHashKVAfterCompactOnLastTombstone(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, keys := range [][]string{
		{"key0"},
		{"key0", "key1"},
	} {
		t.Run(fmt.Sprintf("Keys=%v", keys), func(t *testing.T) {
			for _, tc := range clusterTestCases() {
				t.Run(tc.name, func(t *testing.T) {
					if tc.config.ClusterSize < 2 {
						t.Skip("Skipping test for single-member cluster")
					}

					ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
					defer cancel()

					clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
					defer clus.Close()

					cc := testutils.MustClient(clus.Client())
					tombstoneRevs, latestRev := populateDataForHashKV(t, cc, keys)
					require.NotEmpty(t, tombstoneRevs)

					compactOnRev := tombstoneRevs[len(tombstoneRevs)-1]
					t.Logf("COMPACT rev=%d", compactOnRev)
					_, err := cc.Compact(ctx, compactOnRev, config.CompactOption{Physical: true})
					require.NoError(t, err)

					for rev := compactOnRev; rev <= latestRev; rev++ {
						verifyConsistentHashKVAcrossAllMembers(t, cc, rev)
					}
				})
			}
		})
	}
}

// TestVerifyHashKVAfterCompactAndDefrag tests that HashKV is consistent
// within a member before and after a physical compaction and defragmentation.
func TestVerifyHashKVAfterCompactAndDefrag(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()

			cc := testutils.MustClient(clus.Client())
			tombstoneRevs, _ := populateDataForHashKV(t, cc, []string{"key0"})
			require.NotEmpty(t, tombstoneRevs)

			compactOnRev := tombstoneRevs[0]
			t.Logf("COMPACT rev=%d", compactOnRev)

			before, err := cc.HashKV(ctx, compactOnRev)
			require.NoError(t, err)

			_, err = cc.Compact(ctx, compactOnRev, config.CompactOption{Physical: true})
			require.NoError(t, err)

			err = cc.Defragment(ctx, config.DefragOption{})
			require.NoError(t, err)

			after, err := cc.HashKV(ctx, compactOnRev)
			require.NoError(t, err)

			require.Len(t, before, len(after))
			for i := range before {
				assert.Equal(t, before[i].Hash, after[i].Hash)
			}
		})
	}
}

// populateDataForHashKV populates some sample data, and return a slice of tombstone
// revisions and the latest revision.
func populateDataForHashKV(t *testing.T, cc intf.Client, keys []string) ([]int64, int64) {
	t.Helper()
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
			resp, derr := cc.Delete(ctx, keys[0], config.DeleteOptions{})
			require.NoError(t, derr)
			latestRev = resp.Header.Revision
			tombStoneRevs = append(tombStoneRevs, resp.Header.Revision)
			continue
		}

		value := fmt.Sprintf("%d", i)
		var ops []string
		for _, key := range keys {
			ops = append(ops, fmt.Sprintf("put %s %s", key, value))
		}
		t.Logf("Writing keys: %v, value: %s", keys, value)
		resp, terr := cc.Txn(ctx, nil, ops, nil, config.TxnOptions{Interactive: true})
		require.NoError(t, terr)
		require.True(t, resp.Succeeded)
		require.Len(t, resp.Responses, len(ops))
		latestRev = resp.Header.Revision
	}
	return tombStoneRevs, latestRev
}

func verifyConsistentHashKVAcrossAllMembers(t *testing.T, cc intf.Client, hashKVOnRev int64) {
	t.Helper()
	ctx := t.Context()

	t.Logf("HashKV on rev=%d", hashKVOnRev)

	assert.Eventually(t, func() bool {
		resp, err := cc.HashKV(ctx, hashKVOnRev)
		if err != nil {
			t.Logf("HashKV failed: %v", err)
			return false
		}

		// Ensure that there are multiple members in the cluster.
		if len(resp) <= 1 {
			t.Logf("Expected multiple members, got %d", len(resp))
			return false
		}

		// Check if all members have the same hash
		for i := 1; i < len(resp); i++ {
			if resp[i].Hash != resp[0].Hash {
				t.Logf("There is a chance that the physical compaction is not yet completed on the followers: Leader hash=%d, Follower hash=%d", resp[0].Hash, resp[i].Hash)
				return false
			}
		}
		return true
	}, 3*time.Second, 100*time.Millisecond)
}
