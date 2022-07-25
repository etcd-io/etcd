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

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
)

func TestPeriodicCheck(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cc, err := clus.ClusterClient()
	require.NoError(t, err)

	ctx := context.Background()

	var totalRevisions int64 = 1210
	var rev int64
	for ; rev < totalRevisions; rev += testutil.CompactionCycle {
		testPeriodicCheck(ctx, t, cc, clus, rev, rev+testutil.CompactionCycle)
	}
	testPeriodicCheck(ctx, t, cc, clus, rev, rev+totalRevisions)
	alarmResponse, err := cc.AlarmList(ctx)
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember(nil), alarmResponse.Alarms)
}

func testPeriodicCheck(ctx context.Context, t *testing.T, cc *clientv3.Client, clus *ClusterV3, start, stop int64) {
	for i := start; i <= stop; i++ {
		if i%67 == 0 {
			_, err := cc.Delete(ctx, testutil.PickKey(i+83))
			assert.NoError(t, err, "error on delete")
		} else {
			_, err := cc.Put(ctx, testutil.PickKey(i), fmt.Sprint(i))
			assert.NoError(t, err, "error on put")
		}
	}
	err := clus.Members[0].CorruptionChecker().PeriodicCheck()
	assert.NoError(t, err, "error on periodic check (rev %v)", stop)
}

func TestPeriodicCheckDetectsCorruption(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cc, err := clus.ClusterClient()
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}

	err = clus.Members[0].CorruptionChecker().PeriodicCheck()
	assert.NoError(t, err, "error on periodic check")
	clus.Members[0].Stop(t)
	clus.WaitLeader(t)

	err = testutil.CorruptBBolt(clus.Members[0].BackendPath())
	assert.NoError(t, err)

	err = clus.Members[0].Restart(t)
	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	leader := clus.WaitLeader(t)
	err = clus.Members[leader].CorruptionChecker().PeriodicCheck()
	assert.NoError(t, err, "error on periodic check")
	time.Sleep(50 * time.Millisecond)
	alarmResponse, err := cc.AlarmList(ctx)
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: uint64(clus.Members[0].ID())}}, alarmResponse.Alarms)
}

func TestCompactHashCheck(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cc, err := clus.ClusterClient()
	require.NoError(t, err)

	ctx := context.Background()

	var totalRevisions int64 = 1210
	var rev int64
	for ; rev < totalRevisions; rev += testutil.CompactionCycle {
		testCompactionHash(ctx, t, cc, clus, rev, rev+testutil.CompactionCycle)
	}
	testCompactionHash(ctx, t, cc, clus, rev, rev+totalRevisions)
}

func testCompactionHash(ctx context.Context, t *testing.T, cc *clientv3.Client, clus *ClusterV3, start, stop int64) {
	for i := start; i <= stop; i++ {
		if i%67 == 0 {
			_, err := cc.Delete(ctx, testutil.PickKey(i+83))
			assert.NoError(t, err, "error on delete")
		} else {
			_, err := cc.Put(ctx, testutil.PickKey(i), fmt.Sprint(i))
			assert.NoError(t, err, "error on put")
		}
	}
	_, err := cc.Compact(ctx, stop)
	assert.NoError(t, err, "error on compact (rev %v)", stop)
	// Wait for compaction to be compacted
	time.Sleep(50 * time.Millisecond)

	clus.Members[0].CorruptionChecker().CompactHashCheck()
}

func TestCompactHashCheckDetectCorruption(t *testing.T) {
	BeforeTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cc, err := clus.ClusterClient()
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i))
		assert.NoError(t, err, "error on put")
	}

	clus.Members[0].CorruptionChecker().CompactHashCheck()
	clus.Members[0].Stop(t)
	clus.WaitLeader(t)

	err = testutil.CorruptBBolt(clus.Members[0].BackendPath())
	assert.NoError(t, err)

	err = clus.Members[0].Restart(t)
	assert.NoError(t, err)
	_, err = cc.Compact(ctx, 5)
	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	leader := clus.WaitLeader(t)
	clus.Members[leader].CorruptionChecker().CompactHashCheck()
	time.Sleep(50 * time.Millisecond)
	alarmResponse, err := cc.AlarmList(ctx)
	assert.NoError(t, err, "error on alarm list")
	assert.Equal(t, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: uint64(clus.Members[0].ID())}}, alarmResponse.Alarms)
}
