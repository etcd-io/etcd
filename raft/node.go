package raft

import (
	"encoding/json"
	golog "log"
)

type Interface interface {
	Step(m Message) bool
	Msgs() []Message
}

type tick int

type config struct {
	NodeId  int
	Address string
}

type Node struct {
	sm *stateMachine

	elapsed   tick
	election  tick
	heartbeat tick
}

func New(id int, heartbeat, election tick) *Node {
	if election < heartbeat*3 {
		panic("election is least three times as heartbeat [election: %d, heartbeat: %d]")
	}

	n := &Node{
		heartbeat: heartbeat,
		election:  election,
		sm:        newStateMachine(id, []int{id}),
	}

	return n
}

func Dictate(n *Node) *Node {
	n.Step(Message{Type: msgHup})
	n.Add(n.Id())
	return n
}

func (n *Node) Id() int { return n.sm.id }

func (n *Node) HasLeader() bool { return n.sm.lead != none }

// Propose asynchronously proposes data be applied to the underlying state machine.
func (n *Node) Propose(data []byte) { n.propose(normal, data) }

func (n *Node) propose(t int, data []byte) {
	n.Step(Message{Type: msgProp, Entries: []Entry{{Type: t, Data: data}}})
}

func (n *Node) Add(id int) { n.updateConf(configAdd, &config{NodeId: id}) }

func (n *Node) Remove(id int) { n.updateConf(configRemove, &config{NodeId: id}) }

func (n *Node) Msgs() []Message { return n.sm.Msgs() }

func (n *Node) Step(m Message) bool {
	l := len(n.sm.msgs)
	if !n.sm.Step(m) {
		return false
	}
	for _, m := range n.sm.msgs[l:] {
		switch m.Type {
		case msgAppResp:
			// We just heard from the leader of the same term.
			n.elapsed = 0
		case msgVoteResp:
			// We just heard from the candidate the node voted for.
			if m.Index >= 0 {
				n.elapsed = 0
			}
		}
	}
	return true
}

// Next returns all the appliable entries
func (n *Node) Next() []Entry {
	ents := n.sm.nextEnts()
	for i := range ents {
		switch ents[i].Type {
		case normal:
		case configAdd:
			c := new(config)
			if err := json.Unmarshal(ents[i].Data, c); err != nil {
				golog.Println(err)
				continue
			}
			n.sm.addNode(c.NodeId)
		case configRemove:
			c := new(config)
			if err := json.Unmarshal(ents[i].Data, c); err != nil {
				golog.Println(err)
				continue
			}
			n.sm.removeNode(c.NodeId)
		default:
			panic("unexpected entry type")
		}
	}
	return ents
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

func (n *Node) updateConf(t int, c *config) {
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	n.propose(t, data)
}
