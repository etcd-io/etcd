package rafttest

import (
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/raft"
)

func TestBasicProgress(t *testing.T) {
	peers := []raft.Peer{{1, nil}, {2, nil}, {3, nil}, {4, nil}, {5, nil}}
	nt := newRaftNetwork(1, 2, 3, 4, 5)

	nodes := make([]*node, 0)

	for i := 1; i <= 5; i++ {
		n := startNode(uint64(i), peers, nt.nodeNetwork(uint64(i)))
		nodes = append(nodes, n)
	}

	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 1000; i++ {
		nodes[0].Propose(context.TODO(), []byte("somedata"))
	}

	time.Sleep(100 * time.Millisecond)
	for _, n := range nodes {
		n.stop()
		if n.state.Commit < 1000 {
			t.Errorf("commit = %d, want > 1000", n.state.Commit)
		}
	}
}
