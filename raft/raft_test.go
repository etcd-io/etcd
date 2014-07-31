package raft

import (
	"bytes"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

func TestLeaderElection(t *testing.T) {
	tests := []struct {
		*network
		state stateType
	}{
		{newNetwork(nil, nil, nil), stateLeader},
		{newNetwork(nil, nil, nopStepper), stateLeader},
		{newNetwork(nil, nopStepper, nopStepper), stateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil), stateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil, nil), stateLeader},

		// three logs further along than 0
		{newNetwork(nil, ents(1), ents(2), ents(1, 3), nil), stateFollower},

		// logs converge
		{newNetwork(ents(1), nil, ents(2), ents(1), nil), stateLeader},
	}

	for i, tt := range tests {
		tt.send(Message{From: 0, To: 0, Type: msgHup})
		sm := tt.network.peers[0].(*stateMachine)
		if sm.state != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, sm.state, tt.state)
		}
		if g := sm.term.Get(); g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestLogReplication(t *testing.T) {
	tests := []struct {
		*network
		msgs       []Message
		wcommitted int64
	}{
		{
			newNetwork(nil, nil, nil),
			[]Message{
				{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("somedata")}}},
			},
			2,
		},
		{
			newNetwork(nil, nil, nil),
			[]Message{

				{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("somedata")}}},
				{From: 0, To: 1, Type: msgHup},
				{From: 0, To: 1, Type: msgProp, Entries: []Entry{{Data: []byte("somedata")}}},
			},
			4,
		},
	}

	for i, tt := range tests {
		tt.send(Message{From: 0, To: 0, Type: msgHup})

		for _, m := range tt.msgs {
			tt.send(m)
		}

		for j, x := range tt.network.peers {
			sm := x.(*stateMachine)

			if sm.raftLog.committed != tt.wcommitted {
				t.Errorf("#%d.%d: committed = %d, want %d", i, j, sm.raftLog.committed, tt.wcommitted)
			}

			ents := make([]Entry, 0)
			for _, e := range sm.nextEnts() {
				if e.Data != nil {
					ents = append(ents, e)
				}
			}
			props := make([]Message, 0)
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
	tt.send(Message{From: 0, To: 0, Type: msgHup})
	tt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("some data")}}})
	tt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("some data")}}})

	sm := tt.peers[0].(*stateMachine)
	if sm.raftLog.committed != 3 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 3)
	}
}

// TestCannotCommitWithoutNewTermEntry tests the entries cannot be committed
// when leader changes, no new proposal comes in and ChangeTerm proposal is
// filtered.
func TestCannotCommitWithoutNewTermEntry(t *testing.T) {
	tt := newNetwork(nil, nil, nil, nil, nil)
	tt.send(Message{From: 0, To: 0, Type: msgHup})

	// 0 cannot reach 2,3,4
	tt.cut(0, 2)
	tt.cut(0, 3)
	tt.cut(0, 4)

	tt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("some data")}}})
	tt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("some data")}}})

	sm := tt.peers[0].(*stateMachine)
	if sm.raftLog.committed != 1 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 1)
	}

	// network recovery
	tt.recover()
	// avoid committing ChangeTerm proposal
	tt.ignore(msgApp)

	// elect 1 as the new leader with term 2
	tt.send(Message{From: 1, To: 1, Type: msgHup})

	// no log entries from previous term should be committed
	sm = tt.peers[1].(*stateMachine)
	if sm.raftLog.committed != 1 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 1)
	}

	tt.recover()

	// send out a heartbeat
	// after append a ChangeTerm entry from the current term, all entries
	// should be committed
	tt.send(Message{From: 1, To: 1, Type: msgBeat})

	if sm.raftLog.committed != 4 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 4)
	}

	// still be able to append a entry
	tt.send(Message{From: 1, To: 1, Type: msgProp, Entries: []Entry{{Data: []byte("some data")}}})

	if sm.raftLog.committed != 5 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 5)
	}
}

// TestCommitWithoutNewTermEntry tests the entries could be committed
// when leader changes, no new proposal comes in.
func TestCommitWithoutNewTermEntry(t *testing.T) {
	tt := newNetwork(nil, nil, nil, nil, nil)
	tt.send(Message{From: 0, To: 0, Type: msgHup})

	// 0 cannot reach 2,3,4
	tt.cut(0, 2)
	tt.cut(0, 3)
	tt.cut(0, 4)

	tt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("some data")}}})
	tt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: []byte("some data")}}})

	sm := tt.peers[0].(*stateMachine)
	if sm.raftLog.committed != 1 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 1)
	}

	// network recovery
	tt.recover()

	// elect 1 as the new leader with term 2
	// after append a ChangeTerm entry from the current term, all entries
	// should be committed
	tt.send(Message{From: 1, To: 1, Type: msgHup})

	if sm.raftLog.committed != 4 {
		t.Errorf("committed = %d, want %d", sm.raftLog.committed, 4)
	}
}

func TestDuelingCandidates(t *testing.T) {
	a := newStateMachine(0, nil) // k, id are set later
	b := newStateMachine(0, nil)
	c := newStateMachine(0, nil)

	nt := newNetwork(a, b, c)
	nt.cut(0, 2)

	nt.send(Message{From: 0, To: 0, Type: msgHup})
	nt.send(Message{From: 2, To: 2, Type: msgHup})

	nt.recover()
	nt.send(Message{From: 2, To: 2, Type: msgHup})

	wlog := &raftLog{ents: []Entry{{}, Entry{Type: Normal, Data: nil, Term: 1, Index: 1}}, committed: 1}
	tests := []struct {
		sm      *stateMachine
		state   stateType
		term    int64
		raftLog *raftLog
	}{
		{a, stateFollower, 2, wlog},
		{b, stateFollower, 2, wlog},
		{c, stateFollower, 2, newLog()},
	}

	for i, tt := range tests {
		if g := tt.sm.state; g != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, g, tt.state)
		}
		if g := tt.sm.term.Get(); g != tt.term {
			t.Errorf("#%d: term = %d, want %d", i, g, tt.term)
		}
		base := ltoa(tt.raftLog)
		if sm, ok := nt.peers[int64(i)].(*stateMachine); ok {
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
	tt.isolate(0)

	tt.send(Message{From: 0, To: 0, Type: msgHup})
	tt.send(Message{From: 2, To: 2, Type: msgHup})

	// heal the partition
	tt.recover()

	data := []byte("force follower")
	// send a proposal to 2 to flush out a msgApp to 0
	tt.send(Message{From: 2, To: 2, Type: msgProp, Entries: []Entry{{Data: data}}})

	a := tt.peers[0].(*stateMachine)
	if g := a.state; g != stateFollower {
		t.Errorf("state = %s, want %s", g, stateFollower)
	}
	if g := a.term; g != 1 {
		t.Errorf("term = %d, want %d", g, 1)
	}
	wantLog := ltoa(&raftLog{ents: []Entry{{}, {Type: Normal, Data: nil, Term: 1, Index: 1}, {Term: 1, Index: 2, Data: data}}, committed: 2})
	for i, p := range tt.peers {
		if sm, ok := p.(*stateMachine); ok {
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
	tt.send(Message{From: 0, To: 0, Type: msgHup})

	sm := tt.peers[0].(*stateMachine)
	if sm.state != stateLeader {
		t.Errorf("state = %d, want %d", sm.state, stateLeader)
	}
}

func TestOldMessages(t *testing.T) {
	tt := newNetwork(nil, nil, nil)
	// make 0 leader @ term 3
	tt.send(Message{From: 0, To: 0, Type: msgHup})
	tt.send(Message{From: 1, To: 1, Type: msgHup})
	tt.send(Message{From: 0, To: 0, Type: msgHup})
	// pretend we're an old leader trying to make progress
	tt.send(Message{From: 0, To: 0, Type: msgApp, Term: 1, Entries: []Entry{{Term: 1}}})

	l := &raftLog{
		ents: []Entry{
			{}, {Type: Normal, Data: nil, Term: 1, Index: 1},
			{Type: Normal, Data: nil, Term: 2, Index: 2}, {Type: Normal, Data: nil, Term: 3, Index: 3},
		},
		committed: 3,
	}
	base := ltoa(l)
	for i, p := range tt.peers {
		if sm, ok := p.(*stateMachine); ok {
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
		send := func(m Message) {
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
		send(Message{From: 0, To: 0, Type: msgHup})
		send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Data: data}}})

		wantLog := newLog()
		if tt.success {
			wantLog = &raftLog{ents: []Entry{{}, {Type: Normal, Data: nil, Term: 1, Index: 1}, {Term: 1, Index: 2, Data: data}}, committed: 2}
		}
		base := ltoa(wantLog)
		for i, p := range tt.peers {
			if sm, ok := p.(*stateMachine); ok {
				l := ltoa(sm.raftLog)
				if g := diffu(base, l); g != "" {
					t.Errorf("#%d: diff:\n%s", i, g)
				}
			} else {
				t.Logf("#%d: empty log", i)
			}
		}
		sm := tt.network.peers[0].(*stateMachine)
		if g := sm.term.Get(); g != 1 {
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
		tt.send(Message{From: 0, To: 0, Type: msgHup})

		// propose via follower
		tt.send(Message{From: 1, To: 1, Type: msgProp, Entries: []Entry{{Data: []byte("somedata")}}})

		wantLog := &raftLog{ents: []Entry{{}, {Type: Normal, Data: nil, Term: 1, Index: 1}, {Term: 1, Data: data, Index: 2}}, committed: 2}
		base := ltoa(wantLog)
		for i, p := range tt.peers {
			if sm, ok := p.(*stateMachine); ok {
				l := ltoa(sm.raftLog)
				if g := diffu(base, l); g != "" {
					t.Errorf("#%d: diff:\n%s", i, g)
				}
			} else {
				t.Logf("#%d: empty log", i)
			}
		}
		sm := tt.peers[0].(*stateMachine)
		if g := sm.term.Get(); g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestCommit(t *testing.T) {
	tests := []struct {
		matches []int64
		logs    []Entry
		smTerm  int64
		w       int64
	}{
		// single
		{[]int64{1}, []Entry{{}, {Term: 1}}, 1, 1},
		{[]int64{1}, []Entry{{}, {Term: 1}}, 2, 0},
		{[]int64{2}, []Entry{{}, {Term: 1}, {Term: 2}}, 2, 2},
		{[]int64{1}, []Entry{{}, {Term: 2}}, 2, 1},

		// odd
		{[]int64{2, 1, 1}, []Entry{{}, {Term: 1}, {Term: 2}}, 1, 1},
		{[]int64{2, 1, 1}, []Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
		{[]int64{2, 1, 2}, []Entry{{}, {Term: 1}, {Term: 2}}, 2, 2},
		{[]int64{2, 1, 2}, []Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},

		// even
		{[]int64{2, 1, 1, 1}, []Entry{{}, {Term: 1}, {Term: 2}}, 1, 1},
		{[]int64{2, 1, 1, 1}, []Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
		{[]int64{2, 1, 1, 2}, []Entry{{}, {Term: 1}, {Term: 2}}, 1, 1},
		{[]int64{2, 1, 1, 2}, []Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
		{[]int64{2, 1, 2, 2}, []Entry{{}, {Term: 1}, {Term: 2}}, 2, 2},
		{[]int64{2, 1, 2, 2}, []Entry{{}, {Term: 1}, {Term: 1}}, 2, 0},
	}

	for i, tt := range tests {
		ins := make(map[int64]*index)
		for j := 0; j < len(tt.matches); j++ {
			ins[int64(j)] = &index{tt.matches[j], tt.matches[j] + 1}
		}
		sm := &stateMachine{raftLog: &raftLog{ents: tt.logs}, ins: ins, term: atomicInt(tt.smTerm)}
		sm.maybeCommit()
		if g := sm.raftLog.committed; g != tt.w {
			t.Errorf("#%d: committed = %d, want %d", i, g, tt.w)
		}
	}
}

// TestHandleMsgApp ensures:
// 1. Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm.
// 2. If an existing entry conflicts with a new one (same index but different terms),
//    delete the existing entry and all that follow it; append any new entries not already in the log.
// 3. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry).
func TestHandleMsgApp(t *testing.T) {
	tests := []struct {
		m       Message
		wIndex  int64
		wCommit int64
		wAccept bool
	}{
		// Ensure 1
		{Message{Type: msgApp, Term: 2, LogTerm: 3, Index: 2, Commit: 3}, 2, 0, false}, // previous log mismatch
		{Message{Type: msgApp, Term: 2, LogTerm: 3, Index: 3, Commit: 3}, 2, 0, false}, // previous log non-exist

		// Ensure 2
		{Message{Type: msgApp, Term: 2, LogTerm: 1, Index: 1, Commit: 1}, 2, 1, true},
		{Message{Type: msgApp, Term: 2, LogTerm: 0, Index: 0, Commit: 1, Entries: []Entry{{Term: 2}}}, 1, 1, true},
		{Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 3, Entries: []Entry{{Term: 2}, {Term: 2}}}, 4, 3, true},
		{Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 4, Entries: []Entry{{Term: 2}}}, 3, 3, true},
		{Message{Type: msgApp, Term: 2, LogTerm: 1, Index: 1, Commit: 4, Entries: []Entry{{Term: 2}}}, 2, 2, true},

		// Ensure 3
		{Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 2}, 2, 2, true},
		{Message{Type: msgApp, Term: 2, LogTerm: 2, Index: 2, Commit: 4}, 2, 2, true}, // commit upto min(commit, last)
	}

	for i, tt := range tests {
		sm := &stateMachine{
			state:   stateFollower,
			term:    2,
			raftLog: &raftLog{committed: 0, ents: []Entry{{}, {Term: 1}, {Term: 2}}},
		}

		sm.handleAppendEntries(tt.m)
		if sm.raftLog.lastIndex() != tt.wIndex {
			t.Errorf("#%d: lastIndex = %d, want %d", i, sm.raftLog.lastIndex(), tt.wIndex)
		}
		if sm.raftLog.committed != tt.wCommit {
			t.Errorf("#%d: committed = %d, want %d", i, sm.raftLog.committed, tt.wCommit)
		}
		m := sm.Msgs()
		if len(m) != 1 {
			t.Errorf("#%d: msg = nil, want 1")
		}
		gaccept := true
		if m[0].Index == -1 {
			gaccept = false
		}
		if gaccept != tt.wAccept {
			t.Errorf("#%d: accept = %v, want %v", gaccept, tt.wAccept)
		}
	}
}

func TestRecvMsgVote(t *testing.T) {
	tests := []struct {
		state   stateType
		i, term int64
		voteFor int64
		w       int64
	}{
		{stateFollower, 0, 0, none, -1},
		{stateFollower, 0, 1, none, -1},
		{stateFollower, 0, 2, none, -1},
		{stateFollower, 0, 3, none, 2},

		{stateFollower, 1, 0, none, -1},
		{stateFollower, 1, 1, none, -1},
		{stateFollower, 1, 2, none, -1},
		{stateFollower, 1, 3, none, 2},

		{stateFollower, 2, 0, none, -1},
		{stateFollower, 2, 1, none, -1},
		{stateFollower, 2, 2, none, 2},
		{stateFollower, 2, 3, none, 2},

		{stateFollower, 3, 0, none, -1},
		{stateFollower, 3, 1, none, -1},
		{stateFollower, 3, 2, none, 2},
		{stateFollower, 3, 3, none, 2},

		{stateFollower, 3, 2, 1, 2},
		{stateFollower, 3, 2, 0, -1},

		{stateLeader, 3, 3, 0, -1},
		{stateCandidate, 3, 3, 0, -1},
	}

	for i, tt := range tests {
		sm := &stateMachine{
			state:   tt.state,
			vote:    tt.voteFor,
			raftLog: &raftLog{ents: []Entry{{}, {Term: 2}, {Term: 2}}},
		}

		sm.Step(Message{Type: msgVote, From: 1, Index: tt.i, LogTerm: tt.term})

		msgs := sm.Msgs()
		if g := len(msgs); g != 1 {
			t.Errorf("#%d: len(msgs) = %d, want 1", i, g)
			continue
		}
		if g := msgs[0].Index; g != tt.w {
			t.Errorf("#%d, m.Index = %d, want %d", i, g, tt.w)
		}
	}
}

func TestStateTransition(t *testing.T) {
	tests := []struct {
		from   stateType
		to     stateType
		wallow bool
		wterm  int64
		wlead  int64
	}{
		{stateFollower, stateFollower, true, 1, none},
		{stateFollower, stateCandidate, true, 1, none},
		{stateFollower, stateLeader, false, -1, none},

		{stateCandidate, stateFollower, true, 0, none},
		{stateCandidate, stateCandidate, true, 1, none},
		{stateCandidate, stateLeader, true, 0, 0},

		{stateLeader, stateFollower, true, 1, none},
		{stateLeader, stateCandidate, false, 1, none},
		{stateLeader, stateLeader, true, 0, 0},
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

			sm := newStateMachine(0, []int64{0})
			sm.state = tt.from

			switch tt.to {
			case stateFollower:
				sm.becomeFollower(tt.wterm, tt.wlead)
			case stateCandidate:
				sm.becomeCandidate()
			case stateLeader:
				sm.becomeLeader()
			}

			if sm.term.Get() != tt.wterm {
				t.Errorf("%d: term = %d, want %d", i, sm.term.Get(), tt.wterm)
			}
			if sm.lead.Get() != tt.wlead {
				t.Errorf("%d: lead = %d, want %d", i, sm.lead, tt.wlead)
			}
		}()
	}
}

func TestConf(t *testing.T) {
	sm := newStateMachine(0, []int64{0})
	sm.becomeCandidate()
	sm.becomeLeader()

	sm.Step(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Type: AddNode}}})
	if sm.raftLog.lastIndex() != 2 {
		t.Errorf("lastindex = %d, want %d", sm.raftLog.lastIndex(), 1)
	}
	if !sm.pendingConf {
		t.Errorf("pendingConf = %v, want %v", sm.pendingConf, true)
	}
	if sm.raftLog.ents[2].Type != AddNode {
		t.Errorf("type = %d, want %d", sm.raftLog.ents[1].Type, AddNode)
	}

	// deny the second configuration change request if there is a pending one
	sm.Step(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{Type: AddNode}}})
	if sm.raftLog.lastIndex() != 2 {
		t.Errorf("lastindex = %d, want %d", sm.raftLog.lastIndex(), 1)
	}
}

// Ensures that the new leader sets the pendingConf flag correctly according to
// the uncommitted log entries
func TestConfChangeLeader(t *testing.T) {
	tests := []struct {
		et       int64
		wPending bool
	}{
		{Normal, false},
		{AddNode, true},
		{RemoveNode, true},
	}

	for i, tt := range tests {
		sm := newStateMachine(0, []int64{0})
		sm.raftLog = &raftLog{ents: []Entry{{}, {Type: tt.et}}}

		sm.becomeCandidate()
		sm.becomeLeader()

		if sm.pendingConf != tt.wPending {
			t.Errorf("#%d: pendingConf = %v, want %v", i, sm.pendingConf, tt.wPending)
		}
	}
}

func TestAllServerStepdown(t *testing.T) {
	tests := []struct {
		state stateType

		wstate stateType
		wterm  int64
		windex int64
	}{
		{stateFollower, stateFollower, 3, 1},
		{stateCandidate, stateFollower, 3, 1},
		{stateLeader, stateFollower, 3, 2},
	}

	tmsgTypes := [...]messageType{msgVote, msgApp}
	tterm := int64(3)

	for i, tt := range tests {
		sm := newStateMachine(0, []int64{0, 1, 2})
		switch tt.state {
		case stateFollower:
			sm.becomeFollower(1, 0)
		case stateCandidate:
			sm.becomeCandidate()
		case stateLeader:
			sm.becomeCandidate()
			sm.becomeLeader()
		}

		for j, msgType := range tmsgTypes {
			sm.Step(Message{From: 1, Type: msgType, Term: tterm, LogTerm: tterm})

			if sm.state != tt.wstate {
				t.Errorf("#%d.%d state = %v , want %v", i, j, sm.state, tt.wstate)
			}
			if sm.term.Get() != tt.wterm {
				t.Errorf("#%d.%d term = %v , want %v", i, j, sm.term.Get(), tt.wterm)
			}
			if int64(len(sm.raftLog.ents)) != tt.windex {
				t.Errorf("#%d.%d index = %v , want %v", i, j, len(sm.raftLog.ents), tt.windex)
			}
			wlead := int64(1)
			if msgType == msgVote {
				wlead = none
			}
			if sm.lead.Get() != wlead {
				t.Errorf("#%d, sm.lead = %d, want %d", i, sm.lead.Get(), none)
			}
		}
	}
}

func TestLeaderAppResp(t *testing.T) {
	tests := []struct {
		index      int64
		wmsgNum    int
		windex     int64
		wcommitted int64
	}{
		{-1, 1, 1, 0}, // bad resp; leader does not commit; reply with log entries
		{2, 2, 2, 2},  // good resp; leader commits; broadcast with commit index
	}

	for i, tt := range tests {
		// sm term is 1 after it becomes the leader.
		// thus the last log term must be 1 to be committed.
		sm := newStateMachine(0, []int64{0, 1, 2})
		sm.raftLog = &raftLog{ents: []Entry{{}, {Term: 0}, {Term: 1}}}
		sm.becomeCandidate()
		sm.becomeLeader()
		sm.Msgs()
		sm.Step(Message{From: 1, Type: msgAppResp, Index: tt.index, Term: sm.term.Get()})
		msgs := sm.Msgs()

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

// tests the output of the statemachine when receiving msgBeat
func TestRecvMsgBeat(t *testing.T) {
	tests := []struct {
		state stateType
		wMsg  int
	}{
		{stateLeader, 2},
		// candidate and follower should ignore msgBeat
		{stateCandidate, 0},
		{stateFollower, 0},
	}

	for i, tt := range tests {
		sm := newStateMachine(0, []int64{0, 1, 2})
		sm.raftLog = &raftLog{ents: []Entry{{}, {Term: 0}, {Term: 1}}}
		sm.term.Set(1)
		sm.state = tt.state
		sm.Step(Message{From: 0, To: 0, Type: msgBeat})

		msgs := sm.Msgs()
		if len(msgs) != tt.wMsg {
			t.Errorf("%d: len(msgs) = %d, want %d", i, len(msgs), tt.wMsg)
		}
		for _, m := range msgs {
			if m.Type != msgApp {
				t.Errorf("%d: msg.type = %v, want %v", m.Type, msgApp)
			}
		}
	}
}

func TestRestore(t *testing.T) {
	s := Snapshot{
		Index: defaultCompactThreshold + 1,
		Term:  defaultCompactThreshold + 1,
		Nodes: []int64{0, 1, 2},
	}

	sm := newStateMachine(0, []int64{0, 1})
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
	s := Snapshot{
		Index: defaultCompactThreshold + 1,
		Term:  defaultCompactThreshold + 1,
		Nodes: []int64{0, 1},
	}
	sm := newStateMachine(0, []int64{0})
	// restore the statemachin from a snapshot
	// so it has a compacted log and a snapshot
	sm.restore(s)

	sm.becomeCandidate()
	sm.becomeLeader()

	sm.Step(Message{From: 0, To: 0, Type: msgBeat})
	msgs := sm.Msgs()
	if len(msgs) != 1 {
		t.Errorf("len(msgs) = %d, want 1", len(msgs))
	}
	m := msgs[0]
	if m.Type != msgApp {
		t.Errorf("m.Type = %v, want %v", m.Type, msgApp)
	}

	// force set the next of node 1, so that
	// node 1 needs a snapshot
	sm.ins[1].next = sm.raftLog.offset

	sm.Step(Message{From: 1, To: 0, Type: msgAppResp, Index: -1})
	msgs = sm.Msgs()
	if len(msgs) != 1 {
		t.Errorf("len(msgs) = %d, want 1", len(msgs))
	}
	m = msgs[0]
	if m.Type != msgSnap {
		t.Errorf("m.Type = %v, want %v", m.Type, msgSnap)
	}
}

func TestRestoreFromSnapMsg(t *testing.T) {
	s := Snapshot{
		Index: defaultCompactThreshold + 1,
		Term:  defaultCompactThreshold + 1,
		Nodes: []int64{0, 1},
	}
	m := Message{Type: msgSnap, From: 0, Term: 1, Snapshot: s}

	sm := newStateMachine(1, []int64{0, 1})
	sm.Step(m)

	if !reflect.DeepEqual(sm.raftLog.snapshot, s) {
		t.Errorf("snapshot = %+v, want %+v", sm.raftLog.snapshot, s)
	}
}

func TestSlowNodeRestore(t *testing.T) {
	nt := newNetwork(nil, nil, nil)
	nt.send(Message{From: 0, To: 0, Type: msgHup})

	nt.isolate(2)
	for j := 0; j < defaultCompactThreshold+1; j++ {
		nt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{}}})
	}
	lead := nt.peers[0].(*stateMachine)
	lead.nextEnts()
	lead.compact(nil)

	nt.recover()
	nt.send(Message{From: 0, To: 0, Type: msgBeat})

	follower := nt.peers[2].(*stateMachine)
	if !reflect.DeepEqual(follower.raftLog.snapshot, lead.raftLog.snapshot) {
		t.Errorf("follower.snap = %+v, want %+v", follower.raftLog.snapshot, lead.raftLog.snapshot)
	}

	committed := follower.raftLog.lastIndex()
	nt.send(Message{From: 0, To: 0, Type: msgProp, Entries: []Entry{{}}})
	if follower.raftLog.committed != committed+1 {
		t.Errorf("follower.comitted = %d, want %d", follower.raftLog.committed, committed+1)
	}
}

func TestUnstableState(t *testing.T) {
	sm := newStateMachine(0, []int64{0})
	w := State{}

	sm.setVote(1)
	w.Vote = 1
	if !reflect.DeepEqual(sm.unstableState, w) {
		t.Errorf("unstableState = %v, want %v", sm.unstableState, w)
	}
	sm.clearState()

	sm.setTerm(1)
	w.Term = 1
	if !reflect.DeepEqual(sm.unstableState, w) {
		t.Errorf("unstableState = %v, want %v", sm.unstableState, w)
	}
	sm.clearState()

	sm.raftLog.committed = 1
	sm.addIns(1, 0, 0)
	w.Commit = 1
	if !reflect.DeepEqual(sm.unstableState, w) {
		t.Errorf("unstableState = %v, want %v", sm.unstableState, w)
	}
	sm.clearState()

	sm.raftLog.committed = 2
	sm.deleteIns(1)
	w.Commit = 2
	if !reflect.DeepEqual(sm.unstableState, w) {
		t.Errorf("unstableState = %v, want %v", sm.unstableState, w)
	}
	sm.clearState()
}

func ents(terms ...int64) *stateMachine {
	ents := []Entry{{}}
	for _, term := range terms {
		ents = append(ents, Entry{Term: term})
	}

	sm := &stateMachine{raftLog: &raftLog{ents: ents}}
	sm.reset(0)
	return sm
}

type network struct {
	peers   map[int64]Interface
	dropm   map[connem]float64
	ignorem map[messageType]bool
}

// newNetwork initializes a network from peers.
// A nil node will be replaced with a new *stateMachine.
// A *stateMachine will get its k, id.
// When using stateMachine, the address list is always [0, n).
func newNetwork(peers ...Interface) *network {
	size := len(peers)
	defaultPeerAddrs := make([]int64, size)
	for i := 0; i < size; i++ {
		defaultPeerAddrs[i] = int64(i)
	}

	npeers := make(map[int64]Interface, size)

	for id, p := range peers {
		nid := int64(id)
		switch v := p.(type) {
		case nil:
			sm := newStateMachine(nid, defaultPeerAddrs)
			npeers[nid] = sm
		case *stateMachine:
			v.id = nid
			v.ins = make(map[int64]*index)
			for i := 0; i < size; i++ {
				v.ins[int64(i)] = &index{}
			}
			v.reset(0)
			npeers[nid] = v
		case *Node:
			npeers[v.sm.id] = v
		default:
			npeers[nid] = v
		}
	}
	return &network{
		peers:   npeers,
		dropm:   make(map[connem]float64),
		ignorem: make(map[messageType]bool),
	}
}

func (nw *network) send(msgs ...Message) {
	for len(msgs) > 0 {
		m := msgs[0]
		p := nw.peers[m.To]
		p.Step(m)
		msgs = append(msgs[1:], nw.filter(p.Msgs())...)
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
		nid := int64(i)
		if nid != id {
			nw.drop(id, nid, 1.0)
			nw.drop(nid, id, 1.0)
		}
	}
}

func (nw *network) ignore(t messageType) {
	nw.ignorem[t] = true
}

func (nw *network) recover() {
	nw.dropm = make(map[connem]float64)
	nw.ignorem = make(map[messageType]bool)
}

func (nw *network) filter(msgs []Message) []Message {
	mm := make([]Message, 0)
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

func (blackHole) Step(Message) bool { return true }
func (blackHole) Msgs() []Message   { return nil }

var nopStepper = &blackHole{}
