package raft

import (
	"testing"

	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

func BenchmarkOneNode(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := Start(1, []int64{1}, 0, 0)
	defer n.Stop()

	n.Campaign(ctx)
	for i := 0; i < b.N; i++ {
		<-n.Ready()
		n.Propose(ctx, []byte("foo"))
	}
	rd := <-n.Ready()
	if rd.HardState.Commit != int64(b.N+1) {
		b.Errorf("commit = %d, want %d", b.N+1)
	}
}
