// Copyright 2015 The etcd Authors
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

package raft

import (
	"testing"

	pb "go.etcd.io/etcd/raft/raftpb"
)

var (
	testingSnap = pb.Snapshot{
		Metadata: pb.SnapshotMetadata{
			Index:     11, // magic number
			Term:      11, // magic number
			ConfState: pb.ConfState{Voters: []uint64{1, 2}},
		},
	}
)

func TestSendingSnapshotSetPendingSnapshot(t *testing.T) {
	storage := NewMemoryStorage()
	sm := newTestRaft(1, []uint64{1}, 10, 1, storage)
	sm.restore(testingSnap)

	sm.becomeCandidate()
	sm.becomeLeader()

	// force set the next of node 2, so that
	// node 2 needs a snapshot
	sm.prs.Progress[2].Next = sm.raftLog.firstIndex()

	sm.Step(pb.Message{From: 2, To: 1, Type: pb.MsgAppResp, Index: sm.prs.Progress[2].Next - 1, Reject: true})
	if sm.prs.Progress[2].PendingSnapshot != 11 {
		t.Fatalf("PendingSnapshot = %d, want 11", sm.prs.Progress[2].PendingSnapshot)
	}
}

func TestPendingSnapshotPauseReplication(t *testing.T) {
	storage := NewMemoryStorage()
	sm := newTestRaft(1, []uint64{1, 2}, 10, 1, storage)
	sm.restore(testingSnap)

	sm.becomeCandidate()
	sm.becomeLeader()

	sm.prs.Progress[2].BecomeSnapshot(11)

	sm.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
	msgs := sm.readMessages()
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want 0", len(msgs))
	}
}

func TestSnapshotFailure(t *testing.T) {
	storage := NewMemoryStorage()
	sm := newTestRaft(1, []uint64{1, 2}, 10, 1, storage)
	sm.restore(testingSnap)

	sm.becomeCandidate()
	sm.becomeLeader()

	sm.prs.Progress[2].Next = 1
	sm.prs.Progress[2].BecomeSnapshot(11)

	sm.Step(pb.Message{From: 2, To: 1, Type: pb.MsgSnapStatus, Reject: true})
	if sm.prs.Progress[2].PendingSnapshot != 0 {
		t.Fatalf("PendingSnapshot = %d, want 0", sm.prs.Progress[2].PendingSnapshot)
	}
	if sm.prs.Progress[2].Next != 1 {
		t.Fatalf("Next = %d, want 1", sm.prs.Progress[2].Next)
	}
	if !sm.prs.Progress[2].ProbeSent {
		t.Errorf("ProbeSent = %v, want true", sm.prs.Progress[2].ProbeSent)
	}
}

func TestSnapshotSucceed(t *testing.T) {
	storage := NewMemoryStorage()
	sm := newTestRaft(1, []uint64{1, 2}, 10, 1, storage)
	sm.restore(testingSnap)

	sm.becomeCandidate()
	sm.becomeLeader()

	sm.prs.Progress[2].Next = 1
	sm.prs.Progress[2].BecomeSnapshot(11)

	sm.Step(pb.Message{From: 2, To: 1, Type: pb.MsgSnapStatus, Reject: false})
	if sm.prs.Progress[2].PendingSnapshot != 0 {
		t.Fatalf("PendingSnapshot = %d, want 0", sm.prs.Progress[2].PendingSnapshot)
	}
	if sm.prs.Progress[2].Next != 12 {
		t.Fatalf("Next = %d, want 12", sm.prs.Progress[2].Next)
	}
	if !sm.prs.Progress[2].ProbeSent {
		t.Errorf("ProbeSent = %v, want true", sm.prs.Progress[2].ProbeSent)
	}
}

func TestSnapshotAbort(t *testing.T) {
	storage := NewMemoryStorage()
	sm := newTestRaft(1, []uint64{1, 2}, 10, 1, storage)
	sm.restore(testingSnap)

	sm.becomeCandidate()
	sm.becomeLeader()

	sm.prs.Progress[2].Next = 1
	sm.prs.Progress[2].BecomeSnapshot(11)

	// A successful msgAppResp that has a higher/equal index than the
	// pending snapshot should abort the pending snapshot.
	sm.Step(pb.Message{From: 2, To: 1, Type: pb.MsgAppResp, Index: 11})
	if sm.prs.Progress[2].PendingSnapshot != 0 {
		t.Fatalf("PendingSnapshot = %d, want 0", sm.prs.Progress[2].PendingSnapshot)
	}
	// The follower entered StateReplicate and the leader send an append
	// and optimistically updated the progress (so we see 13 instead of 12).
	// There is something to append because the leader appended an empty entry
	// to the log at index 12 when it assumed leadership.
	if sm.prs.Progress[2].Next != 13 {
		t.Fatalf("Next = %d, want 13", sm.prs.Progress[2].Next)
	}
	if n := sm.prs.Progress[2].Inflights.Count(); n != 1 {
		t.Fatalf("expected an inflight message, got %d", n)
	}
}
