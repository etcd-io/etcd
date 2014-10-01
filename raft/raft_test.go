package raft

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	pb "github.com/coreos/etcd/raft/raftpb"
)

// nextEnts returns the appliable entries and updates the applied index
func nextEnts(r *raft) (ents []pb.Entry) {
	ents = r.raftLog.nextEnts()
	r.raftLog.resetNextEnts()
	return ents
}

type Interface interface {
	Step(m pb.Message) error
	ReadMessages() []pb.Message
}

func TestLeaderElection(t *testing.T) {
	tests := []struct {
		*network
		state StateType
	}{
		{newNetwork(nil, nil, nil), StateLeader},
		{newNetwork(nil, nil, nopStepper), StateLeader},
		{newNetwork(nil, nopStepper, nopStepper), StateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil), StateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil, nil), StateLeader},

		// three logs further along than 0
		{newNetwork(nil, ents(1), ents(2), ents(1, 3), nil), StateFollower},

		// logs converge
		{newNetwork(ents(1), nil, ents(2), ents(1), nil), StateLeader},
	}

	for i, tt := range tests {
		tt.send(pb.Message{From: 1, To: 1, Type: msgHup})
		sm := tt.network.peers[1].(*raft)
		if sm.state != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, sm.state, tt.state)
		}
		if g := sm.Term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestLogReplication(t *testing.T) {
	tests := []struct {
		*network
		msgs       []pb.Message
		wcommitted int64
	}{
		{
			newNetwork(nil, nil, nil),
			[]pb.Message{
				{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}},
			},
			2,
		},
		{
			newNetwork(nil, nil, nil),
			[]pb.Message{
				{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}},
				{From: 1, To: 2, Type: msgHup},
				{From: 1, To: 2, Type: msgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}},
			},
			4,
		},
	}

	for i, tt := range tests {
		tt.send(pb.Message{From: 1, To: 1, Type: msgHup})

		for _, m := range tt.msgs {
			tt.send(m)
		}

		for j, x := range tt.network.peers {
			sm := x.(*raft)

			if sm.raftLog.committed != tt.wcommitted {
				t.Errorf("#%d.%d: committed = %d, want %d", i, j, sm.raftLog.committed, tt.wcommitted)
			}

			ents := make([]pb.Entry, 0)
			for _, e := range nextEnts(sm) {
				if e.Data != nil {
					ents = append(ents, e)
				}
			}
			props := make([]pb.Message, 0)
			for _, m := range tt.msgs {
				if m.Type == msgProp {
					props = append(props, m)
				}
			}
			for k, m := range props {
				if !bytes.Equal(ents[k].Data, m.Entries[0].Data) {
					t.Errorf("#%d.%d: data = %d, want %d", i, j, ents[k].Data, m.Entries[0].Data)
				}
			}
		}
	}
}

func TestSingleNodeCommit(t *testing.T) {
	tt := newNetwork(nil)
	tt.send(pb.Message{From: 1, To: 1, Type: msgHup})
	tt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("some data")}}})
	tt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("some data")}}})

	sm := tt.peers[1].(*raft)
	if sm.raftLog.committed != 3 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 3)
	}
}

// TestCannotCommitWithoutNewTermEntry tests the entries cannot be committed
// when leader changes, no new proposal comes in and ChangeTerm proposal is
// filtered.
func TestCannotCommitWithoutNewTermEntry(t *testing.T) {
	tt := newNetwork(nil, nil, nil, nil, nil)
	tt.send(pb.Message{From: 1, To: 1, Type: msgHup})

	// 0 cannot reach 2,3,4
	tt.cut(1, 3)
	tt.cut(1, 4)
	tt.cut(1, 5)

	tt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("some data")}}})
	tt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("some data")}}})

	sm := tt.peers[1].(*raft)
	if sm.raftLog.committed != 1 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 1)
	}

	// network recovery
	tt.recover()
	// avoid committing ChangeTerm proposal
	tt.ignore(msgApp)

	// elect 1 as the new leader with term 2
	tt.send(pb.Message{From: 2, To: 2, Type: msgHup})

	// no log entries from previous term should be committed
	sm = tt.peers[2].(*raft)
	if sm.raftLog.committed != 1 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 1)
	}

	tt.recover()

	// still be able to append a entry
	tt.send(pb.Message{From: 2, To: 2, Type: msgProp, Entries: []pb.Entry{{Data: []byte("some data")}}})

	if sm.raftLog.committed != 5 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 5)
	}
}

// TestCommitWithoutNewTermEntry tests the entries could be committed
// when leader changes, no new proposal comes in.
func TestCommitWithoutNewTermEntry(t *testing.T) {
	tt := newNetwork(nil, nil, nil, nil, nil)
	tt.send(pb.Message{From: 1, To: 1, Type: msgHup})

	// 0 cannot reach 2,3,4
	tt.cut(1, 3)
	tt.cut(1, 4)
	tt.cut(1, 5)

	tt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("some data")}}})
	tt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: []byte("some data")}}})

	sm := tt.peers[1].(*raft)
	if sm.raftLog.committed != 1 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 1)
	}

	// network recovery
	tt.recover()

	// elect 1 as the new leader with term 2
	// after append a ChangeTerm entry from the current term, all entries
	// should be committed
	tt.send(pb.Message{From: 2, To: 2, Type: msgHup})

	if sm.raftLog.committed != 4 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 4)
	}
}

func TestDuelingCandidates(t *testing.T) {
	a := newRaft(-1, nil, 0, 0) // k, id are set later
	b := newRaft(-1, nil, 0, 0)
	c := newRaft(-1, nil, 0, 0)

	nt := newNetwork(a, b, c)
	nt.cut(1, 3)

	nt.send(pb.Message{From: 1, To: 1, Type: msgHup})
	nt.send(pb.Message{From: 3, To: 3, Type: msgHup})

	nt.recover()
	nt.send(pb.Message{From: 3, To: 3, Type: msgHup})

	wlog := &raftLog{ents: []pb.Entry{{}, pb.Entry{Data: nil, Term: 1, Index: 1}}, committed: 1}
	tests := []struct {
		sm      *raft
		state   StateType
		term    int64
		raftLog *raftLog
	}{
		{a, StateFollower, 2, wlog},
		{b, StateFollower, 2, wlog},
		{c, StateFollower, 2, newLog()},
	}

	for i, tt := range tests {
		if g := tt.sm.state; g != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, g, tt.state)
		}
		if g := tt.sm.Term; g != tt.term {
			t.Errorf("#%d: term = %d, want %d", i, g, tt.term)
		}
		base := ltoa(tt.raftLog)
		if sm, ok := nt.peers[1+int64(i)].(*raft); ok {
			l := ltoa(sm.raftLog)
			if g := diffu(base, l); g != "" {
				t.Errorf("#%d: diff:\n%s", i, g)
			}
		} else {
			t.Logf("#%d: empty log", i)
		}
	}
}

func TestCandidateConcede(t *testing.T) {
	tt := newNetwork(nil, nil, nil)
	tt.isolate(1)

	tt.send(pb.Message{From: 1, To: 1, Type: msgHup})
	tt.send(pb.Message{From: 3, To: 3, Type: msgHup})

	// heal the partition
	tt.recover()

	data := []byte("force follower")
	// send a proposal to 2 to flush out a msgApp to 0
	tt.send(pb.Message{From: 3, To: 3, Type: msgProp, Entries: []pb.Entry{{Data: data}}})

	a := tt.peers[1].(*raft)
	if g := a.state; g != StateFollower {
		t.Errorf("state = %s, want %s", g, StateFollower)
	}
	if g := a.Term; g != 1 {
		t.Errorf("term = %d, want %d", g, 1)
	}
	wantLog := ltoa(&raftLog{ents: []pb.Entry{{}, {Data: nil, Term: 1, Index: 1}, {Term: 1, Index: 2, Data: data}}, committed: 2})
	for i, p := range tt.peers {
		if sm, ok := p.(*raft); ok {
			l := ltoa(sm.raftLog)
			if g := diffu(wantLog, l); g != "" {
				t.Errorf("#%d: diff:\n%s", i, g)
			}
		} else {
			t.Logf("#%d: empty log", i)
		}
	}
}

func TestSingleNodeCandidate(t *testing.T) {
	tt := newNetwork(nil)
	tt.send(pb.Message{From: 1, To: 1, Type: msgHup})

	sm := tt.peers[1].(*raft)
	if sm.state != StateLeader {
		t.Errorf("state = %d, want %d", sm.state, StateLeader)
	}
}

func TestOldMessages(t *testing.T) {
	tt := newNetwork(nil, nil, nil)
	// make 0 leader @ term 3
	tt.send(pb.Message{From: 1, To: 1, Type: msgHup})
	tt.send(pb.Message{From: 2, To: 2, Type: msgHup})
	tt.send(pb.Message{From: 1, To: 1, Type: msgHup})
	// pretend we're an old leader trying to make progress
	tt.send(pb.Message{From: 1, To: 1, Type: msgApp, Term: 1, Entries: []pb.Entry{{Term: 1}}})

	l := &raftLog{
		ents: []pb.Entry{
			{}, {Data: nil, Term: 1, Index: 1},
			{Data: nil, Term: 2, Index: 2}, {Data: nil, Term: 3, Index: 3},
		},
		committed: 3,
	}
	base := ltoa(l)
	for i, p := range tt.peers {
		if sm, ok := p.(*raft); ok {
			l := ltoa(sm.raftLog)
			if g := diffu(base, l); g != "" {
				t.Errorf("#%d: diff:\n%s", i, g)
			}
		} else {
			t.Logf("#%d: empty log", i)
		}
	}
}

// TestOldMessagesReply - optimization - reply with new term.

func TestProposal(t *testing.T) {
	tests := []struct {
		*network
		success bool
	}{
		{newNetwork(nil, nil, nil), true},
		{newNetwork(nil, nil, nopStepper), true},
		{newNetwork(nil, nopStepper, nopStepper), false},
		{newNetwork(nil, nopStepper, nopStepper, nil), false},
		{newNetwork(nil, nopStepper, nopStepper, nil, nil), true},
	}

	for i, tt := range tests {
		send := func(m pb.Message) {
			defer func() {
				// only recover is we expect it to panic so
				// panics we don't expect go up.
				if !tt.success {
					e := recover()
					if e != nil {
						t.Logf("#%d: err: %s", i, e)
					}
				}
			}()
			tt.send(m)
		}

		data := []byte("somedata")

		// promote 0 the leader
		send(pb.Message{From: 1, To: 1, Type: msgHup})
		send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Data: data}}})

		wantLog := newLog()
		if tt.success {
			wantLog = &raftLog{ents: []pb.Entry{{}, {Data: nil, Term: 1, Index: 1}, {Term: 1, Index: 2, Data: data}}, committed: 2}
		}
		base := ltoa(wantLog)
		for i, p := range tt.peers {
			if sm, ok := p.(*raft); ok {
				l := ltoa(sm.raftLog)
				if g := diffu(base, l); g != "" {
					t.Errorf("#%d: diff:\n%s", i, g)
				}
			} else {
				t.Logf("#%d: empty log", i)
			}
		}
		sm := tt.network.peers[1].(*raft)
		if g := sm.Term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestProposalByProxy(t *testing.T) {
	data := []byte("somedata")
	tests := []*network{
		newNetwork(nil, nil, nil),
		newNetwork(nil, nil, nopStepper),
	}

	for i, tt := range tests {
		// promote 0 the leader
		tt.send(pb.Message{From: 1, To: 1, Type: msgHup})

		// propose via follower
		tt.send(pb.Message{From: 2, To: 2, Type: msgProp, Entries: []pb.Entry{{Data: []byte("somedata")}}})

		wantLog := &raftLog{ents: []pb.Entry{{}, {Data: nil, Term: 1, Index: 1}, {Term: 1, Data: data, Index: 2}}, committed: 2}
		base := ltoa(wantLog)
		for i, p := range tt.peers {
			if sm, ok := p.(*raft); ok {
				l := ltoa(sm.raftLog)
				if g := diffu(base, l); g != "" {
					t.Errorf("#%d: diff:\n%s", i, g)
				}
			} else {
				t.Logf("#%d: empty log", i)
			}
		}
		sm := tt.peers[1].(*raft)
		if g := sm.Term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestCommit(t *testing.T) {
	tests := []struct {
		matches []int64
		logs    []pb.Entry
		smTerm  int64
		w       int64
	}{
		// single
		{[]int64{1}, []pb.Entry{{}, {Term: 1}}, 1, 1},
		{[]int64{1}, []pb.Entry{{}, {Term: 1}}, 2, 0},
		{[]int64{2}, []pb.Entry{{}, {Term: 1}, {Term: 2}}, 2, 2},
		{[]int64{1}, []pb.Entry{{}, {Term: 2}}, 2, 1},

		// odd
		{[]int64{2, 1, 1}, []pb.Entry{{}, {Term: 1}, {Term: 2}}, 1, 1},
		{[]int64{2, 1, 1}, []pb.Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
		{[]int64{2, 1, 2}, []pb.Entry{{}, {Term: 1}, {Term: 2}}, 2, 2},
		{[]int64{2, 1, 2}, []pb.Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},

		// even
		{[]int64{2, 1, 1, 1}, []pb.Entry{{}, {Term: 1}, {Term: 2}}, 1, 1},
		{[]int64{2, 1, 1, 1}, []pb.Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
		{[]int64{2, 1, 1, 2}, []pb.Entry{{}, {Term: 1}, {Term: 2}}, 1, 1},
		{[]int64{2, 1, 1, 2}, []pb.Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
		{[]int64{2, 1, 2, 2}, []pb.Entry{{}, {Term: 1}, {Term: 2}}, 2, 2},
		{[]int64{2, 1, 2, 2}, []pb.Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
	}

	for i, tt := range tests {
		prs := make(map[int64]*progress)
		for j := 0; j < len(tt.matches); j++ {
			prs[int64(j)] = &progress{tt.matches[j], tt.matches[j] + 1}
		}
		sm := &raft{raftLog: &raftLog{ents: tt.logs}, prs: prs, HardState: pb.HardState{Term: tt.smTerm}}
		sm.maybeCommit()
		if g := sm.raftLog.committed; g != tt.w {
			t.Errorf("#%d: committed = %d, want %d", i, g, tt.w)
		}
	}
}

// ensure that the Step function ignores the message from old term and does not pass it to the
// acutal stepX function.
func TestStepIgnoreOldTermMsg(t *testing.T) {
	called := false
	fakeStep := func(r *raft, m pb.Message) {
		called = true
	}
	sm := newRaft(1, []int64{1}, 0, 0)
	sm.step = fakeStep
	sm.Term = 2
	sm.Step(pb.Message{Type: msgApp, Term: sm.Term - 1})
	if called == true {
		t.Errorf("stepFunc called = %v , want %v", called, false)
	}
}

// TestHandleMsgApp ensures:
// 1. Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm.
// 2. If an existing entry conflicts with a new one (same index but different terms),
//    delete the existing entry and all that follow it; append any new entries not already in the log.
// 3. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry).
func TestHandleMsgApp(t *testing.T) {
	tests := []struct {
		m       pb.Message
		wIndex  int64
		wCommit int64
		wReject bool
	}{
		// Ensure 1
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 3, Index: 2, Commit: 3}, 2, 0, true}, // previous log mismatch
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 3, Index: 3, Commit: 3}, 2, 0, true}, // previous log non-exist

		// Ensure 2
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 1, Index: 1, Commit: 1}, 2, 1, false},
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 0, Index: 0, Commit: 1, Entries: []pb.Entry{{Term: 2}}}, 1, 1, false},
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 3, Entries: []pb.Entry{{Term: 2}, {Term: 2}}}, 4, 3, false},
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 4, Entries: []pb.Entry{{Term: 2}}}, 3, 3, false},
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 1, Index: 1, Commit: 4, Entries: []pb.Entry{{Term: 2}}}, 2, 2, false},

		// Ensure 3
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 2}, 2, 2, false},
		{pb.Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 4}, 2, 2, false}, // commit upto min(commit, last)
	}

	for i, tt := range tests {
		sm := &raft{
			state:     StateFollower,
			HardState: pb.HardState{Term: 2},
			raftLog:   &raftLog{committed: 0, ents: []pb.Entry{{}, {Term: 1}, {Term: 2}}},
		}

		sm.handleAppendEntries(tt.m)
		if sm.raftLog.lastIndex() != tt.wIndex {
			t.Errorf("#%d: lastIndex = %d, want %d", i, sm.raftLog.lastIndex(), tt.wIndex)
		}
		if sm.raftLog.committed != tt.wCommit {
			t.Errorf("#%d: committed = %d, want %d", i, sm.raftLog.committed, tt.wCommit)
		}
		m := sm.ReadMessages()
		if len(m) != 1 {
			t.Fatalf("#%d: msg = nil, want 1", i)
		}
		if m[0].Reject != tt.wReject {
			t.Errorf("#%d: reject = %v, want %v", i, m[0].Reject, tt.wReject)
		}
	}
}

func TestRecvMsgVote(t *testing.T) {
	tests := []struct {
		state   StateType
		i, term int64
		voteFor int64
		wreject bool
	}{
		{StateFollower, 0, 0, None, true},
		{StateFollower, 0, 1, None, true},
		{StateFollower, 0, 2, None, true},
		{StateFollower, 0, 3, None, false},

		{StateFollower, 1, 0, None, true},
		{StateFollower, 1, 1, None, true},
		{StateFollower, 1, 2, None, true},
		{StateFollower, 1, 3, None, false},

		{StateFollower, 2, 0, None, true},
		{StateFollower, 2, 1, None, true},
		{StateFollower, 2, 2, None, false},
		{StateFollower, 2, 3, None, false},

		{StateFollower, 3, 0, None, true},
		{StateFollower, 3, 1, None, true},
		{StateFollower, 3, 2, None, false},
		{StateFollower, 3, 3, None, false},

		{StateFollower, 3, 2, 2, false},
		{StateFollower, 3, 2, 1, true},

		{StateLeader, 3, 3, 1, true},
		{StateCandidate, 3, 3, 1, true},
	}

	for i, tt := range tests {
		sm := newRaft(1, []int64{1}, 0, 0)
		sm.state = tt.state
		switch tt.state {
		case StateFollower:
			sm.step = stepFollower
		case StateCandidate:
			sm.step = stepCandidate
		case StateLeader:
			sm.step = stepLeader
		}
		sm.HardState = pb.HardState{Vote: tt.voteFor}
		sm.raftLog = &raftLog{ents: []pb.Entry{{}, {Term: 2}, {Term: 2}}}

		sm.Step(pb.Message{Type: msgVote, From: 2, Index: tt.i, LogTerm: tt.term})

		msgs := sm.ReadMessages()
		if g := len(msgs); g != 1 {
			t.Fatalf("#%d: len(msgs) = %d, want 1", i, g)
			continue
		}
		if g := msgs[0].Reject; g != tt.wreject {
			t.Errorf("#%d, m.Reject = %d, want %d", i, g, tt.wreject)
		}
	}
}

func TestStateTransition(t *testing.T) {
	tests := []struct {
		from   StateType
		to     StateType
		wallow bool
		wterm  int64
		wlead  int64
	}{
		{StateFollower, StateFollower, true, 1, None},
		{StateFollower, StateCandidate, true, 1, None},
		{StateFollower, StateLeader, false, -1, None},

		{StateCandidate, StateFollower, true, 0, None},
		{StateCandidate, StateCandidate, true, 1, None},
		{StateCandidate, StateLeader, true, 0, 1},

		{StateLeader, StateFollower, true, 1, None},
		{StateLeader, StateCandidate, false, 1, None},
		{StateLeader, StateLeader, true, 0, 1},
	}

	for i, tt := range tests {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if tt.wallow == true {
						t.Errorf("%d: allow = %v, want %v", i, false, true)
					}
				}
			}()

			sm := newRaft(1, []int64{1}, 0, 0)
			sm.state = tt.from

			switch tt.to {
			case StateFollower:
				sm.becomeFollower(tt.wterm, tt.wlead)
			case StateCandidate:
				sm.becomeCandidate()
			case StateLeader:
				sm.becomeLeader()
			}

			if sm.Term != tt.wterm {
				t.Errorf("%d: term = %d, want %d", i, sm.Term, tt.wterm)
			}
			if sm.lead != tt.wlead {
				t.Errorf("%d: lead = %d, want %d", i, sm.lead, tt.wlead)
			}
		}()
	}
}

func TestAllServerStepdown(t *testing.T) {
	tests := []struct {
		state StateType

		wstate StateType
		wterm  int64
		windex int64
	}{
		{StateFollower, StateFollower, 3, 1},
		{StateCandidate, StateFollower, 3, 1},
		{StateLeader, StateFollower, 3, 2},
	}

	tmsgTypes := [...]int64{msgVote, msgApp}
	tterm := int64(3)

	for i, tt := range tests {
		sm := newRaft(1, []int64{1, 2, 3}, 0, 0)
		switch tt.state {
		case StateFollower:
			sm.becomeFollower(1, None)
		case StateCandidate:
			sm.becomeCandidate()
		case StateLeader:
			sm.becomeCandidate()
			sm.becomeLeader()
		}

		for j, msgType := range tmsgTypes {
			sm.Step(pb.Message{From: 2, Type: msgType, Term: tterm, LogTerm: tterm})

			if sm.state != tt.wstate {
				t.Errorf("#%d.%d state = %v , want %v", i, j, sm.state, tt.wstate)
			}
			if sm.Term != tt.wterm {
				t.Errorf("#%d.%d term = %v , want %v", i, j, sm.Term, tt.wterm)
			}
			if int64(len(sm.raftLog.ents)) != tt.windex {
				t.Errorf("#%d.%d index = %v , want %v", i, j, len(sm.raftLog.ents), tt.windex)
			}
			wlead := int64(2)
			if msgType == msgVote {
				wlead = None
			}
			if sm.lead != wlead {
				t.Errorf("#%d, sm.lead = %d, want %d", i, sm.lead, None)
			}
		}
	}
}

func TestLeaderAppResp(t *testing.T) {
	tests := []struct {
		index      int64
		reject     bool
		wmsgNum    int
		windex     int64
		wcommitted int64
	}{
		{-1, true, 1, 1, 0}, // bad resp; leader does not commit; reply with log entries
		{2, false, 2, 2, 2}, // good resp; leader commits; broadcast with commit index
	}

	for i, tt := range tests {
		// sm term is 1 after it becomes the leader.
		// thus the last log term must be 1 to be committed.
		sm := newRaft(1, []int64{1, 2, 3}, 0, 0)
		sm.raftLog = &raftLog{ents: []pb.Entry{{}, {Term: 0}, {Term: 1}}}
		sm.becomeCandidate()
		sm.becomeLeader()
		sm.ReadMessages()
		sm.Step(pb.Message{From: 2, Type: msgAppResp, Index: tt.index, Term: sm.Term, Reject: tt.reject})
		msgs := sm.ReadMessages()

		if len(msgs) != tt.wmsgNum {
			t.Errorf("#%d msgNum = %d, want %d", i, len(msgs), tt.wmsgNum)
		}
		for j, msg := range msgs {
			if msg.Index != tt.windex {
				t.Errorf("#%d.%d index = %d, want %d", i, j, msg.Index, tt.windex)
			}
			if msg.Commit != tt.wcommitted {
				t.Errorf("#%d.%d commit = %d, want %d", i, j, msg.Commit, tt.wcommitted)
			}
		}
	}
}

// When the leader receives a heartbeat tick, it should
// send a msgApp with m.Index = 0, m.LogTerm=0 and empty entries.
func TestBcastBeat(t *testing.T) {
	offset := int64(1000)
	// make a state machine with log.offset = 1000
	s := pb.Snapshot{
		Index: offset,
		Term:  1,
		Nodes: []int64{1, 2, 3},
	}
	sm := newRaft(1, []int64{1, 2, 3}, 0, 0)
	sm.Term = 1
	sm.restore(s)

	sm.becomeCandidate()
	sm.becomeLeader()
	for i := 0; i < 10; i++ {
		sm.appendEntry(pb.Entry{})
	}

	sm.Step(pb.Message{Type: msgBeat})
	msgs := sm.ReadMessages()
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %v, want 1", len(msgs))
	}
	tomap := map[int64]bool{2: true, 3: true}
	for i, m := range msgs {
		if m.Type != msgApp {
			t.Fatalf("#%d: type = %v, want = %v", i, m.Type, msgApp)
		}
		if m.Index != 0 {
			t.Fatalf("#%d: prevIndex = %d, want %d", i, m.Index, 0)
		}
		if m.LogTerm != 0 {
			t.Fatalf("#%d: prevTerm = %d, want %d", i, m.LogTerm, 0)
		}
		if !tomap[m.To] {
			t.Fatalf("#%d: unexpected to %d", i, m.To)
		} else {
			delete(tomap, m.To)
		}
		if len(m.Entries) != 0 {
			t.Fatalf("#%d: len(entries) = %d, want 0", i, len(m.Entries))
		}
	}
}

// tests the output of the statemachine when receiving msgBeat
func TestRecvMsgBeat(t *testing.T) {
	tests := []struct {
		state StateType
		wMsg  int
	}{
		{StateLeader, 2},
		// candidate and follower should ignore msgBeat
		{StateCandidate, 0},
		{StateFollower, 0},
	}

	for i, tt := range tests {
		sm := newRaft(1, []int64{1, 2, 3}, 0, 0)
		sm.raftLog = &raftLog{ents: []pb.Entry{{}, {Term: 0}, {Term: 1}}}
		sm.Term = 1
		sm.state = tt.state
		switch tt.state {
		case StateFollower:
			sm.step = stepFollower
		case StateCandidate:
			sm.step = stepCandidate
		case StateLeader:
			sm.step = stepLeader
		}
		sm.Step(pb.Message{From: 1, To: 1, Type: msgBeat})

		msgs := sm.ReadMessages()
		if len(msgs) != tt.wMsg {
			t.Errorf("%d: len(msgs) = %d, want %d", i, len(msgs), tt.wMsg)
		}
		for _, m := range msgs {
			if m.Type != msgApp {
				t.Errorf("%d: msg.type = %v, want %v", i, m.Type, msgApp)
			}
		}
	}
}

func TestRestore(t *testing.T) {
	s := pb.Snapshot{
		Index: defaultCompactThreshold + 1,
		Term:  defaultCompactThreshold + 1,
		Nodes: []int64{1, 2, 3},
	}

	sm := newRaft(1, []int64{1, 2}, 0, 0)
	if ok := sm.restore(s); !ok {
		t.Fatal("restore fail, want succeed")
	}

	if sm.raftLog.lastIndex() != s.Index {
		t.Errorf("log.lastIndex = %d, want %d", sm.raftLog.lastIndex(), s.Index)
	}
	if sm.raftLog.term(s.Index) != s.Term {
		t.Errorf("log.lastTerm = %d, want %d", sm.raftLog.term(s.Index), s.Term)
	}
	sg := int64Slice(sm.nodes())
	sw := int64Slice(s.Nodes)
	sort.Sort(sg)
	sort.Sort(sw)
	if !reflect.DeepEqual(sg, sw) {
		t.Errorf("sm.Nodes = %+v, want %+v", sg, sw)
	}
	if !reflect.DeepEqual(sm.raftLog.snapshot, s) {
		t.Errorf("snapshot = %+v, want %+v", sm.raftLog.snapshot, s)
	}

	if ok := sm.restore(s); ok {
		t.Fatal("restore succeed, want fail")
	}
}

func TestProvideSnap(t *testing.T) {
	s := pb.Snapshot{
		Index: defaultCompactThreshold + 1,
		Term:  defaultCompactThreshold + 1,
		Nodes: []int64{1, 2},
	}
	sm := newRaft(1, []int64{1}, 0, 0)
	// restore the statemachin from a snapshot
	// so it has a compacted log and a snapshot
	sm.restore(s)

	sm.becomeCandidate()
	sm.becomeLeader()

	// force set the next of node 1, so that
	// node 1 needs a snapshot
	sm.prs[2].next = sm.raftLog.offset

	sm.Step(pb.Message{From: 2, To: 1, Type: msgAppResp, Index: -1, Reject: true})
	msgs := sm.ReadMessages()
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	m := msgs[0]
	if m.Type != msgSnap {
		t.Errorf("m.Type = %v, want %v", m.Type, msgSnap)
	}
}

func TestRestoreFromSnapMsg(t *testing.T) {
	s := pb.Snapshot{
		Index: defaultCompactThreshold + 1,
		Term:  defaultCompactThreshold + 1,
		Nodes: []int64{1, 2},
	}
	m := pb.Message{Type: msgSnap, From: 1, Term: 2, Snapshot: s}

	sm := newRaft(2, []int64{1, 2}, 0, 0)
	sm.Step(m)

	if !reflect.DeepEqual(sm.raftLog.snapshot, s) {
		t.Errorf("snapshot = %+v, want %+v", sm.raftLog.snapshot, s)
	}
}

func TestSlowNodeRestore(t *testing.T) {
	nt := newNetwork(nil, nil, nil)
	nt.send(pb.Message{From: 1, To: 1, Type: msgHup})

	nt.isolate(3)
	for j := 0; j < defaultCompactThreshold+1; j++ {
		nt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{}}})
	}
	lead := nt.peers[1].(*raft)
	nextEnts(lead)
	lead.compact(nil)

	nt.recover()
	// trigger a snapshot
	nt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{}}})
	follower := nt.peers[3].(*raft)
	if !reflect.DeepEqual(follower.raftLog.snapshot, lead.raftLog.snapshot) {
		t.Errorf("follower.snap = %+v, want %+v", follower.raftLog.snapshot, lead.raftLog.snapshot)
	}

	// trigger a commit
	nt.send(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{}}})
	if follower.raftLog.committed != lead.raftLog.committed {
		t.Errorf("follower.comitted = %d, want %d", follower.raftLog.committed, lead.raftLog.committed)
	}
}

// TestStepConfig tests that when raft step msgProp in EntryConfChange type,
// it appends the entry to log and sets pendingConf to be true.
func TestStepConfig(t *testing.T) {
	// a raft that cannot make progress
	r := newRaft(1, []int64{1, 2}, 0, 0)
	r.becomeCandidate()
	r.becomeLeader()
	index := r.raftLog.lastIndex()
	r.Step(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Type: pb.EntryConfChange}}})
	if g := r.raftLog.lastIndex(); g != index+1 {
		t.Errorf("index = %d, want %d", g, index+1)
	}
	if r.pendingConf != true {
		t.Errorf("pendingConf = %v, want true", r.pendingConf)
	}
}

// TestStepIgnoreConfig tests that if raft step the second msgProp in
// EntryConfChange type when the first one is uncommitted, the node will deny
// the proposal and keep its original state.
func TestStepIgnoreConfig(t *testing.T) {
	// a raft that cannot make progress
	r := newRaft(1, []int64{1, 2}, 0, 0)
	r.becomeCandidate()
	r.becomeLeader()
	r.Step(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Type: pb.EntryConfChange}}})
	index := r.raftLog.lastIndex()
	pendingConf := r.pendingConf
	r.Step(pb.Message{From: 1, To: 1, Type: msgProp, Entries: []pb.Entry{{Type: pb.EntryConfChange}}})
	if g := r.raftLog.lastIndex(); g != index {
		t.Errorf("index = %d, want %d", g, index)
	}
	if r.pendingConf != pendingConf {
		t.Errorf("pendingConf = %v, want %v", r.pendingConf, pendingConf)
	}
}

// TestRecoverPendingConfig tests that new leader recovers its pendingConf flag
// based on uncommitted entries.
func TestRecoverPendingConfig(t *testing.T) {
	tests := []struct {
		entType  pb.EntryType
		wpending bool
	}{
		{pb.EntryNormal, false},
		{pb.EntryConfChange, true},
	}
	for i, tt := range tests {
		r := newRaft(1, []int64{1, 2}, 0, 0)
		r.appendEntry(pb.Entry{Type: tt.entType})
		r.becomeCandidate()
		r.becomeLeader()
		if r.pendingConf != tt.wpending {
			t.Errorf("#%d: pendingConf = %v, want %v", i, r.pendingConf, tt.wpending)
		}
	}
}

// TestRecoverDoublePendingConfig tests that new leader will panic if
// there exist two uncommitted config entries.
func TestRecoverDoublePendingConfig(t *testing.T) {
	func() {
		defer func() {
			if err := recover(); err == nil {
				t.Errorf("expect panic, but nothing happens")
			}
		}()
		r := newRaft(1, []int64{1, 2}, 0, 0)
		r.appendEntry(pb.Entry{Type: pb.EntryConfChange})
		r.appendEntry(pb.Entry{Type: pb.EntryConfChange})
		r.becomeCandidate()
		r.becomeLeader()
	}()
}

// TestAddNode tests that addNode could update pendingConf and peer list correctly.
func TestAddNode(t *testing.T) {
	r := newRaft(1, []int64{1}, 0, 0)
	r.pendingConf = true
	r.addNode(2)
	if r.pendingConf != false {
		t.Errorf("pendingConf = %v, want false", r.pendingConf)
	}
	nodes := r.nodes()
	sort.Sort(int64Slice(nodes))
	wnodes := []int64{1, 2}
	if !reflect.DeepEqual(nodes, wnodes) {
		t.Errorf("nodes = %v, want %v", nodes, wnodes)
	}
}

// TestRemoveNode tests that removeNode could update pendingConf and peer list correctly.
func TestRemoveNode(t *testing.T) {
	r := newRaft(1, []int64{1, 2}, 0, 0)
	r.pendingConf = true
	r.removeNode(2)
	if r.pendingConf != false {
		t.Errorf("pendingConf = %v, want false", r.pendingConf)
	}
	w := []int64{1}
	if g := r.nodes(); !reflect.DeepEqual(g, w) {
		t.Errorf("nodes = %v, want %v", g, w)
	}
}

func TestPromotable(t *testing.T) {
	id := int64(1)
	tests := []struct {
		peers []int64
		wp    bool
	}{
		{[]int64{1}, true},
		{[]int64{1, 2, 3}, true},
		{[]int64{}, false},
		{[]int64{2, 3}, false},
	}
	for i, tt := range tests {
		r := &raft{id: id, prs: make(map[int64]*progress)}
		for _, id := range tt.peers {
			r.prs[id] = &progress{}
		}
		if g := r.promotable(); g != tt.wp {
			t.Errorf("#%d: promotable = %v, want %v", i, g, tt.wp)
		}
	}
}

func ents(terms ...int64) *raft {
	ents := []pb.Entry{{}}
	for _, term := range terms {
		ents = append(ents, pb.Entry{Term: term})
	}

	sm := &raft{raftLog: &raftLog{ents: ents}}
	sm.reset(0)
	return sm
}

type network struct {
	peers   map[int64]Interface
	dropm   map[connem]float64
	ignorem map[int64]bool
}

// newNetwork initializes a network from peers.
// A nil node will be replaced with a new *stateMachine.
// A *stateMachine will get its k, id.
// When using stateMachine, the address list is always [0, n).
func newNetwork(peers ...Interface) *network {
	size := len(peers)
	peerAddrs := make([]int64, size)
	for i := 0; i < size; i++ {
		peerAddrs[i] = 1 + int64(i)
	}

	npeers := make(map[int64]Interface, size)

	for i, p := range peers {
		id := peerAddrs[i]
		switch v := p.(type) {
		case nil:
			sm := newRaft(id, peerAddrs, 0, 0)
			npeers[id] = sm
		case *raft:
			v.id = id
			v.prs = make(map[int64]*progress)
			for i := 0; i < size; i++ {
				v.prs[peerAddrs[i]] = &progress{}
			}
			v.reset(0)
			npeers[id] = v
		case *blackHole:
			npeers[id] = v
		default:
			panic(fmt.Sprintf("unexpected state machine type: %T", p))
		}
	}
	return &network{
		peers:   npeers,
		dropm:   make(map[connem]float64),
		ignorem: make(map[int64]bool),
	}
}

func (nw *network) send(msgs ...pb.Message) {
	for len(msgs) > 0 {
		m := msgs[0]
		p := nw.peers[m.To]
		p.Step(m)
		msgs = append(msgs[1:], nw.filter(p.ReadMessages())...)
	}
}

func (nw *network) drop(from, to int64, perc float64) {
	nw.dropm[connem{from, to}] = perc
}

func (nw *network) cut(one, other int64) {
	nw.drop(one, other, 1)
	nw.drop(other, one, 1)
}

func (nw *network) isolate(id int64) {
	for i := 0; i < len(nw.peers); i++ {
		nid := int64(i) + 1
		if nid != id {
			nw.drop(id, nid, 1.0)
			nw.drop(nid, id, 1.0)
		}
	}
}

func (nw *network) ignore(t int64) {
	nw.ignorem[t] = true
}

func (nw *network) recover() {
	nw.dropm = make(map[connem]float64)
	nw.ignorem = make(map[int64]bool)
}

func (nw *network) filter(msgs []pb.Message) []pb.Message {
	mm := make([]pb.Message, 0)
	for _, m := range msgs {
		if nw.ignorem[m.Type] {
			continue
		}
		switch m.Type {
		case msgHup:
			// hups never go over the network, so don't drop them but panic
			panic("unexpected msgHup")
		default:
			perc := nw.dropm[connem{m.From, m.To}]
			if n := rand.Float64(); n < perc {
				continue
			}
		}
		mm = append(mm, m)
	}
	return mm
}

type connem struct {
	from, to int64
}

type blackHole struct{}

func (blackHole) Step(pb.Message) error      { return nil }
func (blackHole) ReadMessages() []pb.Message { return nil }

var nopStepper = &blackHole{}
