package raft

import (
	"testing"
)

const (
	defaultHeartbeat = 1
	defaultElection  = 5
)

func TestTickMsgHub(t *testing.T) {
	n := New(0, []int{0, 1, 2}, defaultHeartbeat, defaultElection)

	for i := 0; i < defaultElection+1; i++ {
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
	n := New(0, []int{0, 1, 2}, defaultHeartbeat, defaultElection)

	n.Step(Message{Type: msgHup}) // become leader please
	for _, m := range n.Msgs() {
		if m.Type == msgVote {
			n.Step(Message{From: 1, Type: msgVoteResp, Index: 1, Term: 1})
		}
	}

	for i := 0; i < defaultHeartbeat+1; i++ {
		n.Tick()
	}

	called := 0
	for _, m := range n.Msgs() {
		if m.Type == msgApp {
			called++
		}
	}

	// becomeLeader -> k-1 append
	// msgBeat -> k-1 append
	w := (k - 1) * 2
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
		{Message{From: 0, To: 1, Type: msgVote, Term: 2}, 0},
		{Message{From: 0, To: 1, Type: msgVote, Term: 1}, 1},
	}

	for i, tt := range tests {
		n := New(0, []int{0, 1, 2}, defaultHeartbeat, defaultElection)
		n.sm.term = 2

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

func TestAdd(t *testing.T) {
	n := New(0, []int{0}, defaultHeartbeat, defaultElection)

	n.sm.becomeCandidate()
	n.sm.becomeLeader()
	n.Add(1)
	n.Next()

	if len(n.sm.ins) != 2 {
		t.Errorf("k = %d, want 2", len(n.sm.ins))
	}
}
