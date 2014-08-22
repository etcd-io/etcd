// Package raft implements raft.
package raft

import "code.google.com/p/go.net/context"

type stateResp struct {
	state       State
	ents, cents []Entry
	msgs        []Message
}

type Node struct {
	ctx    context.Context
	propc  chan []byte
	recvc  chan Message
	statec chan stateResp
	tickc  chan struct{}
}

func Start(ctx context.Context, name string, election, heartbeat int) *Node {
	n := &Node{
		ctx:    ctx,
		propc:  make(chan []byte),
		recvc:  make(chan Message),
		statec: make(chan stateResp),
		tickc:  make(chan struct{}),
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

		sr := stateResp{
			r.State,
			r.raftLog.unstableEnts(),
			r.raftLog.nextEnts(),
			r.msgs,
		}

		select {
		case p := <-propc:
			r.propose(p)
		case m := <-n.recvc:
			r.Step(m) // raft never returns an error
		case <-n.tickc:
			// r.tick()
		case n.statec <- sr:
			r.raftLog.resetNextEnts()
			r.raftLog.resetUnstable()
			r.msgs = nil
		case <-n.ctx.Done():
			return
		}
	}
}

func (n *Node) Tick() error {
	select {
	case n.tickc <- struct{}{}:
		return nil
	case <-n.ctx.Done():
		return n.ctx.Err()
	}
}

// Propose proposes data be appended to the log.
func (n *Node) Propose(data []byte) error {
	select {
	case n.propc <- data:
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

// ReadState returns the current point-in-time state.
func (n *Node) ReadState() (st State, ents, cents []Entry, msgs []Message, err error) {
	select {
	case sr := <-n.statec:
		return sr.state, sr.ents, sr.cents, sr.msgs, nil
	case <-n.ctx.Done():
		return State{}, nil, nil, nil, n.ctx.Err()
	}
}
