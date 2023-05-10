// Copyright 2023 The etcd Authors
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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/raft/v3"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/interfaces"
)

func TestGracefulShutdown(t *testing.T) {
	tcs := []struct {
		name        string
		clusterSize int
	}{
		{
			name:        "clusterSize3",
			clusterSize: 3,
		},
		{
			name:        "clusterSize5",
			clusterSize: 5,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testRunner := e2e.NewE2eRunner()
			testRunner.BeforeTest(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterSize(tc.clusterSize))
			// clean up orphaned resources like closing member client.
			defer clus.Close()
			// shutdown each etcd member process sequentially
			// and start from old leader, (new leader), (follower)
			tryShutdownLeader(ctx, t, clus.Members())
		})
	}
}

// tryShutdownLeader tries stop etcd member if it is leader.
// it also asserts stop leader should not take longer than 1.5 seconds and leaderID has been changed within 500ms.
func tryShutdownLeader(ctx context.Context, t *testing.T, members []interfaces.Member) {
	quorum := len(members)/2 + 1
	for len(members) > quorum {
		leader, leaderID, term, followers := getLeader(ctx, t, members)
		stopped := make(chan error, 1)
		go func() {
			// each etcd server will wait up to 1 seconds to close all idle connections in peer handler.
			start := time.Now()
			leader.Stop()
			took := time.Since(start)
			if took > 1500*time.Millisecond {
				stopped <- fmt.Errorf("leader stop took %v longer than 1.5 seconds", took)
				return
			}
			stopped <- nil
		}()

		// etcd election timeout could range from 1s to 2s without explicit leadership transfer.
		// assert leader ID has been changed within 500ms
		time.Sleep(500 * time.Millisecond)
		resps, err := followers[0].Client().Status(ctx)
		require.NoError(t, err)
		require.NotEqual(t, leaderID, raft.None)
		require.Equal(t, resps[0].RaftTerm, term+1)
		require.NotEqualf(t, resps[0].Leader, leaderID, "expect old leaderID %x changed to new leader ID %x", leaderID, resps[0].Leader)

		err = <-stopped
		require.NoError(t, err)

		members = followers
	}
}

func getLeader(ctx context.Context, t *testing.T, members []interfaces.Member) (leader interfaces.Member, leaderID, term uint64, followers []interfaces.Member) {
	leaderIdx := -1
	for i, m := range members {
		mc := m.Client()
		sresps, err := mc.Status(ctx)
		require.NoError(t, err)
		if sresps[0].Leader == sresps[0].Header.MemberId {
			leaderIdx = i
			leaderID = sresps[0].Leader
			term = sresps[0].RaftTerm
			break
		}
	}
	if leaderIdx == -1 {
		return nil, 0, 0, members
	}
	leader = members[leaderIdx]
	return leader, leaderID, term, append(members[:leaderIdx], members[leaderIdx+1:]...)
}
