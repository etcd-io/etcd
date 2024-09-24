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
	kvc := integration.ToGRPC(mem.Client).KV

	// When starting a new cluster with 1 member, the member will have an index of 4.
	// TODO: Can someone explain this?
	// Currently, if `ep.appliedi-ep.snapi > s.Cfg.SnapshotCount`,
	// a raft log snapshot is created, and raft log entries are compacted.
	// In this case, it triggers when the index is a multiple of 11.
	appliedi := 4
	for ; appliedi <= 10; appliedi++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", appliedi, err)
		}
	}
	// The first snapshot and compaction shouldn't happen because the index is less than 11
	expectMemberLogTimeout(t, mem, 5*time.Second, "saved snapshot", 1)
	expectMemberLogTimeout(t, mem, time.Second, "compacted Raft logs", 1)

	for ; appliedi <= 11; appliedi++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", appliedi, err)
		}
	}
	// The first snapshot and compaction should happen because the index is 11
	expectMemberLog(t, mem, 5*time.Second, "saved snapshot", 1)
	expectMemberLog(t, mem, time.Second, "compacted Raft logs", 1)
	expectMemberLog(t, mem, time.Second, "\"compact-index\": 6", 1)

	for ; appliedi <= 1100; appliedi++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", appliedi, err)
		}
	}
	// With the index at 1100, snapshot and compaction should happen 100 times.
	expectMemberLog(t, mem, 5*time.Second, "saved snapshot", 100)
	expectMemberLog(t, mem, time.Second, "compacted Raft logs", 100)
	expectMemberLog(t, mem, time.Second, "\"compact-index\": 1095", 1)

	// No more snapshot and compaction should happen.
	expectMemberLogTimeout(t, mem, 5*time.Second, "saved snapshot", 101)
	expectMemberLogTimeout(t, mem, time.Second, "compacted Raft logs", 101)
}

// expectMemberLogTimeout ensures that the log has fewer than `count` occurrences of `s` before timing out
func expectMemberLogTimeout(t *testing.T, m *integration.Member, timeout time.Duration, s string, count int) {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	_, err := m.LogObserver.Expect(ctx, s, count)
	if !errors.Is(err, context.DeadlineExceeded) {
		if err != nil {
			t.Fatalf("failed to expect (log:%s, count:%v): %v", s, count, err)
		}
	}
}
