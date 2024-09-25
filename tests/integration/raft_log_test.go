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
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestRaftLogCompaction tests whether raft log snapshot and compaction work correctly.
func TestRaftLogCompaction(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:                   1,
		SnapshotCount:          10,
		SnapshotCatchUpEntries: 5,
	})
	defer clus.Terminate(t)

	mem := clus.Members[0]

	// Get applied index of raft log
	endpoint := mem.Client.Endpoints()[0]
	assert.NotEmpty(t, endpoint)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, _ := mem.Client.Status(ctx, endpoint)
	appliedi := status.RaftAppliedIndex
	// Assume applied index is less than 10, should be fine at this stage
	assert.Less(t, appliedi, uint64(10))

	kvc := integration.ToGRPC(mem.Client).KV

	// When applied index is a multiple of 11 (SnapshotCount+1),
	// a snapshot is created, and entries are compacted.
	//
	// increase applied index to 10
	for ; appliedi < 10; appliedi++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", appliedi, err)
		}
	}
	// The first snapshot and compaction shouldn't happen because the index is less than 11
	logOccurredAtMostNTimes(t, mem, 5*time.Second, "saved snapshot", 0)
	logOccurredAtMostNTimes(t, mem, time.Second, "compacted Raft logs", 0)

	// increase applied index to 11
	for ; appliedi < 11; appliedi++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", appliedi, err)
		}
	}
	// The first snapshot and compaction should happen because the index is 11
	logOccurredAtMostNTimes(t, mem, 5*time.Second, "saved snapshot", 1)
	logOccurredAtMostNTimes(t, mem, time.Second, "compacted Raft logs", 1)
	expectMemberLog(t, mem, time.Second, "\"compact-index\": 6", 1)

	// increase applied index to 1100
	for ; appliedi < 1100; appliedi++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", appliedi, err)
		}
	}
	// With applied index at 1100, snapshot and compaction should happen 100 times.
	logOccurredAtMostNTimes(t, mem, 5*time.Second, "saved snapshot", 100)
	logOccurredAtMostNTimes(t, mem, time.Second, "compacted Raft logs", 100)
	expectMemberLog(t, mem, time.Second, "\"compact-index\": 1095", 1)
}

// logOccurredAtMostNTimes ensures that the log has exactly `count` occurrences of `s` before timing out, no more, no less.
func logOccurredAtMostNTimes(t *testing.T, m *integration.Member, timeout time.Duration, s string, count int) {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	// The log must have `count` occurrences before timeout
	_, err := m.LogObserver.Expect(ctx, s, count)
	if err != nil {
		t.Fatalf("failed to expect(log:%s, count:%d): %v", s, count, err)
	}

	// The log mustn't have `count+1` occurrences before timeout
	lines, err := m.LogObserver.Expect(ctx, s, count+1)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return
		} else {
			t.Fatalf("failed to expect(log:%s, count:%d): %v", s, count+1, err)
		}
	}
	t.Fatalf("failed: too many occurrences of %s, expect %d, got %d", s, count, len(lines))
}