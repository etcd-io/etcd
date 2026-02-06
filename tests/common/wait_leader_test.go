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

	"go.etcd.io/etcd/tests/v3/framework/config"
)

func TestWaitLeader(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()

			leader := clus.WaitLeader(t)
			require.GreaterOrEqualf(t, leader, 0, "WaitLeader failed for the leader index (%d) is out of range, cluster member count: %d", leader, len(clus.Members()))
			require.Lessf(t, leader, len(clus.Members()), "WaitLeader failed for the leader index (%d) is out of range, cluster member count: %d", leader, len(clus.Members()))
		})
	}
}

func TestWaitLeader_MemberStop(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []testCase{
		{
			name:   "PeerTLS",
			config: config.NewClusterConfig(config.WithPeerTLS(config.ManualTLS)),
		},
		{
			name:   "PeerAutoTLS",
			config: config.NewClusterConfig(config.WithPeerTLS(config.AutoTLS)),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()

			lead1 := clus.WaitLeader(t)
			require.GreaterOrEqualf(t, lead1, 0, "WaitLeader failed for the leader index (%d) is out of range, cluster member count: %d", lead1, len(clus.Members()))
			require.Lessf(t, lead1, len(clus.Members()), "WaitLeader failed for the leader index (%d) is out of range, cluster member count: %d", lead1, len(clus.Members()))

			clus.Members()[lead1].Stop()
			lead2 := clus.WaitLeader(t)
			require.GreaterOrEqualf(t, lead2, 0, "WaitLeader failed for the leader index (%d) is out of range, cluster member count: %d", lead2, len(clus.Members()))
			require.Lessf(t, lead2, len(clus.Members()), "WaitLeader failed for the leader index (%d) is out of range, cluster member count: %d", lead2, len(clus.Members()))

			require.NotEqualf(t, lead1, lead2, "WaitLeader failed for the leader(index=%d) did not change as expected after a member stopped", lead1)
		})
	}
}
