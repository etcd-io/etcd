package raft

import (
	"encoding/json"
)

type Interface interface {
	Step(m Message)
	Msgs() []Message
}

type tick int

type ConfigCmd struct {
	Type string
	Addr int
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
	}

	return n
}

// Propose asynchronously proposes data be applied to the underlying state machine.
func (n *Node) Propose(data []byte) {
	m := Message{Type: msgProp, Entries: []Entry{{Data: data}}}
	n.Step(m)
}

func (n *Node) StartCluster() {
	if n.sm != nil {
		panic("node is started")
	}
	n.sm = newStateMachine(n.addr, []int{n.addr})
	n.Step(Message{Type: msgHup})
	n.Step(n.newConfMessage(&ConfigCmd{Type: "add", Addr: n.addr}))
	n.Next()
}

func (n *Node) Start() {
	if n.sm != nil {
		panic("node is started")
	}
	n.sm = newStateMachine(n.addr, nil)
}

func (n *Node) Add(addr int) {
	n.Step(n.newConfMessage(&ConfigCmd{Type: "add", Addr: addr}))
}

func (n *Node) Remove(addr int) {
	n.Step(n.newConfMessage(&ConfigCmd{Type: "remove", Addr: addr}))
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
		case config:
			c := new(ConfigCmd)
			err := json.Unmarshal(ents[i].Data, c)
			if err != nil {
				// warning
				continue
			}
			n.updateConf(c)
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

func (n *Node) newConfMessage(c *ConfigCmd) Message {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return Message{Type: msgProp, To: n.addr, Entries: []Entry{Entry{Type: config, Data: data}}}
}

func (n *Node) updateConf(c *ConfigCmd) {
	switch c.Type {
	case "add":
		n.sm.Add(c.Addr)
	case "remove":
		n.sm.Remove(c.Addr)
	default:
		// warn
	}
}
