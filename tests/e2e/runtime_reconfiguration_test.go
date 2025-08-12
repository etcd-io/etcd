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

//go:build !cluster_proxy

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestRuntimeReconfigGrowClusterSize ensures growing cluster size with two phases
// Phase 1 - Inform cluster of new configuration
// Phase 2 - Start new member
func TestRuntimeReconfigGrowClusterSize(t *testing.T) {
	e2e.BeforeTest(t)

	tcs := []struct {
		name        string
		clusterSize int
		asLearner   bool
	}{
		{
			name:        "grow cluster size from 1 to 3",
			clusterSize: 1,
		},
		{
			name:        "grow cluster size from 3 to 5",
			clusterSize: 3,
		},
		{
			name:        "grow cluster size from 1 to 3 with learner",
			clusterSize: 1,
			asLearner:   true,
		},
		{
			name:        "grow cluster size from 3 to 5 with learner",
			clusterSize: 3,
			asLearner:   true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithClusterSize(tc.clusterSize))
			require.NoError(t, err)
			require.NoError(t, epc.Procs[0].Etcdctl().Health(ctx))
			defer func() {
				err := epc.Close()
				require.NoErrorf(t, err, "failed to close etcd cluster")
			}()

			for i := 0; i < 2; i++ {
				time.Sleep(etcdserver.HealthInterval)
				if !tc.asLearner {
					addMember(ctx, t, epc)
				} else {
					addMemberAsLearnerAndPromote(ctx, t, epc)
				}
			}
		})
	}
}

func TestRuntimeReconfigDecreaseClusterSize(t *testing.T) {
	e2e.BeforeTest(t)

	tcs := []struct {
		name        string
		clusterSize int
		asLearner   bool
	}{
		{
			name:        "decrease cluster size from 3 to 1",
			clusterSize: 3,
		},
		{
			name:        "decrease cluster size from 5 to 3",
			clusterSize: 5,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithClusterSize(tc.clusterSize))
			require.NoError(t, err)
			require.NoError(t, epc.Procs[0].Etcdctl().Health(ctx))
			defer func() {
				err := epc.Close()
				require.NoErrorf(t, err, "failed to close etcd cluster")
			}()

			for i := 0; i < 2; i++ {
				time.Sleep(etcdserver.HealthInterval)
				removeFirstMember(ctx, t, epc)
			}
		})
	}
}

func TestRuntimeReconfigRollingUpgrade(t *testing.T) {
	e2e.BeforeTest(t)

	tcs := []struct {
		name        string
		withLearner bool
	}{
		{
			name:        "with learner",
			withLearner: true,
		},
		{
			name:        "without learner",
			withLearner: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			epc, err := e2e.NewEtcdProcessCluster(ctx, t, e2e.WithClusterSize(3))
			require.NoError(t, err)
			require.NoError(t, epc.Procs[0].Etcdctl().Health(ctx))
			defer func() {
				err := epc.Close()
				require.NoErrorf(t, err, "failed to close etcd cluster")
			}()

			for i := 0; i < 2; i++ {
				time.Sleep(etcdserver.HealthInterval)
				removeFirstMember(ctx, t, epc)
				epc.WaitLeader(t)
				// if we do not wait for leader, without the fix of notify raft Advance,
				// have to wait 1 sec to pass the test stably.
				if tc.withLearner {
					addMemberAsLearnerAndPromote(ctx, t, epc)
				} else {
					addMember(ctx, t, epc)
				}
			}
		})
	}
}

func addMember(ctx context.Context, t *testing.T, epc *e2e.EtcdProcessCluster) {
	_, err := epc.StartNewProc(ctx, nil, t, false /* addAsLearner */)
	require.NoError(t, err)
	require.NoError(t, epc.Procs[len(epc.Procs)-1].Etcdctl().Health(ctx))
}

func addMemberAsLearnerAndPromote(ctx context.Context, t *testing.T, epc *e2e.EtcdProcessCluster) {
	endpoints := epc.EndpointsGRPC()

	id, err := epc.StartNewProc(ctx, nil, t, true /* addAsLearner */)
	require.NoError(t, err)

	attempt := 0
	for attempt < 3 {
		_, err = epc.Etcdctl(e2e.WithEndpoints(endpoints)).MemberPromote(ctx, id)
		if err == nil || !strings.Contains(err.Error(), "can only promote a learner member which is in sync with leader") {
			break
		}
		time.Sleep(100 * time.Millisecond)
		attempt++
	}
	require.NoError(t, err)

	newLearnerMemberProc := epc.Procs[len(epc.Procs)-1]
	require.NoError(t, newLearnerMemberProc.Etcdctl().Health(ctx))
}

func removeFirstMember(ctx context.Context, t *testing.T, epc *e2e.EtcdProcessCluster) {
	// avoid tearing down the last etcd process
	if len(epc.Procs) == 1 {
		return
	}

	firstProc := epc.Procs[0]
	sts, err := firstProc.Etcdctl().Status(ctx)
	require.NoError(t, err)
	memberIDToRemove := sts[0].Header.MemberId

	epc.Procs = epc.Procs[1:]
	_, err = epc.Etcdctl().MemberRemove(ctx, memberIDToRemove)
	require.NoError(t, err)
	require.NoError(t, firstProc.Stop())
	require.NoError(t, firstProc.Close())
}
