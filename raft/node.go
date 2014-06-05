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
	Id   int
}

type Node struct {
	// election timeout and heartbeat timeout in tick
	election  tick
	heartbeat tick

	// elapsed ticks after the last reset
	elapsed tick
	sm      *stateMachine
}

func New(addr int, peers []int, heartbeat, election tick) *Node {
	if election < heartbeat*3 {
		panic("election is least three times as heartbeat [election: %d, heartbeat: %d]")
	}

	n := &Node{
		sm:        newStateMachine(addr, peers),
		heartbeat: heartbeat,
		election:  election,
	}

	return n
}

// Propose asynchronously proposes data be applied to the underlying state machine.
func (n *Node) Propose(data []byte) {
	m := Message{Type: msgProp, Entries: []Entry{{Data: data}}}
	n.Step(m)
}

func (n *Node) Add(id int) {
	c := &ConfigCmd{
		Type: "add",
		Id:   id,
	}

	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	m := Message{Type: msgProp, Entries: []Entry{Entry{Type: config, Data: data}}}
	n.Step(m)
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
func (n *Node) Next() {
	ents := n.sm.nextEnts()
	for i := range ents {
		switch ents[i].Type {
		case normal:
			// dispatch to the application state machine
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

func (n *Node) updateConf(c *ConfigCmd) {
	switch c.Type {
	case "add":
		n.sm.Add(c.Id)
	default:
		// warn
	}
}
