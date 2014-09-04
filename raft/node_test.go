package raft

import (
	"reflect"
	"testing"
	"github.com/coreos/etcd/raft/raftpb"

	"code.google.com/p/go.net/context"
)

func TestNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n := Start(1, []int64{1}, 0, 0)
	ch := make(chan Ready)
	go func() {
		for {
			ch <- <-n.Ready()
		}
	}()
	n.Campaign(ctx)
	n.Propose(ctx, []byte("foo"))

	want := []Ready{
		{
			State:            raftpb.State{Term: 1, Vote: -1, Commit: 1, LastIndex: 1},
			Entries:          []raftpb.Entry{{Term: 1, Index: 1}},
			CommittedEntries: []raftpb.Entry{{Term: 1, Index: 1}},
		},
		{
			State:            raftpb.State{Term: 1, Vote: -1, Commit: 2, LastIndex: 2},
			Entries:          []raftpb.Entry{{Term: 1, Index: 2, Data: []byte("foo")}},
			CommittedEntries: []raftpb.Entry{{Term: 1, Index: 2, Data: []byte("foo")}},
		},
	}

	for i, w := range want {
		if g := <-ch; !reflect.DeepEqual(g, w) {
			t.Errorf("#%d: g = %+v,\n             w   %+v", i, g, w)
		}
	}

	select {
	case rd := <-ch:
		t.Errorf("unexpected Ready: %+v", rd)
	default:
	}

}
