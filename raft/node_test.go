package raft

import "testing"

const (
	defaultHeartbeat = 1
	defaultElection  = 5
)

func TestTickMsgHub(t *testing.T) {
	n := New(3, 0, defaultHeartbeat, defaultElection, nil)

	called := false
	n.next = stepperFunc(func(m Message) {
		if m.Type == msgVote {
			called = true
		}
	})

	for i := 0; i < defaultElection+1; i++ {
		n.Tick()
	}

	if !called {
		t.Errorf("called = %v, want true", called)
	}
}

func TestTickMsgBeat(t *testing.T) {
	k := 3
	n := New(k, 0, defaultHeartbeat, defaultElection, nil)

	called := 0
	n.next = stepperFunc(func(m Message) {
		if m.Type == msgApp {
			called++
		}
		if m.Type == msgVote {
			n.Step(Message{From: 1, Type: msgVoteResp, Index: 1, Term: 1})
		}
	})

	n.Step(Message{Type: msgHup}) // become leader please

	for i := 0; i < defaultHeartbeat+1; i++ {
		n.Tick()
	}

	// becomeLeader -> k-1 append
	// msgBeat -> k-1 append
	w := (k - 1) * 2
	if called != w {
		t.Errorf("called = %v, want %v", called, w)
	}
}
