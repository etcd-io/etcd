package rafttest

import (
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
)

func TestNetworkDrop(t *testing.T) {
	// drop around 10% messages
	sent := 1000
	droprate := 0.1
	nt := newRaftNetwork(1, 2)
	nt.drop(1, 2, droprate)
	for i := 0; i < sent; i++ {
		nt.send(raftpb.Message{From: 1, To: 2})
	}

	c := nt.recvFrom(2)

	received := 0
	done := false
	for !done {
		select {
		case <-c:
			received++
		default:
			done = true
		}
	}

	drop := sent - received
	if drop > int((droprate+0.1)*float64(sent)) || drop < int((droprate-0.1)*float64(sent)) {
		t.Errorf("drop = %d, want around %d", drop, droprate*float64(sent))
	}
}
