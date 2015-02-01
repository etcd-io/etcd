package rafttest

import (
	"time"

	"github.com/coreos/etcd/raft/raftpb"
)

type network interface {
	send(m raftpb.Message)
	recv() chan raftpb.Message
	// drop message at given rate (1.0 drops all messages)
	drop(from, to uint64, rate float64)
	// delay message for (0, d] randomly at given rate (1.0 delay all messages)
	// do we need rate here?
	delay(from, to uint64, d time.Duration, rate float64)
}

type raftNetwork struct {
	recvQueues map[uint64]chan raftpb.Message
}

func newRaftNetwork(nodes ...uint64) *raftNetwork {
	pn := &raftNetwork{
		recvQueues: make(map[uint64]chan raftpb.Message, 0),
	}

	for _, n := range nodes {
		pn.recvQueues[n] = make(chan raftpb.Message, 1024)
	}
	return pn
}

func (rn *raftNetwork) nodeNetwork(id uint64) *nodeNetwork {
	return &nodeNetwork{id: id, raftNetwork: rn}
}

func (rn *raftNetwork) send(m raftpb.Message) {
	to := rn.recvQueues[m.To]
	if to == nil {
		panic("sent to nil")
	}
	to <- m
}

func (rn *raftNetwork) recvFrom(from uint64) chan raftpb.Message {
	fromc := rn.recvQueues[from]
	if fromc == nil {
		panic("recv from nil")
	}
	return fromc
}

func (rn *raftNetwork) drop(from, to uint64, rate float64) {
	panic("unimplemented")
}

func (rn *raftNetwork) delay(from, to uint64, d time.Duration, rate float64) {
	panic("unimplemented")
}

type nodeNetwork struct {
	id uint64
	*raftNetwork
}

func (nt *nodeNetwork) send(m raftpb.Message) {
	nt.raftNetwork.send(m)
}

func (nt *nodeNetwork) recv() chan raftpb.Message {
	return nt.recvFrom(nt.id)
}
