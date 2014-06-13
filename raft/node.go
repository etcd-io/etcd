package raft

import (
	"encoding/json"
	golog "log"
)

type Interface interface {
	Step(m Message)
	Msgs() []Message
}

type tick int

type Config struct {
	NodeId    int
	ClusterId int
	Address   string
}

type Node struct {
	// election timeout and heartbeat timeout in tick
	election  tick
	heartbeat tick

	// elapsed ticks after the last reset
	elapsed tick
	sm      *stateMachine

	addr int
}

func New(addr int, heartbeat, election tick) *Node {
	if election < heartbeat*3 {
		panic("election is least three times as heartbeat [election: %d, heartbeat: %d]")
	}

	n := &Node{
		heartbeat: heartbeat,
		election:  election,
		addr:      addr,
		sm:        newStateMachine(addr, []int{addr}),
	}

	return n
}

func Dictate(n *Node) *Node {
	n.Step(Message{Type: msgHup})
	n.Step(n.newConfMessage(configAdd, &Config{NodeId: n.addr}))
	return n
}

// Propose asynchronously proposes data be applied to the underlying state machine.
func (n *Node) Propose(data []byte) {
	m := Message{Type: msgProp, Entries: []Entry{{Data: data}}}
	n.Step(m)
}

func (n *Node) Add(addr int) {
	n.Step(n.newConfMessage(configAdd, &Config{NodeId: addr}))
}

func (n *Node) Remove(addr int) {
	n.Step(n.newConfMessage(configRemove, &Config{NodeId: addr}))
}

func (n *Node) Msgs() []Message {
	return n.sm.Msgs()
}

func (n *Node) Step(m Message) {
	l := len(n.sm.msgs)
	n.sm.Step(m)
	for _, m := range n.sm.msgs[l:] {
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
	}
}

// Next applies all available committed commands.
func (n *Node) Next() []Entry {
	ents := n.sm.nextEnts()
	nents := make([]Entry, 0)
	for i := range ents {
		switch ents[i].Type {
		case normal:
			// dispatch to the application state machine
			nents = append(nents, ents[i])
		case configAdd:
			c := new(Config)
			if err := json.Unmarshal(ents[i].Data, c); err != nil {
				golog.Println(err)
				continue
			}
			n.sm.Add(c.NodeId)
		case configRemove:
			c := new(Config)
			if err := json.Unmarshal(ents[i].Data, c); err != nil {
				golog.Println(err)
				continue
			}
			n.sm.Remove(c.NodeId)
		default:
			panic("unexpected entry type")
		}
	}
	return nents
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

func (n *Node) newConfMessage(t int, c *Config) Message {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return Message{Type: msgProp, To: n.addr, Entries: []Entry{Entry{Type: t, Data: data}}}
}
