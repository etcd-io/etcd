package raft

import (
	"reflect"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

func TestNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wants := []Ready{
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

	n := Start(1, []int64{1}, 0, 0)
	n.Campaign(ctx)
	if g := <-n.Ready(); !reflect.DeepEqual(g, wants[0]) {
		t.Errorf("#%d: g = %+v,\n             w   %+v", 1, g, wants[0])
	}

	n.Propose(ctx, []byte("foo"))
	if g := <-n.Ready(); !reflect.DeepEqual(g, wants[1]) {
		t.Errorf("#%d: g = %+v,\n             w   %+v", 2, g, wants[1])
	}

	select {
	case rd := <-n.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	default:
	}
}
