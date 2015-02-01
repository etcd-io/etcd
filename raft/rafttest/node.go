package rafttest

import (
	"log"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/raft"
	"github.com/coreos/etcd/raft/raftpb"
)

type node struct {
	raft.Node
	paused bool
	nt     network
	stopc  chan struct{}

	// stable
	storage *raft.MemoryStorage
	state   raftpb.HardState
}

func startNode(id uint64, peers []raft.Peer, nt network) *node {
	st := raft.NewMemoryStorage()
	rn := raft.StartNode(id, peers, 10, 1, st)
	n := &node{
		Node:    rn,
		storage: st,
		nt:      nt,
		stopc:   make(chan struct{}),
	}

	ticker := time.Tick(5 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ticker:
				n.Tick()
			case rd := <-n.Ready():
				if !raft.IsEmptyHardState(rd.HardState) {
					n.state = rd.HardState
				}
				n.storage.Append(rd.Entries)
				go func() {
					for _, m := range rd.Messages {
						nt.send(m)
					}
				}()
				n.Advance()
			case m := <-n.nt.recv():
				n.Step(context.TODO(), m)
			case <-n.stopc:
				log.Printf("raft.%d: stop", id)
				return
			}
		}
	}()
	return n
}

func (n *node) stop() { close(n.stopc) }

// restart restarts the node with the given delay.
// All in memory state of node is reset to initialized state.
// All stable MUST be unchanged.
func (n *node) restart(delay time.Duration) {
	panic("unimplemented")
}

// pause pauses the node.
// The paused node buffers the received messages and replies
// all of them when it resumes.
func (n *node) pause() {
	panic("unimplemented")
}

// resume resumes the paused node.
func (n *node) resume() {
	panic("unimplemented")
}

func (n *node) isPaused() bool {
	return n.paused
}
