// Package raft implements raft.
package raft

import (
	"errors"
	"log"

	pb "github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

var (
	EmptyState = pb.State{}
	ErrStopped = errors.New("raft: stopped")
)

// Ready encapsulates the entries and messages that are ready to be saved to
// stable storage, committed or sent to other peers.
type Ready struct {
	// The current state of a Node
	pb.State

	// Entries specifies entries to be saved to stable storage BEFORE
	// Messages are sent.
	Entries []pb.Entry

	// CommittedEntries specifies entries to be committed to a
	// store/state-machine. These have previously been committed to stable
	// store.
	CommittedEntries []pb.Entry

	// Messages specifies outbound messages to be sent AFTER Entries are
	// committed to stable storage.
	Messages []pb.Message
}

func isStateEqual(a, b pb.State) bool {
	return a.Term == b.Term && a.Vote == b.Vote && a.LastIndex == b.LastIndex
}

func (rd Ready) containsUpdates(prevSt pb.State) bool {
	return !isStateEqual(prevSt, rd.State) || len(rd.Entries) > 0 || len(rd.CommittedEntries) > 0 || len(rd.Messages) > 0
}

type Node struct {
	ctx    context.Context
	propc  chan pb.Message
	recvc  chan pb.Message
	readyc chan Ready
	tickc  chan struct{}
	done   chan struct{}
}

func Start(id int64, peers []int64, election, heartbeat int) Node {
	n := newNode()
	r := newRaft(id, peers, election, heartbeat)
	go n.run(r)
	return n
}

func Restart(id int64, peers []int64, election, heartbeat int, st pb.State, ents []pb.Entry) Node {
	n := newNode()
	r := newRaft(id, peers, election, heartbeat)
	r.loadState(st)
	r.loadEnts(ents)
	go n.run(r)
	return n
}

func newNode() Node {
	return Node{
		propc:  make(chan pb.Message),
		recvc:  make(chan pb.Message),
		readyc: make(chan Ready),
		tickc:  make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (n *Node) Stop() {
	close(n.done)
}

func (n *Node) run(r *raft) {
	propc := n.propc
	readyc := n.readyc

	var lead int64
	prevSt := r.State

	for {
		if lead != r.lead {
			log.Printf("raft: leader changed from %#x to %#x", lead, r.lead)
			lead = r.lead
			if r.hasLeader() {
				propc = n.propc
			} else {
				propc = nil
			}
		}

		rd := Ready{
			r.State,
			r.raftLog.unstableEnts(),
			r.raftLog.nextEnts(),
			r.msgs,
		}

		if rd.containsUpdates(prevSt) {
			readyc = n.readyc
		} else {
			readyc = nil
		}

		select {
		case m := <-propc:
			m.From = r.id
			r.Step(m)
		case m := <-n.recvc:
			r.Step(m) // raft never returns an error
		case <-n.tickc:
			r.tick()
		case readyc <- rd:
			r.raftLog.resetNextEnts()
			r.raftLog.resetUnstable()
			prevSt = rd.State
			r.msgs = nil
		case <-n.done:
			return
		}
	}
}

func (n *Node) Tick() error {
	select {
	case n.tickc <- struct{}{}:
		return nil
	case <-n.done:
		return n.ctx.Err()
	}
}

func (n *Node) Campaign(ctx context.Context) error {
	return n.Step(ctx, pb.Message{Type: msgHup})
}

// Propose proposes data be appended to the log.
func (n *Node) Propose(ctx context.Context, data []byte) error {
	return n.Step(ctx, pb.Message{Type: msgProp, Entries: []pb.Entry{{Data: data}}})
}

// Step advances the state machine using msgs. The ctx.Err() will be returned,
// if any.
func (n *Node) Step(ctx context.Context, m pb.Message) error {
	ch := n.recvc
	if m.Type == msgProp {
		ch = n.propc
	}

	select {
	case ch <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-n.done:
		return ErrStopped
	}
}

// ReadState returns the current point-in-time state.
func (n *Node) Ready() <-chan Ready {
	return n.readyc
}
