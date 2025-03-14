// Copyright 2022 The etcd Authors
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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestLeaseGrantTimeToLive(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				ttl := int64(10)
				leaseResp, err := cc.Grant(ctx, ttl)
				require.NoError(t, err)

				ttlResp, err := cc.TimeToLive(ctx, leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				require.Equal(t, ttl, ttlResp.GrantedTTL)
			})
		})
	}
}

func TestLeaseGrantAndList(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		nestedCases := []struct {
			name       string
			leaseCount int
		}{
			{
				name:       "no_leases",
				leaseCount: 0,
			},
			{
				name:       "one_lease",
				leaseCount: 1,
			},
			{
				name:       "many_leases",
				leaseCount: 3,
			},
		}

		for _, nc := range nestedCases {
			t.Run(tc.name+"/"+nc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				t.Logf("Creating cluster...")
				clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
				defer clus.Close()
				cc := testutils.MustClient(clus.Client())
				t.Logf("Created cluster and client")
				testutils.ExecuteUntil(ctx, t, func() {
					var createdLeases []clientv3.LeaseID
					for i := 0; i < nc.leaseCount; i++ {
						leaseResp, err := cc.Grant(ctx, 10)
						t.Logf("Grant returned: resp:%s err:%v", leaseResp.String(), err)
						require.NoError(t, err)
						createdLeases = append(createdLeases, leaseResp.ID)
					}

					// Because we're not guaranteed to talk to the same member, wait for
					// listing to eventually return true, either by the result propagating
					// or by hitting an up to date member.
					var leases []clientv3.LeaseStatus
					require.Eventually(t, func() bool {
						resp, err := cc.Leases(ctx)
						if err != nil {
							return false
						}
						leases = resp.Leases
						// TODO: update this to use last Revision from leaseResp
						// after https://github.com/etcd-io/etcd/issues/13989 is fixed
						return len(leases) == len(createdLeases)
					}, 2*time.Second, 10*time.Millisecond)

					returnedLeases := make([]clientv3.LeaseID, 0, nc.leaseCount)
					for _, status := range leases {
						returnedLeases = append(returnedLeases, status.ID)
					}

					require.ElementsMatch(t, createdLeases, returnedLeases)
				})
			})
		}
	}
}

func TestLeaseGrantTimeToLiveExpired(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				leaseResp, err := cc.Grant(ctx, 2)
				require.NoError(t, err)

				err = cc.Put(ctx, "foo", "bar", config.PutOptions{LeaseID: leaseResp.ID})
				require.NoError(t, err)

				getResp, err := cc.Get(ctx, "foo", config.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, int64(1), getResp.Count)

				// FIXME: When leader changes, old leader steps
				// back to follower and ignores the lease revoking.
				// The new leader will restart TTL counting. If so,
				// we should call time.Sleep again and wait for revoking.
				// It can't avoid flakey but reduce flakey possiblility.
				for i := 0; i < 3; i++ {
					currentLeader := clus.WaitLeader(t)
					t.Logf("[%d] current leader index %d", i, currentLeader)

					time.Sleep(3 * time.Second)

					newLeader := clus.WaitLeader(t)
					if newLeader == currentLeader {
						break
					}
					t.Logf("[%d] leader changed, new leader index %d", i, newLeader)
				}

				ttlResp, err := cc.TimeToLive(ctx, leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				require.Equal(t, int64(-1), ttlResp.TTL)

				getResp, err = cc.Get(ctx, "foo", config.GetOptions{})
				require.NoError(t, err)
				// Value should expire with the lease
				require.Equal(t, int64(0), getResp.Count)
			})
		})
	}
}

func TestLeaseGrantKeepAliveOnce(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				leaseResp, err := cc.Grant(ctx, 2)
				require.NoError(t, err)

				_, err = cc.KeepAliveOnce(ctx, leaseResp.ID)
				require.NoError(t, err)

				// FIXME: When leader changes, old leader steps
				// back to follower and ignores the lease revoking.
				// The new leader will restart TTL counting. If so,
				// we should call time.Sleep again and wait for revoking.
				// It can't avoid flakey but reduce flakey possiblility.
				for i := 0; i < 3; i++ {
					currentLeader := clus.WaitLeader(t)
					t.Logf("[%d] current leader index %d", i, currentLeader)

					time.Sleep(2 * time.Second)

					newLeader := clus.WaitLeader(t)
					if newLeader == currentLeader {
						break
					}
					t.Logf("[%d] leader changed, new leader index %d", i, newLeader)
				}

				ttlResp, err := cc.TimeToLive(ctx, leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				// We still have a lease!
				require.Greater(t, int64(2), ttlResp.TTL)
			})
		})
	}
}

func TestLeaseGrantRevoke(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				leaseResp, err := cc.Grant(ctx, 20)
				require.NoError(t, err)

				err = cc.Put(ctx, "foo", "bar", config.PutOptions{LeaseID: leaseResp.ID})
				require.NoError(t, err)

				getResp, err := cc.Get(ctx, "foo", config.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, int64(1), getResp.Count)

				_, err = cc.Revoke(ctx, leaseResp.ID)
				require.NoError(t, err)

				ttlResp, err := cc.TimeToLive(ctx, leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				require.Equal(t, int64(-1), ttlResp.TTL)

				getResp, err = cc.Get(ctx, "foo", config.GetOptions{})
				require.NoError(t, err)
				// Value should expire with the lease
				require.Equal(t, int64(0), getResp.Count)
			})
		})
	}
}
