package raft

import (
	"reflect"
	"testing"
)

const (
	defaultHeartbeat = 1
	defaultElection  = 5
)

func TestTickMsgHup(t *testing.T) {
	n := New(0, defaultHeartbeat, defaultElection)
	n.sm = newStateMachine(0, []int64{0, 1, 2})
	// simulate to patch the join log
	n.Step(Message{From: 1, Type: msgApp, Commit: 1, Entries: []Entry{Entry{}}})

	for i := 0; i < defaultElection*2; i++ {
		n.Tick()
	}

	called := false
	for _, m := range n.Msgs() {
		if m.Type == msgVote {
			called = true
		}
	}

	if !called {
		t.Errorf("called = %v, want true", called)
	}
}

func TestTickMsgBeat(t *testing.T) {
	k := 3
	n := dictate(New(0, defaultHeartbeat, defaultElection))
	n.Next()
	for i := 1; i < k; i++ {
		n.Add(int64(i), "", nil)
		for _, m := range n.Msgs() {
			if m.Type == msgApp {
				n.Step(Message{From: m.To, ClusterId: m.ClusterId, Type: msgAppResp, Index: m.Index + int64(len(m.Entries))})
			}
		}
		// ignore commit index update messages
		n.Msgs()
		n.Next()
	}

	for i := 0; i < defaultHeartbeat+1; i++ {
		n.Tick()
	}

	called := 0
	for _, m := range n.Msgs() {
		if m.Type == msgApp && len(m.Entries) == 0 {
			called++
		}
	}

	// msgBeat -> k-1 append
	w := k - 1
	if called != w {
		t.Errorf("called = %v, want %v", called, w)
	}
}

func TestResetElapse(t *testing.T) {
	tests := []struct {
		msg      Message
		welapsed tick
	}{
		{Message{From: 0, To: 1, Type: msgApp, Term: 2, Entries: []Entry{{Term: 1}}}, 0},
		{Message{From: 0, To: 1, Type: msgApp, Term: 1, Entries: []Entry{{Term: 1}}}, 1},
		{Message{From: 0, To: 1, Type: msgVote, Term: 2, Index: 1, LogTerm: 1}, 0},
		{Message{From: 0, To: 1, Type: msgVote, Term: 1}, 1},
	}

	for i, tt := range tests {
		n := New(0, defaultHeartbeat, defaultElection)
		n.sm = newStateMachine(0, []int64{0, 1, 2})
		n.sm.raftLog.append(0, Entry{Type: Normal, Term: 1})
		n.sm.term = 2
		n.sm.raftLog.committed = 1

		n.Tick()
		if n.elapsed != 1 {
			t.Errorf("%d: elpased = %d, want %d", i, n.elapsed, 1)
		}

		n.Step(tt.msg)
		if n.elapsed != tt.welapsed {
			t.Errorf("%d: elpased = %d, want %d", i, n.elapsed, tt.welapsed)
		}
	}
}

func TestStartCluster(t *testing.T) {
	n := dictate(New(0, defaultHeartbeat, defaultElection))
	n.Next()

	if len(n.sm.ins) != 1 {
		t.Errorf("k = %d, want 1", len(n.sm.ins))
	}
	if n.sm.id != 0 {
		t.Errorf("id = %d, want 0", n.sm.id)
	}
	if n.sm.state != stateLeader {
		t.Errorf("state = %s, want %s", n.sm.state, stateLeader)
	}
}

func TestAdd(t *testing.T) {
	n := dictate(New(0, defaultHeartbeat, defaultElection))
	n.Next()
	n.Add(1, "", nil)
	n.Next()

	if len(n.sm.ins) != 2 {
		t.Errorf("k = %d, want 2", len(n.sm.ins))
	}
	if n.sm.id != 0 {
		t.Errorf("id = %d, want 0", n.sm.id)
	}
}

func TestRemove(t *testing.T) {
	n := dictate(New(0, defaultHeartbeat, defaultElection))
	n.Next()
	n.Add(1, "", nil)
	n.Next()
	n.Remove(0)
	n.Step(Message{Type: msgAppResp, From: 1, ClusterId: n.ClusterId(), Term: 1, Index: 5})
	n.Next()

	if len(n.sm.ins) != 1 {
		t.Errorf("k = %d, want 1", len(n.sm.ins))
	}
	if n.sm.id != 0 {
		t.Errorf("id = %d, want 0", n.sm.id)
	}
}

func TestDenial(t *testing.T) {
	logents := []Entry{
		{Type: AddNode, Term: 1, Data: []byte(`{"NodeId":1}`)},
		{Type: AddNode, Term: 1, Data: []byte(`{"NodeId":2}`)},
		{Type: RemoveNode, Term: 1, Data: []byte(`{"NodeId":2}`)},
	}

	tests := []struct {
		ent     Entry
		wdenied map[int64]bool
	}{
		{
			Entry{Type: AddNode, Term: 1, Data: []byte(`{"NodeId":2}`)},
			map[int64]bool{1: false, 2: false},
		},
		{
			Entry{Type: RemoveNode, Term: 1, Data: []byte(`{"NodeId":1}`)},
			map[int64]bool{1: true, 2: true},
		},
		{
			Entry{Type: RemoveNode, Term: 1, Data: []byte(`{"NodeId":0}`)},
			map[int64]bool{1: false, 2: true},
		},
	}

	for i, tt := range tests {
		n := dictate(New(0, defaultHeartbeat, defaultElection))
		n.Next()
		n.Msgs()
		n.sm.raftLog.append(n.sm.raftLog.committed, append(logents, tt.ent)...)
		n.sm.raftLog.committed += int64(len(logents) + 1)
		n.Next()

		for id, denied := range tt.wdenied {
			n.Step(Message{From: id, To: 0, ClusterId: n.ClusterId(), Type: msgApp, Term: 1})
			w := []Message{}
			if denied {
				w = []Message{{From: 0, To: id, ClusterId: n.ClusterId(), Term: 1, Type: msgDenied}}
			}
			if g := n.Msgs(); !reflect.DeepEqual(g, w) {
				t.Errorf("#%d: msgs for %d = %+v, want %+v", i, id, g, w)
			}
		}
	}
}

func TestRecover(t *testing.T) {
	ents := []Entry{{Term: 1}, {Term: 2}, {Term: 3}}
	state := State{Term: 500, Vote: 1, Commit: 3}

	n := Recover(0, ents, state, defaultHeartbeat, defaultElection)
	if g := n.Next(); !reflect.DeepEqual(g, ents) {
		t.Errorf("ents = %+v, want %+v", g, ents)
	}
	if g := n.sm.term; g.Get() != state.Term {
		t.Errorf("term = %d, want %d", g, state.Term)
	}
	if g := n.sm.vote; g != state.Vote {
		t.Errorf("vote = %d, want %d", g, state.Vote)
	}
	if g := n.sm.raftLog.committed; g != state.Commit {
		t.Errorf("committed = %d, want %d", g, state.Commit)
	}
	if g := n.UnstableEnts(); g != nil {
		t.Errorf("unstableEnts = %+v, want nil", g)
	}
	if g := n.UnstableState(); !reflect.DeepEqual(g, state) {
		t.Errorf("unstableState = %+v, want %+v", g, state)
	}
	if g := n.Msgs(); len(g) != 0 {
		t.Errorf("#%d: len(msgs) = %d, want 0", len(g))
	}
}

func dictate(n *Node) *Node {
	n.Step(Message{From: n.Id(), Type: msgHup})
	n.InitCluster(0xBEEF)
	n.Add(n.Id(), "", nil)
	return n
}
