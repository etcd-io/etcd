package raft

import (
	"testing"
)

const (
	defaultHeartbeat = 1
	defaultElection  = 5
)

func TestTickMsgHub(t *testing.T) {
	n := New(3, 0, defaultHeartbeat, defaultElection)

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
	n := New(k, 0, defaultHeartbeat, defaultElection)

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
