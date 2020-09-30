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

	pb "go.etcd.io/etcd/v3/raft/raftpb"
)

// TestMsgAppFlowControlFull ensures:
// 1. msgApp can fill the sending window until full
// 2. when the window is full, no more msgApp can be sent.

func TestMsgAppFlowControlFull(t *testing.T) {
	r := newTestRaft(1, []uint64{1, 2}, 5, 1, NewMemoryStorage())
	r.becomeCandidate()
	r.becomeLeader()

	pr2 := r.prs.Progress[2]
	// force the progress to be in replicate state
	pr2.BecomeReplicate()
	// fill in the inflights window
	for i := 0; i < r.prs.MaxInflight; i++ {
		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
		ms := r.readMessages()
		if len(ms) != 1 {
			t.Fatalf("#%d: len(ms) = %d, want 1", i, len(ms))
		}
	}

	// ensure 1
	if !pr2.Inflights.Full() {
		t.Fatalf("inflights.full = %t, want %t", pr2.Inflights.Full(), true)
	}

	// ensure 2
	for i := 0; i < 10; i++ {
		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
		ms := r.readMessages()
		if len(ms) != 0 {
			t.Fatalf("#%d: len(ms) = %d, want 0", i, len(ms))
		}
	}
}

// TestMsgAppFlowControlMoveForward ensures msgAppResp can move
// forward the sending window correctly:
// 1. valid msgAppResp.index moves the windows to pass all smaller or equal index.
// 2. out-of-dated msgAppResp has no effect on the sliding window.
func TestMsgAppFlowControlMoveForward(t *testing.T) {
	r := newTestRaft(1, []uint64{1, 2}, 5, 1, NewMemoryStorage())
	r.becomeCandidate()
	r.becomeLeader()

	pr2 := r.prs.Progress[2]
	// force the progress to be in replicate state
	pr2.BecomeReplicate()
	// fill in the inflights window
	for i := 0; i < r.prs.MaxInflight; i++ {
		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
		r.readMessages()
	}

	// 1 is noop, 2 is the first proposal we just sent.
	// so we start with 2.
	for tt := 2; tt < r.prs.MaxInflight; tt++ {
		// move forward the window
		r.Step(pb.Message{From: 2, To: 1, Type: pb.MsgAppResp, Index: uint64(tt)})
		r.readMessages()

		// fill in the inflights window again
		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
		ms := r.readMessages()
		if len(ms) != 1 {
			t.Fatalf("#%d: len(ms) = %d, want 1", tt, len(ms))
		}

		// ensure 1
		if !pr2.Inflights.Full() {
			t.Fatalf("inflights.full = %t, want %t", pr2.Inflights.Full(), true)
		}

		// ensure 2
		for i := 0; i < tt; i++ {
			r.Step(pb.Message{From: 2, To: 1, Type: pb.MsgAppResp, Index: uint64(i)})
			if !pr2.Inflights.Full() {
				t.Fatalf("#%d: inflights.full = %t, want %t", tt, pr2.Inflights.Full(), true)
			}
		}
	}
}

// TestMsgAppFlowControlRecvHeartbeat ensures a heartbeat response
// frees one slot if the window is full.
func TestMsgAppFlowControlRecvHeartbeat(t *testing.T) {
	r := newTestRaft(1, []uint64{1, 2}, 5, 1, NewMemoryStorage())
	r.becomeCandidate()
	r.becomeLeader()

	pr2 := r.prs.Progress[2]
	// force the progress to be in replicate state
	pr2.BecomeReplicate()
	// fill in the inflights window
	for i := 0; i < r.prs.MaxInflight; i++ {
		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
		r.readMessages()
	}

	for tt := 1; tt < 5; tt++ {
		if !pr2.Inflights.Full() {
			t.Fatalf("#%d: inflights.full = %t, want %t", tt, pr2.Inflights.Full(), true)
		}

		// recv tt msgHeartbeatResp and expect one free slot
		for i := 0; i < tt; i++ {
			r.Step(pb.Message{From: 2, To: 1, Type: pb.MsgHeartbeatResp})
			r.readMessages()
			if pr2.Inflights.Full() {
				t.Fatalf("#%d.%d: inflights.full = %t, want %t", tt, i, pr2.Inflights.Full(), false)
			}
		}

		// one slot
		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
		ms := r.readMessages()
		if len(ms) != 1 {
			t.Fatalf("#%d: free slot = 0, want 1", tt)
		}

		// and just one slot
		for i := 0; i < 10; i++ {
			r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})
			ms1 := r.readMessages()
			if len(ms1) != 0 {
				t.Fatalf("#%d.%d: len(ms) = %d, want 0", tt, i, len(ms1))
			}
		}

		// clear all pending messages.
		r.Step(pb.Message{From: 2, To: 1, Type: pb.MsgHeartbeatResp})
		r.readMessages()
	}
}
