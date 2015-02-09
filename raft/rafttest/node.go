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
	id     uint64
	paused bool
	iface  iface
	stopc  chan struct{}

	// stable
	storage *raft.MemoryStorage
	state   raftpb.HardState
}

func startNode(id uint64, peers []raft.Peer, iface iface) *node {
	st := raft.NewMemoryStorage()
	rn := raft.StartNode(id, peers, 10, 1, st)
	n := &node{
		Node:    rn,
		id:      id,
		storage: st,
		iface:   iface,
	}
	n.start()
	return n
}

func (n *node) start() {
	n.stopc = make(chan struct{})
	ticker := time.Tick(5 * time.Millisecond)

	go func() {
		for {
			select {
			case <-ticker:
				n.Tick()
			case rd := <-n.Ready():
				if !raft.IsEmptyHardState(rd.HardState) {
					n.state = rd.HardState
					n.storage.SetHardState(n.state)
				}
				n.storage.Append(rd.Entries)
				// TODO: make send async, more like real world...
				for _, m := range rd.Messages {
					n.iface.send(m)
				}
				n.Advance()
			case m := <-n.iface.recv():
				n.Step(context.TODO(), m)
			case <-n.stopc:
				n.Stop()
				log.Printf("raft.%d: stop", n.id)
				n.Node = nil
				close(n.stopc)
				return
			}
		}
	}()
}

// stop stops the node. stop a stopped node might panic.
// All in memory state of node is discarded.
// All stable MUST be unchanged.
func (n *node) stop() {
	n.iface.disconnect()
	n.stopc <- struct{}{}
	// wait for the shutdown
	<-n.stopc
}

// restart restarts the node. restart a started node
// blocks and might affect the future stop operation.
func (n *node) restart() {
	// wait for the shutdown
	<-n.stopc
	n.Node = raft.RestartNode(n.id, 10, 1, n.storage, 0)
	n.start()
	n.iface.connect()
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
