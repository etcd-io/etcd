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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestLeaseGrantTimeToLive(t *testing.T) {
	testRunner.BeforeTest(t)

	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				ttl := int64(10)
				leaseResp, err := cc.Grant(ttl)
				require.NoError(t, err)

				ttlResp, err := cc.TimeToLive(leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				require.Equal(t, ttl, ttlResp.GrantedTTL)
			})
		})
	}
}

func TestLeaseGrantAndList(t *testing.T) {
	testRunner.BeforeTest(t)

	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
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
				t.Logf("Creating cluster...")
				clus := testRunner.NewCluster(t, tc.config)
				defer clus.Close()
				cc := clus.Client()
				t.Logf("Created cluster and client")
				testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
					createdLeases := []clientv3.LeaseID{}
					lastRev := int64(0)
					for i := 0; i < nc.leaseCount; i++ {
						leaseResp, err := cc.Grant(10)
						t.Logf("Grant returned: resp:%s err:%v", leaseResp.String(), err)
						require.NoError(t, err)
						createdLeases = append(createdLeases, leaseResp.ID)
						lastRev = leaseResp.GetRevision()
					}

					// Because we're not guarunteed to talk to the same member, wait for
					// listing to eventually return true, either by the result propagaing
					// or by hitting an up to date member.
					leases := []clientv3.LeaseStatus{}
					require.Eventually(t, func() bool {
						resp, err := cc.LeaseList()
						if err != nil {
							return false
						}
						leases = resp.Leases
						return resp.GetRevision() >= lastRev
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

	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				leaseResp, err := cc.Grant(2)
				require.NoError(t, err)

				err = cc.Put("foo", "bar", config.PutOptions{LeaseID: leaseResp.ID})
				require.NoError(t, err)

				getResp, err := cc.Get("foo", config.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, int64(1), getResp.Count)

				time.Sleep(3 * time.Second)

				ttlResp, err := cc.TimeToLive(leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				require.Equal(t, int64(-1), ttlResp.TTL)

				getResp, err = cc.Get("foo", config.GetOptions{})
				require.NoError(t, err)
				// Value should expire with the lease
				require.Equal(t, int64(0), getResp.Count)
			})
		})
	}
}

func TestLeaseGrantKeepAliveOnce(t *testing.T) {
	testRunner.BeforeTest(t)

	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				leaseResp, err := cc.Grant(2)
				require.NoError(t, err)

				_, err = cc.LeaseKeepAliveOnce(leaseResp.ID)
				require.NoError(t, err)

				time.Sleep(2 * time.Second) // Wait for the original lease to expire

				ttlResp, err := cc.TimeToLive(leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				// We still have a lease!
				require.Greater(t, int64(2), ttlResp.TTL)
			})
		})
	}
}

func TestLeaseGrantRevoke(t *testing.T) {
	testRunner.BeforeTest(t)

	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				leaseResp, err := cc.Grant(20)
				require.NoError(t, err)

				err = cc.Put("foo", "bar", config.PutOptions{LeaseID: leaseResp.ID})
				require.NoError(t, err)

				getResp, err := cc.Get("foo", config.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, int64(1), getResp.Count)

				_, err = cc.LeaseRevoke(leaseResp.ID)
				require.NoError(t, err)

				ttlResp, err := cc.TimeToLive(leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				require.Equal(t, int64(-1), ttlResp.TTL)

				getResp, err = cc.Get("foo", config.GetOptions{})
				require.NoError(t, err)
				// Value should expire with the lease
				require.Equal(t, int64(0), getResp.Count)
			})
		})
	}
}
