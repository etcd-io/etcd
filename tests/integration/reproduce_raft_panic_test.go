// Copyright 2024 The etcd Authors
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestMustPanic(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:                   3,
		SnapshotCount:          1000,
		SnapshotCatchUpEntries: 100,
	})
	defer clus.Terminate(t)

	// inject partition between [member-0] and [member-1, member 2]
	clus.Members[0].InjectPartition(t, clus.Members[1:]...)

	// wait for leader in [member-1, member 2]
	lead := clus.WaitMembersForLeader(t, clus.Members[1:]) + 1
	t.Logf("elected lead: %v", clus.Members[lead].Server.MemberID())
	time.Sleep(2 * time.Second)

	// send 500 put request to [member-1, member 2], resulting at least 400 compactions in raft log
	for i := 0; i < 500; i++ {
		_, err := clus.Client(1).Put(context.TODO(), "foo", "bar")
		if err != nil {
			return
		}
	}

	expectMemberLog(t, clus.Members[lead], 5*time.Second, "compacted Raft logs", 400)
	expectMemberLog(t, clus.Members[lead], 5*time.Second, "\"compact-index\": 400", 1)

	// member-0 rejoins the cluster. Since its appliedIndex is very low (less than 10, not in leader raft log's range),
	// the leader decides to send a snapshot to member-0.
	clus.Members[0].RecoverPartition(t, clus.Members[1:]...)

	// Wait for the leader to panic with `panic("need non-empty snapshot")`
	time.Sleep(5 * time.Second)
}

func TestMustNotPanic(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:                   3,
		SnapshotCount:          1000,
		SnapshotCatchUpEntries: 100,
	})
	defer clus.Terminate(t)

	// inject partition between [member-0] and [member-1, member 2]
	clus.Members[0].InjectPartition(t, clus.Members[1:]...)

	// wait for leader in [member-1, member 2]
	lead := clus.WaitMembersForLeader(t, clus.Members[1:]) + 1
	t.Logf("elected lead: %v", clus.Members[lead].Server.MemberID())
	time.Sleep(2 * time.Second)

	// send 80 put request to [member-1, member 2], no compaction in raft log
	for i := 0; i < 80; i++ {
		_, err := clus.Client(1).Put(context.TODO(), "foo", "bar")
		if err != nil {
			return
		}
	}

	// leader should not compact raft logs
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	_, err := clus.Members[lead].LogObserver.Expect(ctx, "compacted Raft logs", 1)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// member-0 rejoins the cluster.
	// Since member-0's appliedIndex is within the leader raft log's range,
	// the leader won't send a snapshot. So, no panic should happen here.
	clus.Members[0].RecoverPartition(t, clus.Members[1:]...)

	// No errors should occur during this wait
	time.Sleep(5 * time.Second)
}
