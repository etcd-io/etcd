package rafttest

import (
	"testing"
	"time"

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

func TestNetworkDelay(t *testing.T) {
	sent := 1000
	delay := time.Millisecond
	delayrate := 0.1
	nt := newRaftNetwork(1, 2)

	nt.delay(1, 2, delay, delayrate)
	var total time.Duration
	for i := 0; i < sent; i++ {
		s := time.Now()
		nt.send(raftpb.Message{From: 1, To: 2})
		total += time.Since(s)
	}

	w := time.Duration(float64(sent)*delayrate/2) * delay
	// there are pretty overhead in the send call, since it genarete random numbers.
	if total < w+10*delay {
		t.Errorf("total = %v, want > %v", total, w)
	}
}
