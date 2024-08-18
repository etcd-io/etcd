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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

const (
	// NOTE: When starting a new cluster with 1 member, the member will
	// apply 3 ConfChange directly at the beginning, meaning its applied
	// index is 4.
	MemberInitialAppliedIndex = 4
)

// TestCompactRaftLog_Config tests different value for raftLogCompactionStep
func TestCompactRaftLog_Config(t *testing.T) {
	integration.BeforeTest(t)

	expectConfig := func(RaftLogCompactionStep uint64, expected uint64) {
		clus := integration.NewCluster(t, &integration.ClusterConfig{
			Size:                  3,
			RaftLogCompactionStep: RaftLogCompactionStep,
		})
		defer clus.Terminate(t)
		for _, member := range clus.Members {
			assert.Equal(t, expected, member.ServerConfig.RaftLogCompactionStep)
		}
	}

	expectConfig(0, etcdserver.DefaultRaftLogCompactionStep)
	expectConfig(1, 1)
	expectConfig(1234, 1234)
}

// test case for TestCompactRaftLog_CompactionTimes
type tsCompactionTimes struct {
	raftLogCompactionStep  int
	snapshotCatchUpEntries int
	// minimum value is MemberInitialAppliedIndex
	increaseAppliedIndexTo  int
	expectedCompactionTimes int
}

// TestCompactRaftLog_CompactionTimes tests how many times compaction should occur
//
// 1. raft log gets compacted only when raft storage entries size is greater than SnapshotCatchUpEntries
// 2. raft log gets compacted when applied index is a multiple of RaftLogCompactionStep
func TestCompactRaftLog_CompactionTimes(t *testing.T) {
	integration.BeforeTest(t)

	tt := []tsCompactionTimes{
		// compaction should NOT occur if increaseAppliedIndexTo <= snapshotCatchUpEntries
		{
			raftLogCompactionStep:   1,
			snapshotCatchUpEntries:  201,
			increaseAppliedIndexTo:  201,
			expectedCompactionTimes: 0,
		},

		// if raftLogCompactionStep is 1 and increaseAppliedIndexTo > snapshotCatchUpEntries, compaction should occur after each put request
		{
			raftLogCompactionStep:   1,
			snapshotCatchUpEntries:  201,
			increaseAppliedIndexTo:  202,
			expectedCompactionTimes: 1,
		},
		{
			raftLogCompactionStep:   1,
			snapshotCatchUpEntries:  201,
			increaseAppliedIndexTo:  300,
			expectedCompactionTimes: 300 - 201,
		},

		// compaction should only occur when increaseAppliedIndexTo is a multiple of raftLogCompactionStep
		{
			raftLogCompactionStep:   1005,
			snapshotCatchUpEntries:  1000,
			increaseAppliedIndexTo:  1004,
			expectedCompactionTimes: 0,
		},
		{
			raftLogCompactionStep:   1005,
			snapshotCatchUpEntries:  1000,
			increaseAppliedIndexTo:  1005,
			expectedCompactionTimes: 1,
		},
		{
			raftLogCompactionStep:   1005,
			snapshotCatchUpEntries:  1000,
			increaseAppliedIndexTo:  2009,
			expectedCompactionTimes: 1,
		},
		{
			raftLogCompactionStep:   1005,
			snapshotCatchUpEntries:  1000,
			increaseAppliedIndexTo:  2010,
			expectedCompactionTimes: 2,
		},
		{
			raftLogCompactionStep:   1005,
			snapshotCatchUpEntries:  1000,
			increaseAppliedIndexTo:  3014,
			expectedCompactionTimes: 2,
		},
		{
			raftLogCompactionStep:   1005,
			snapshotCatchUpEntries:  1000,
			increaseAppliedIndexTo:  3015,
			expectedCompactionTimes: 3,
		},
	}

	for _, ts := range tt {
		mustCompactForExpectedTimes(t, ts)
	}
}

func mustCompactForExpectedTimes(t *testing.T, ts tsCompactionTimes) {
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:                   1,
		SnapshotCatchUpEntries: uint64(ts.snapshotCatchUpEntries),
		RaftLogCompactionStep:  uint64(ts.raftLogCompactionStep),
	})
	defer clus.Terminate(t)

	kvc := integration.ToGRPC(clus.RandClient()).KV

	for i := 0; i < ts.increaseAppliedIndexTo-MemberInitialAppliedIndex; i++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", i, err)
		}
	}
	t.Logf("test case: %+v", ts)
	expectMemberLogExactCount(t, clus.Members[0], 5*time.Second, "compacted Raft logs", ts.expectedCompactionTimes)
}

// expectMemberLogExactCount ensures the member log has exactly 'count' entries containing `s` - no more, no less
func expectMemberLogExactCount(t *testing.T, m *integration.Member, timeout time.Duration, s string, count int) {
	// make sure at LEAST `count` log entries contain `s`
	if count > 0 {
		ctx1, cancel1 := context.WithTimeout(context.TODO(), timeout)
		defer cancel1()

		_, err := m.LogObserver.Expect(ctx1, s, count)
		if err != nil {
			t.Fatalf("failed to expect (log:%s, count:%v): %v", s, count, err)
		}
	}

	ctx2, cancel2 := context.WithTimeout(context.TODO(), timeout)
	defer cancel2()

	// make sure at MOST `count` log entries contain `s`
	lines, err := m.LogObserver.Expect(ctx2, s, count+1)
	if err != nil {
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("failed to expect (log:%s, count:%v): %v", s, count, err)
		} else {
			return
		}
	}

	for _, line := range lines {
		t.Logf("[unexpected line]: %v", line)
	}

	t.Fatalf("expected %d line, got %d line(s)", count, len(lines))
}
