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
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestMemberList(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteUntil(ctx, t, func() {
				resp, err := cc.MemberList(ctx)
				if err != nil {
					t.Fatalf("could not get member list, err: %s", err)
				}
				expectNum := len(clus.Members())
				gotNum := len(resp.Members)
				if expectNum != gotNum {
					t.Fatalf("number of members not equal, expect: %d, got: %d", expectNum, gotNum)
				}
				for _, m := range resp.Members {
					if len(m.ClientURLs) == 0 {
						t.Fatalf("member is not started, memberId:%d, memberName:%s", m.ID, m.Name)
					}
				}
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
			for _, clusterTc := range clusterTestCases {
				t.Run(learnerTc.name+"/"+quorumTc.name+"/"+clusterTc.name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					c := clusterTc.config
					c.DisableStrictReconfigCheck = !quorumTc.strictReconfigCheck
					clus := testRunner.NewCluster(ctx, t, c)
					defer clus.Close()
					cc := clus.Client()

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
							require.NoError(t, err, "MemberAdd failed")
							if addResp.Member == nil {
								t.Fatalf("MemberAdd failed, expected: member != nil, got: member == nil")
							}
							if addResp.Member.ID == 0 {
								t.Fatalf("MemberAdd failed, expected: ID != 0, got: ID == 0")
							}
							if len(addResp.Member.PeerURLs) == 0 {
								t.Fatalf("MemberAdd failed, expected: non-empty PeerURLs, got: empty PeerURLs")
							}
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
		for _, clusterTc := range clusterTestCases {
			if !quorumTc.strictReconfigCheck && clusterTc.config.ClusterSize == 1 {
				// skip these test cases
				// when strictReconfigCheck is disabled, calling MemberRemove will cause the single node to panic
				continue
			}
			t.Run(quorumTc.name+"/"+clusterTc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				c := clusterTc.config
				c.DisableStrictReconfigCheck = !quorumTc.strictReconfigCheck
				clus := testRunner.NewCluster(ctx, t, c)
				defer clus.Close()
				// client connects to a specific member which won't be removed from cluster
				cc := clus.Members()[0].Client()

				testutils.ExecuteUntil(ctx, t, func() {
					if quorumTc.waitForQuorum {
						time.Sleep(etcdserver.HealthInterval)
					}

					memberId, clusterId := memberToRemove(ctx, t, cc, c.ClusterSize)
					removeResp, err := cc.MemberRemove(ctx, memberId)

					if c.ClusterSize == 1 && quorumTc.expectSingleNodeError {
						require.ErrorContains(t, err, "etcdserver: re-configuration failed due to not enough started members")
						return
					}

					if c.ClusterSize > 1 && quorumTc.expectClusterError {
						require.ErrorContains(t, err, "etcdserver: unhealthy cluster")
						return
					}

					require.NoError(t, err, "MemberRemove failed")
					t.Logf("removeResp.Members:%v", removeResp.Members)
					if removeResp.Header.ClusterId != clusterId {
						t.Fatalf("MemberRemove failed, expected ClusterId: %d, got: %d", clusterId, removeResp.Header.ClusterId)
					}
					if len(removeResp.Members) != c.ClusterSize-1 {
						t.Fatalf("MemberRemove failed, expected length of members: %d, got: %d", c.ClusterSize-1, len(removeResp.Members))
					}
					for _, m := range removeResp.Members {
						if m.ID == memberId {
							t.Fatalf("MemberRemove failed, member(id=%d) is still in cluster", memberId)
						}
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
func memberToRemove(ctx context.Context, t *testing.T, client framework.Client, clusterSize int) (memberId uint64, clusterId uint64) {
	listResp, err := client.MemberList(ctx)
	if err != nil {
		t.Fatal(err)
	}

	clusterId = listResp.Header.ClusterId
	if clusterSize == 1 {
		memberId = listResp.Members[0].ID
	} else {
		// get status of the specific member that client has connected to
		statusResp, err := client.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}

		// choose a member that client has not connected to
		for _, m := range listResp.Members {
			if m.ID != statusResp[0].Header.MemberId {
				memberId = m.ID
				break
			}
		}
		if memberId == 0 {
			t.Fatalf("memberToRemove failed. listResp:%v, statusResp:%v", listResp, statusResp)
		}
	}
	return memberId, clusterId
}
