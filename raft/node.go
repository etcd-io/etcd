package raft

import "sync"

type Interface interface {
	Step(m Message)
}

type Node struct {
	lk sync.Mutex
	sm *stateMachine
}

func New(k, addr int, next Interface) *Node {
	n := &Node{
		sm: newStateMachine(k, addr, next),
	}
	return n
}

// Propose asynchronously proposes data be applied to the underlying state machine.
func (n *Node) Propose(data []byte) {
	m := Message{Type: msgHup, Data: data}
	n.Step(m)
}

func (n *Node) Step(m Message) {
	n.lk.Lock()
	defer n.lk.Unlock()
	n.sm.Step(m)
}
