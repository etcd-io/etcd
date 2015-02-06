package rafttest

import (
	"math/rand"
	"sync"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
)

// a network interface
type iface interface {
	send(m raftpb.Message)
	recv() chan raftpb.Message
	disconnect()
	connect()
}

// a network
type network interface {
	// drop message at given rate (1.0 drops all messages)
	drop(from, to uint64, rate float64)
	// delay message for (0, d] randomly at given rate (1.0 delay all messages)
	// do we need rate here?
	delay(from, to uint64, d time.Duration, rate float64)
	disconnect(id uint64)
	connect(id uint64)
	// heal heals the network
	heal()
}

type raftNetwork struct {
	mu           sync.Mutex
	disconnected map[uint64]bool
	dropmap      map[conn]float64
	recvQueues   map[uint64]chan raftpb.Message
}

type conn struct {
	from, to uint64
}

func newRaftNetwork(nodes ...uint64) *raftNetwork {
	pn := &raftNetwork{
		recvQueues:   make(map[uint64]chan raftpb.Message),
		dropmap:      make(map[conn]float64),
		disconnected: make(map[uint64]bool),
	}

	for _, n := range nodes {
		pn.recvQueues[n] = make(chan raftpb.Message, 1024)
	}
	return pn
}

func (rn *raftNetwork) nodeNetwork(id uint64) iface {
	return &nodeNetwork{id: id, raftNetwork: rn}
}

func (rn *raftNetwork) send(m raftpb.Message) {
	rn.mu.Lock()
	to := rn.recvQueues[m.To]
	if rn.disconnected[m.To] {
		to = nil
	}
	drop := rn.dropmap[conn{m.From, m.To}]
	rn.mu.Unlock()

	if to == nil {
		return
	}
	if drop != 0 && rand.Float64() < drop {
		return
	}

	to <- m
}

func (rn *raftNetwork) recvFrom(from uint64) chan raftpb.Message {
	rn.mu.Lock()
	fromc := rn.recvQueues[from]
	if rn.disconnected[from] {
		fromc = nil
	}
	rn.mu.Unlock()

	return fromc
}

func (rn *raftNetwork) drop(from, to uint64, rate float64) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.dropmap[conn{from, to}] = rate
}

func (rn *raftNetwork) delay(from, to uint64, d time.Duration, rate float64) {
	panic("unimplemented")
}

func (rn *raftNetwork) heal() {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.dropmap = make(map[conn]float64)
}

func (rn *raftNetwork) disconnect(id uint64) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.disconnected[id] = true
}

func (rn *raftNetwork) connect(id uint64) {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.disconnected[id] = false
}

type nodeNetwork struct {
	id uint64
	*raftNetwork
}

func (nt *nodeNetwork) connect() {
	nt.raftNetwork.connect(nt.id)
}

func (nt *nodeNetwork) disconnect() {
	nt.raftNetwork.disconnect(nt.id)
}

func (nt *nodeNetwork) send(m raftpb.Message) {
	nt.raftNetwork.send(m)
}

func (nt *nodeNetwork) recv() chan raftpb.Message {
	return nt.recvFrom(nt.id)
}
