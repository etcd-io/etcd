/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*
This file contains tests which verify that the scenarios described
in raft paper are handled by the raft implementation correctly.
Each test focuses on several sentences written in the paper. This could
help us to prevent most implementation bugs.

Each test is composed of three parts: init, test and check.
Init part uses simple and understandable way to simulate the init state.
Test part uses Step function to generate the scenario. Check part checks
outgoint messages and state.
*/
package raft

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	pb "github.com/coreos/etcd/raft/raftpb"
)

func TestFollowerUpdateTermFromMessage(t *testing.T) {
	testUpdateTermFromMessage(t, StateFollower)
}
func TestCandidateUpdateTermFromMessage(t *testing.T) {
	testUpdateTermFromMessage(t, StateCandidate)
}
func TestLeaderUpdateTermFromMessage(t *testing.T) {
	testUpdateTermFromMessage(t, StateLeader)
}

// testUpdateTermFromMessage tests that if one server’s current term is
// smaller than the other’s, then it updates its current term to the larger
// value. If a candidate or leader discovers that its term is out of date,
// it immediately reverts to follower state.
// Reference: section 5.1
func testUpdateTermFromMessage(t *testing.T, state StateType) {
	r := newRaft(1, []uint64{1, 2, 3}, 10, 1)
	switch state {
	case StateFollower:
		r.becomeFollower(1, 2)
	case StateCandidate:
		r.becomeCandidate()
	case StateLeader:
		r.becomeCandidate()
		r.becomeLeader()
	}

	r.Step(pb.Message{Type: pb.MsgApp, Term: 2})

	if r.Term != 2 {
		t.Errorf("term = %d, want %d", r.Term, 2)
	}
	if r.state != StateFollower {
		t.Errorf("state = %v, want %v", r.state, StateFollower)
	}
}

// TestRejectStaleTermMessage tests that if a server receives a request with
// a stale term number, it rejects the request.
// Our implementation ignores the request instead.
// Reference: section 5.1
func TestRejectStaleTermMessage(t *testing.T) {
	called := false
	fakeStep := func(r *raft, m pb.Message) {
		called = true
	}
	r := newRaft(1, []uint64{1, 2, 3}, 10, 1)
	r.step = fakeStep
	r.loadState(pb.HardState{Term: 2})

	r.Step(pb.Message{Type: pb.MsgApp, Term: r.Term - 1})

	if called == true {
		t.Errorf("stepFunc called = %v, want %v", called, false)
	}
}

// TestStartAsFollower tests that when servers start up, they begin as followers.
// Reference: section 5.2
func TestStartAsFollower(t *testing.T) {
	r := newRaft(1, []uint64{1, 2, 3}, 10, 1)
	if r.state != StateFollower {
		t.Errorf("state = %s, want %s", r.state, StateFollower)
	}
}

// TestLeaderBcastBeat tests that if the leader receives a heartbeat tick,
// it will send a msgApp with m.Index = 0, m.LogTerm=0 and empty entries as
// heartbeat to all followers.
// Reference: section 5.2
func TestLeaderBcastBeat(t *testing.T) {
	// heartbeat interval
	hi := 1
	r := newRaft(1, []uint64{1, 2, 3}, 10, hi)
	r.becomeCandidate()
	r.becomeLeader()
	for i := 0; i < 10; i++ {
		r.appendEntry(pb.Entry{})
	}

	for i := 0; i <= hi; i++ {
		r.tick()
	}

	msgs := r.ReadMessages()
	sort.Sort(messageSlice(msgs))
	wmsgs := []pb.Message{
		{From: 1, To: 2, Term: 1, Type: pb.MsgApp},
		{From: 1, To: 3, Term: 1, Type: pb.MsgApp},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Errorf("msgs = %v, want %v", msgs, wmsgs)
	}
}

func TestFollowerStartElection(t *testing.T) {
	testNonleaderStartElection(t, StateFollower)
}
func TestCandidateStartNewElection(t *testing.T) {
	testNonleaderStartElection(t, StateCandidate)
}

// testNonleaderStartElection tests that if a follower receives no communication
// over election timeout, it begins an election to choose a new leader. It
// increments its current term and transitions to candidate state. It then
// votes for itself and issues RequestVote RPCs in parallel to each of the
// other servers in the cluster.
// Reference: section 5.2
// Also if a candidate fails to obtain a majority, it will time out and
// start a new election by incrementing its term and initiating another
// round of RequestVote RPCs.
// Reference: section 5.2
func testNonleaderStartElection(t *testing.T, state StateType) {
	// election timeout
	et := 10
	r := newRaft(1, []uint64{1, 2, 3}, et, 1)
	switch state {
	case StateFollower:
		r.becomeFollower(1, 2)
	case StateCandidate:
		r.becomeCandidate()
	}

	for i := 0; i < 2*et; i++ {
		r.tick()
	}

	if r.Term != 2 {
		t.Errorf("term = %d, want 2", r.Term)
	}
	if r.state != StateCandidate {
		t.Errorf("state = %s, want %s", r.state, StateCandidate)
	}
	if r.votes[r.id] != true {
		t.Errorf("vote for self = false, want true")
	}
	msgs := r.ReadMessages()
	sort.Sort(messageSlice(msgs))
	wmsgs := []pb.Message{
		{From: 1, To: 2, Term: 2, Type: pb.MsgVote},
		{From: 1, To: 3, Term: 2, Type: pb.MsgVote},
	}
	if !reflect.DeepEqual(msgs, wmsgs) {
		t.Errorf("msgs = %v, want %v", msgs, wmsgs)
	}
}

// TestLeaderElectionInOneRoundRPC tests all cases that may happen in
// leader election during one round of RequestVote RPC:
// a) it wins the election
// b) it loses the election
// c) it is unclear about the result
// Reference: section 5.2
func TestLeaderElectionInOneRoundRPC(t *testing.T) {
	tests := []struct {
		size  int
		votes map[uint64]bool
		state StateType
	}{
		// win the election when receiving votes from a majority of the servers
		{1, map[uint64]bool{}, StateLeader},
		{3, map[uint64]bool{2: true, 3: true}, StateLeader},
		{3, map[uint64]bool{2: true}, StateLeader},
		{5, map[uint64]bool{2: true, 3: true, 4: true, 5: true}, StateLeader},
		{5, map[uint64]bool{2: true, 3: true, 4: true}, StateLeader},
		{5, map[uint64]bool{2: true, 3: true}, StateLeader},

		// return to follower state if it receives vote denial from a majority
		{3, map[uint64]bool{2: false, 3: false}, StateFollower},
		{5, map[uint64]bool{2: false, 3: false, 4: false, 5: false}, StateFollower},
		{5, map[uint64]bool{2: true, 3: false, 4: false, 5: false}, StateFollower},

		// stay in candidate if it does not obtain the majority
		{3, map[uint64]bool{}, StateCandidate},
		{5, map[uint64]bool{2: true}, StateCandidate},
		{5, map[uint64]bool{2: false, 3: false}, StateCandidate},
		{5, map[uint64]bool{}, StateCandidate},
	}
	for i, tt := range tests {
		r := newRaft(1, idsBySize(tt.size), 10, 1)

		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgHup})
		for id, vote := range tt.votes {
			r.Step(pb.Message{From: id, To: 1, Type: pb.MsgVoteResp, Reject: !vote})
		}

		if r.state != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, r.state, tt.state)
		}
		if g := r.Term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

// TestFollowerVote tests that each follower will vote for at most one
// candidate in a given term, on a first-come-first-served basis.
// TODO: (note: Section 5.4 adds an additional restriction on votes).
// Reference: section 5.2
func TestFollowerVote(t *testing.T) {
	tests := []struct {
		vote    uint64
		nvote   uint64
		wreject bool
	}{
		{None, 1, false},
		{None, 2, false},
		{1, 1, false},
		{2, 2, false},
		{1, 2, true},
		{2, 1, true},
	}
	for i, tt := range tests {
		r := newRaft(1, []uint64{1, 2, 3}, 10, 1)
		r.loadState(pb.HardState{Term: 1, Vote: tt.vote})

		r.Step(pb.Message{From: tt.nvote, To: 1, Term: 1, Type: pb.MsgVote})

		msgs := r.ReadMessages()
		wmsgs := []pb.Message{
			{From: 1, To: tt.nvote, Term: 1, Type: pb.MsgVoteResp, Reject: tt.wreject},
		}
		if !reflect.DeepEqual(msgs, wmsgs) {
			t.Errorf("#%d: msgs = %v, want %v", i, msgs, wmsgs)
		}
	}
}

// TestCandidateFallback tests that while waiting for votes,
// if a candidate receives an AppendEntries RPC from another server claiming
// to be leader whose term is at least as large as the candidate's current term,
// it recognizes the leader as legitimate and returns to follower state.
// Reference: section 5.2
func TestCandidateFallback(t *testing.T) {
	tests := []pb.Message{
		{From: 2, To: 1, Term: 1, Type: pb.MsgApp},
		{From: 2, To: 1, Term: 2, Type: pb.MsgApp},
	}
	for i, tt := range tests {
		r := newRaft(1, []uint64{1, 2, 3}, 10, 1)
		r.Step(pb.Message{From: 1, To: 1, Type: pb.MsgHup})
		if r.state != StateCandidate {
			t.Fatalf("unexpected state = %s, want %s", r.state, StateCandidate)
		}

		r.Step(tt)

		if g := r.state; g != StateFollower {
			t.Errorf("#%d: state = %s, want %s", i, g, StateFollower)
		}
		if g := r.Term; g != tt.Term {
			t.Errorf("#%d: term = %d, want %d", i, g, tt.Term)
		}
	}
}

func TestFollowerElectionTimeoutRandomized(t *testing.T) {
	testNonleaderElectionTimeoutRandomized(t, StateFollower)
}
func TestCandidateElectionTimeoutRandomized(t *testing.T) {
	testNonleaderElectionTimeoutRandomized(t, StateCandidate)
}

// testNonleaderElectionTimeoutRandomized tests that election timeout for
// follower or candidate is randomized.
// Reference: section 5.2
func testNonleaderElectionTimeoutRandomized(t *testing.T, state StateType) {
	et := 10
	r := newRaft(1, []uint64{1, 2, 3}, et, 1)
	timeouts := make(map[int]bool)
	for round := 0; round < 50*et; round++ {
		switch state {
		case StateFollower:
			r.becomeFollower(r.Term+1, 2)
		case StateCandidate:
			r.becomeCandidate()
		}

		time := 0
		for len(r.ReadMessages()) == 0 {
			r.tick()
			time++
		}
		timeouts[time] = true
	}

	for d := et + 1; d < 2*et; d++ {
		if timeouts[d] != true {
			t.Errorf("timeout in %d ticks should happen", d)
		}
	}
}

func TestFollowersElectioinTimeoutNonconflict(t *testing.T) {
	testNonleadersElectionTimeoutNonconflict(t, StateFollower)
}
func TestCandidatesElectionTimeoutNonconflict(t *testing.T) {
	testNonleadersElectionTimeoutNonconflict(t, StateCandidate)
}

// testNonleadersElectionTimeoutNonconflict tests that in most cases only a
// single server(follower or candidate) will time out, which reduces the
// likelihood of split vote in the new election.
// Reference: section 5.2
func testNonleadersElectionTimeoutNonconflict(t *testing.T, state StateType) {
	et := 10
	size := 5
	rs := make([]*raft, size)
	ids := idsBySize(size)
	for k := range rs {
		rs[k] = newRaft(ids[k], ids, et, 1)
	}
	conflicts := 0
	for round := 0; round < 1000; round++ {
		for _, r := range rs {
			switch state {
			case StateFollower:
				r.becomeFollower(r.Term+1, None)
			case StateCandidate:
				r.becomeCandidate()
			}
		}

		timeoutNum := 0
		for timeoutNum == 0 {
			for _, r := range rs {
				r.tick()
				if len(r.ReadMessages()) > 0 {
					timeoutNum++
				}
			}
		}
		// several rafts time out at the same tick
		if timeoutNum > 1 {
			conflicts++
		}
	}

	if g := float64(conflicts) / 1000; g > 0.4 {
		t.Errorf("probability of conflicts = %v, want <= 0.4", g)
	}
}

type messageSlice []pb.Message

func (s messageSlice) Len() int           { return len(s) }
func (s messageSlice) Less(i, j int) bool { return fmt.Sprint(s[i]) < fmt.Sprint(s[j]) }
func (s messageSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
