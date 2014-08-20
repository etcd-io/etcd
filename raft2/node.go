// Package raft implements raft.
package raft

import "code.google.com/p/go.net/context"

type stateResp struct {
	state State
	ents  []Entry
	msgs  []Message
}

type proposal struct {
	id   int64
	data []byte
}

type Node struct {
	ctx    context.Context
	propc  chan proposal
	recvc  chan Message
	statec chan stateResp
}

func Start(ctx context.Context, name string, election, heartbeat int) *Node {
	n := &Node{
		ctx:    ctx,
		propc:  make(chan proposal),
		recvc:  make(chan Message),
		statec: make(chan stateResp),
	}
	r := &raft{
		name:      name,
		election:  election,
		heartbeat: heartbeat,
	}
	go n.run(r)
	return n
}

func (n *Node) run(r *raft) {
	propc := n.propc

	for {
		if r.hasLeader() {
			propc = n.propc
		} else {
			// We cannot accept proposals because we don't know who
			// to send them to, so we'll apply back-pressure and
			// block senders.
			propc = nil
		}

		select {
		case p := <-propc:
			r.propose(p.id, p.data)
		case m := <-n.recvc:
			r.step(m)
		case n.statec <- stateResp{r.State, r.ents, r.msgs}:
			r.resetState()
		case <-n.ctx.Done():
			return
		}
	}
}

// Propose proposes data be appended to the log.
func (n *Node) Propose(id int64, data []byte) error {
	select {
	case n.propc <- proposal{id, data}:
		return nil
	case <-n.ctx.Done():
		return n.ctx.Err()
	}
}

// Step advances the state machine using m.
func (n *Node) Step(m Message) error {
	select {
	case n.recvc <- m:
		return nil
	case <-n.ctx.Done():
		return n.ctx.Err()
	}
}

// ReadMessages returns the current point-in-time state.
func (n *Node) ReadState() (State, []Entry, []Message, error) {
	select {
	case sr := <-n.statec:
		return sr.state, sr.ents, sr.msgs, nil
	case <-n.ctx.Done():
		return State{}, nil, nil, n.ctx.Err()
	}
}
