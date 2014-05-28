package raft

type Interface interface {
	Step(m Message)
}

type tick int

type Node struct {
	// election timeout and heartbeat timeout in tick
	election  tick
	heartbeat tick

	// elapsed ticks after the last reset
	elapsed tick
	sm      *stateMachine

	next Interface
}

func New(k, addr int, heartbeat, election tick, next Interface) *Node {
	if election < heartbeat*3 {
		panic("election is least three times as heartbeat [election: %d, heartbeat: %d]")
	}

	n := &Node{
		sm:        newStateMachine(k, addr),
		next:      next,
		heartbeat: heartbeat,
		election:  election,
	}

	return n
}

// Propose asynchronously proposes data be applied to the underlying state machine.
func (n *Node) Propose(data []byte) {
	m := Message{Type: msgProp, Data: data}
	n.Step(m)
}

func (n *Node) Step(m Message) {
	n.sm.Step(m)
	ms := n.sm.Msgs()
	for _, m := range ms {
		// reset elapsed in two cases:
		// msgAppResp -> heard from the leader of the same term
		// msgVoteResp with grant -> heard from the candidate the node voted for
		switch m.Type {
		case msgAppResp:
			n.elapsed = 0
		case msgVoteResp:
			if m.Index >= 0 {
				n.elapsed = 0
			}
		}
		n.next.Step(m)
	}
}

// Next advances the commit index and returns any new
// commitable entries.
func (n *Node) Next() []Entry {
	return n.sm.nextEnts()
}

// Tick triggers the node to do a tick.
// If the current elapsed is greater or equal than the timeout,
// node will send corresponding message to the statemachine.
func (n *Node) Tick() {
	timeout, msgType := n.election, msgHup
	if n.sm.state == stateLeader {
		timeout, msgType = n.heartbeat, msgBeat
	}
	if n.elapsed >= timeout {
		n.Step(Message{Type: msgType})
		n.elapsed = 0
	} else {
		n.elapsed++
	}
}
