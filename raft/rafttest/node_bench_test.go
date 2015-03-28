package rafttest

import (
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/raft"
)

func BenchmarkProposal3Nodes(b *testing.B) {
	peers := []raft.Peer{{1, nil}, {2, nil}, {3, nil}}
	nt := newRaftNetwork(1, 2, 3)

	nodes := make([]*node, 0)

	for i := 1; i <= 3; i++ {
		n := startNode(uint64(i), peers, nt.nodeNetwork(uint64(i)))
		nodes = append(nodes, n)
	}
	// get ready and warm up
	time.Sleep(50 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nodes[0].Propose(context.TODO(), []byte("somedata"))
	}

	for _, n := range nodes {
		if n.state.Commit != uint64(b.N+4) {
			continue
		}
	}
	b.StopTimer()

	for _, n := range nodes {
		n.stop()
	}
}
