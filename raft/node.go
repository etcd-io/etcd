package raft

import (
	"errors"
	"log"

	pb "github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

var (
	emptyState = pb.HardState{}
	ErrStopped = errors.New("raft: stopped")
)

// SoftState provides state that is useful for logging and debugging.
// The state is volatile and does not need to be persisted to the WAL.
type SoftState struct {
	Lead      int64
	RaftState StateType
}

func (a *SoftState) equal(b *SoftState) bool {
	return a.Lead == b.Lead && a.RaftState == b.RaftState
}

// Ready encapsulates the entries and messages that are ready to read,
// be saved to stable storage, committed or sent to other peers.
// All fields in Ready are read-only.
type Ready struct {
	// The current volatile state of a Node.
	// SoftState will be nil if there is no update.
	// It is not required to consume or store SoftState.
	*SoftState

	// The current state of a Node to be saved to stable storage BEFORE
	// Messages are sent.
	// HardState will be equal to empty state if there is no update.
	pb.HardState

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

func isStateEqual(a, b pb.HardState) bool {
	return a.Term == b.Term && a.Vote == b.Vote && a.Commit == b.Commit
}

func IsEmptyState(st pb.HardState) bool {
	return isStateEqual(st, emptyState)
}

func (rd Ready) containsUpdates() bool {
	return rd.SoftState != nil || !IsEmptyState(rd.HardState) || len(rd.Entries) > 0 || len(rd.CommittedEntries) > 0 || len(rd.Messages) > 0
}

type Node struct {
	propc  chan pb.Message
	recvc  chan pb.Message
	readyc chan Ready
	tickc  chan struct{}
	done   chan struct{}
}

// Start returns a new Node given a unique raft id, a list of raft peers, and
// the election and heartbeat timeouts in units of ticks.
func Start(id int64, peers []int64, election, heartbeat int) Node {
	n := newNode()
	r := newRaft(id, peers, election, heartbeat)
	go n.run(r)
	return n
}

// Restart is identical to Start but takes an initial State and a slice of
// entries. Generally this is used when restarting from a stable storage
// log.
func Restart(id int64, peers []int64, election, heartbeat int, st pb.HardState, ents []pb.Entry) Node {
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
	var propc chan pb.Message
	var readyc chan Ready

	lead := None
	prevSoftSt := r.softState()
	prevHardSt := r.HardState

	for {
		rd := newReady(r, prevSoftSt, prevHardSt)
		if rd.containsUpdates() {
			readyc = n.readyc
		} else {
			readyc = nil
		}

		if rd.SoftState != nil && lead != rd.SoftState.Lead {
			log.Printf("raft: leader changed from %#x to %#x", lead, rd.SoftState.Lead)
			lead = rd.SoftState.Lead
			if r.hasLeader() {
				propc = n.propc
			} else {
				propc = nil
			}
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
			if rd.SoftState != nil {
				prevSoftSt = rd.SoftState
			}
			if !IsEmptyState(rd.HardState) {
				prevHardSt = rd.HardState
			}
			r.raftLog.resetNextEnts()
			r.raftLog.resetUnstable()
			r.msgs = nil
		case <-n.done:
			return
		}
	}
}

// Tick increments the internal logical clock for this Node. Election timeouts
// and heartbeat timeouts are in units of ticks.
func (n *Node) Tick() {
	select {
	case n.tickc <- struct{}{}:
	case <-n.done:
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

func newReady(r *raft, prevSoftSt *SoftState, prevHardSt pb.HardState) Ready {
	rd := Ready{
		Entries:          r.raftLog.unstableEnts(),
		CommittedEntries: r.raftLog.nextEnts(),
		Messages:         r.msgs,
	}
	if softSt := r.softState(); !softSt.equal(prevSoftSt) {
		rd.SoftState = softSt
	}
	if !isStateEqual(r.HardState, prevHardSt) {
		rd.HardState = r.HardState
	}
	return rd
}
