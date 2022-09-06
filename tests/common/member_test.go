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
			expectError:         false,
		},
		{
			name:                "StrictReconfigCheck/NoWaitForQuorum",
			strictReconfigCheck: true,
			waitForQuorum:       false,
			expectError:         true,
		},
		{
			name:                "DisableStrictReconfigCheck/WaitForQuorum",
			strictReconfigCheck: false,
			waitForQuorum:       true,
			expectError:         false,
		},
		{
			name:                "DisableStrictReconfigCheck/NoWaitForQuorum",
			strictReconfigCheck: false,
			waitForQuorum:       false,
			expectError:         false,
		},
	}

	for _, learnerTc := range learnerTcs {
		for _, quorumTc := range quorumTcs {
			for _, clusterTc := range clusterTestCases {
				t.Run(learnerTc.name+"/"+quorumTc.name+"/"+clusterTc.name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					c := clusterTc.config
					if !quorumTc.strictReconfigCheck {
						c.DisableStrictReconfigCheck = true
					}
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
