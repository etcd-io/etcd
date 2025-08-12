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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/config"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestMemberList(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				resp, err := cc.MemberList(ctx, false)
				require.NoErrorf(t, err, "could not get member list")
				expectNum := len(clus.Members())
				require.Lenf(t, resp.Members, expectNum, "unexpected number of members")
				assert.Eventually(t, func() bool {
					resp, err := cc.MemberList(ctx, false)
					if err != nil {
						t.Logf("Failed to get member list, err: %v", err)
						return false
					}
					for _, m := range resp.Members {
						if len(m.ClientURLs) == 0 {
							t.Logf("member is not started, memberID:%d, memberName:%s", m.ID, m.Name)
							return false
						}
					}
					return true
				}, time.Second*5, time.Millisecond*100)
			})
		})
	}
}

func TestMemberAdd(t *testing.T) {
	testRunner.BeforeTest(t)

	learnerTcs := []struct {
		name    string
		learner bool
	}{
		{
			name:    "NotLearner",
			learner: false,
		},
		{
			name:    "Learner",
			learner: true,
		},
	}

	quorumTcs := []struct {
		name                string
		strictReconfigCheck bool
		waitForQuorum       bool
		expectError         bool
	}{
		{
			name:                "StrictReconfigCheck/WaitForQuorum",
			strictReconfigCheck: true,
			waitForQuorum:       true,
		},
		{
			name:                "StrictReconfigCheck/NoWaitForQuorum",
			strictReconfigCheck: true,
			expectError:         true,
		},
		{
			name:          "DisableStrictReconfigCheck/WaitForQuorum",
			waitForQuorum: true,
		},
		{
			name: "DisableStrictReconfigCheck/NoWaitForQuorum",
		},
	}

	for _, learnerTc := range learnerTcs {
		for _, quorumTc := range quorumTcs {
			for _, clusterTc := range clusterTestCases() {
				t.Run(learnerTc.name+"/"+quorumTc.name+"/"+clusterTc.name, func(t *testing.T) {
					ctxTimeout := 10 * time.Second
					if quorumTc.waitForQuorum {
						ctxTimeout += etcdserver.HealthInterval
					}
					ctx, cancel := context.WithTimeout(t.Context(), ctxTimeout)
					defer cancel()
					c := clusterTc.config
					c.StrictReconfigCheck = quorumTc.strictReconfigCheck
					clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(c))
					defer clus.Close()
					cc := testutils.MustClient(clus.Client())

					testutils.ExecuteUntil(ctx, t, func() {
						var addResp *clientv3.MemberAddResponse
						var err error
						if quorumTc.waitForQuorum {
							time.Sleep(etcdserver.HealthInterval)
						}
						if learnerTc.learner {
							addResp, err = cc.MemberAddAsLearner(ctx, "newmember", []string{"http://localhost:123"})
						} else {
							addResp, err = cc.MemberAdd(ctx, "newmember", []string{"http://localhost:123"})
						}
						if quorumTc.expectError && c.ClusterSize > 1 {
							// calling MemberAdd/MemberAddAsLearner on a single node will not fail,
							// whether strictReconfigCheck or whether waitForQuorum
							require.ErrorContains(t, err, "etcdserver: unhealthy cluster")
						} else {
							require.NoErrorf(t, err, "MemberAdd failed")
							require.NotNilf(t, addResp.Member, "MemberAdd failed, expected: member != nil, got: member == nil")
							require.NotZerof(t, addResp.Member.ID, "MemberAdd failed, expected: ID != 0, got: ID == 0")
							require.NotEmptyf(t, addResp.Member.PeerURLs, "MemberAdd failed, expected: non-empty PeerURLs, got: empty PeerURLs")
						}
					})
				})
			}
		}
	}
}

func TestMemberRemove(t *testing.T) {
	testRunner.BeforeTest(t)

	tcs := []struct {
		name                  string
		strictReconfigCheck   bool
		waitForQuorum         bool
		expectSingleNodeError bool
		expectClusterError    bool
	}{
		{
			name:                  "StrictReconfigCheck/WaitForQuorum",
			strictReconfigCheck:   true,
			waitForQuorum:         true,
			expectSingleNodeError: true,
		},
		{
			name:                  "StrictReconfigCheck/NoWaitForQuorum",
			strictReconfigCheck:   true,
			expectSingleNodeError: true,
			expectClusterError:    true,
		},
		{
			name:          "DisableStrictReconfigCheck/WaitForQuorum",
			waitForQuorum: true,
		},
		{
			name: "DisableStrictReconfigCheck/NoWaitForQuorum",
		},
	}

	for _, quorumTc := range tcs {
		for _, clusterTc := range clusterTestCases() {
			if !quorumTc.strictReconfigCheck && clusterTc.config.ClusterSize == 1 {
				// skip these test cases
				// when strictReconfigCheck is disabled, calling MemberRemove will cause the single node to panic
				continue
			}
			t.Run(quorumTc.name+"/"+clusterTc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 14*time.Second)
				defer cancel()
				c := clusterTc.config
				c.StrictReconfigCheck = quorumTc.strictReconfigCheck
				clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(c))
				defer clus.Close()
				// client connects to a specific member which won't be removed from cluster
				cc := clus.Members()[0].Client()

				testutils.ExecuteUntil(ctx, t, func() {
					if quorumTc.waitForQuorum {
						// wait for health interval + leader election
						time.Sleep(etcdserver.HealthInterval + 2*time.Second)
					}

					memberID, clusterID := memberToRemove(ctx, t, cc, c.ClusterSize)
					removeResp, err := cc.MemberRemove(ctx, memberID)

					if c.ClusterSize == 1 && quorumTc.expectSingleNodeError {
						require.ErrorContains(t, err, "etcdserver: re-configuration failed due to not enough started members")
						return
					}

					if c.ClusterSize > 1 && quorumTc.expectClusterError {
						require.ErrorContains(t, err, "etcdserver: unhealthy cluster")
						return
					}

					require.NoErrorf(t, err, "MemberRemove failed")
					t.Logf("removeResp.Members:%v", removeResp.Members)
					require.Equalf(t, removeResp.Header.ClusterId, clusterID, "MemberRemove failed, expected ClusterID: %d, got: %d", clusterID, removeResp.Header.ClusterId)
					require.Lenf(t, removeResp.Members, c.ClusterSize-1, "MemberRemove failed, expected length of members: %d, got: %d", c.ClusterSize-1, len(removeResp.Members))
					for _, m := range removeResp.Members {
						require.NotEqualf(t, m.ID, memberID, "MemberRemove failed, member(id=%d) is still in cluster", memberID)
					}
				})
			})
		}
	}
}

// memberToRemove chooses a member to remove.
// If clusterSize == 1, return the only member.
// Otherwise, return a member that client has not connected to.
// It ensures that `MemberRemove` function does not return an "etcdserver: server stopped" error.
func memberToRemove(ctx context.Context, t *testing.T, client intf.Client, clusterSize int) (memberID uint64, clusterID uint64) {
	listResp, err := client.MemberList(ctx, false)
	require.NoError(t, err)

	clusterID = listResp.Header.ClusterId
	if clusterSize == 1 {
		memberID = listResp.Members[0].ID
	} else {
		// get status of the specific member that client has connected to
		statusResp, err := client.Status(ctx)
		require.NoError(t, err)

		// choose a member that client has not connected to
		for _, m := range listResp.Members {
			if m.ID != statusResp[0].Header.MemberId {
				memberID = m.ID
				break
			}
		}
		require.NotZerof(t, memberID, "memberToRemove failed. listResp:%v, statusResp:%v", listResp, statusResp)
	}
	return memberID, clusterID
}

func getMemberIDToEndpoints(ctx context.Context, t *testing.T, clus intf.Cluster) (memberIDToEndpoints map[uint64]string) {
	memberIDToEndpoints = make(map[uint64]string, len(clus.Endpoints()))
	for _, ep := range clus.Endpoints() {
		cc := testutils.MustClient(clus.Client(WithEndpoints([]string{ep})))
		gresp, err := cc.Get(ctx, "health", config.GetOptions{})
		require.NoError(t, err)
		memberIDToEndpoints[gresp.Header.MemberId] = ep
	}
	return memberIDToEndpoints
}
