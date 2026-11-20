// Copyright 2026 The etcd Authors
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

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestPeriodicCheckDetectsCorruption(t *testing.T) {
	testDetectCorruption(t, false)
}

func TestCompactHashCheckDetectCorruption(t *testing.T) {
	testDetectCorruption(t, true)
}

func testDetectCorruption(t *testing.T, compact bool) {
	t.Helper()
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	const checkInterval = time.Second
	opts := []config.ClusterOption{config.WithClusterSize(3)}
	if compact {
		opts = append(opts, config.WithCompactHashCheckEnabled(true), config.WithCompactHashCheckTime(checkInterval))
	} else {
		opts = append(opts, config.WithCorruptCheckTime(checkInterval))
	}
	clus := testRunner.NewCluster(ctx, t, opts...)
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())

	var revision int64
	for i := 0; i < 10; i++ {
		resp, err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i), config.PutOptions{})
		require.NoError(t, err)
		revision = resp.Header.Revision
	}

	member := clus.Members()[0]
	// Wait for the member's local store to apply every write before changing it.
	// Otherwise a write still in its Raft log could mask the injected corruption.
	require.Eventually(t, func() bool {
		resp, err := member.Client().Get(ctx, testutil.PickKey(9), config.GetOptions{Serializable: true})
		return err == nil && resp.Header.Revision == revision && len(resp.Kvs) == 1
	}, 5*time.Second, 10*time.Millisecond)
	status, err := member.Client().Status(ctx)
	require.NoError(t, err)
	require.Len(t, status, 1)
	memberID := status[0].Header.MemberId

	alarmResponse, err := cc.AlarmList(ctx)
	require.NoError(t, err)
	require.Empty(t, alarmResponse.Alarms)

	member.Stop()
	require.NoError(t, testutil.CorruptBBolt(member.BackendPath()))
	require.NoError(t, member.Start(ctx))
	clus.WaitLeader(t)
	if compact {
		_, err = cc.Compact(ctx, 5, config.CompactOption{Physical: true})
		require.NoError(t, err)
	}

	// Detection and alarm replication are asynchronous. Poll rather than assuming
	// that one check interval is enough for all members to observe the alarm.
	require.Eventually(t, func() bool {
		response, err := cc.AlarmList(ctx)
		return err == nil && len(response.Alarms) == 1 &&
			response.Alarms[0].Alarm == etcdserverpb.AlarmType_CORRUPT &&
			response.Alarms[0].MemberID == memberID
	}, 10*time.Second, 50*time.Millisecond)
}
