package raft

import (
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/etcd/third_party/code.google.com/p/go.net/context"
)

// TestNodeStep ensures that node.Step sends msgProp to propc chan
// and other kinds of messages to recvc chan.
func TestNodeStep(t *testing.T) {
	for i := range mtmap {
		n := &Node{
			propc: make(chan raftpb.Message, 1),
			recvc: make(chan raftpb.Message, 1),
		}
		n.Step(context.TODO(), raftpb.Message{Type: int64(i)})
		// Proposal goes to proc chan. Others go to recvc chan.
		if int64(i) == msgProp {
			select {
			case <-n.propc:
			default:
				t.Errorf("%d: cannot receive %s on propc chan", i, mtmap[i])
			}
		} else {
			select {
			case <-n.recvc:
			default:
				t.Errorf("%d: cannot receive %s on recvc chan", i, mtmap[i])
			}
		}
	}
}

// Cancel and Stop should unblock Step()
func TestNodeStepUnblock(t *testing.T) {
	// a node without buffer to block step
	n := &Node{
		propc: make(chan raftpb.Message),
		done:  make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopFunc := func() { close(n.done) }

	tests := []struct {
		unblock func()
		werr    error
	}{
		{stopFunc, ErrStopped},
		{cancel, context.Canceled},
	}

	for i, tt := range tests {
		errc := make(chan error, 1)
		go func() {
			err := n.Step(ctx, raftpb.Message{Type: msgProp})
			errc <- err
		}()
		tt.unblock()
		select {
		case err := <-errc:
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", err, tt.werr)
			}
			//clean up side-effect
			if ctx.Err() != nil {
				ctx = context.TODO()
			}
			select {
			case <-n.done:
				n.done = make(chan struct{})
			default:
			}
		case <-time.After(time.Millisecond * 100):
			t.Errorf("#%d: failed to unblock step", i)
		}
	}
}

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

func TestNodeRestart(t *testing.T) {
	entries := []raftpb.Entry{
		{Term: 1, Index: 1},
		{Term: 1, Index: 2, Data: []byte("foo")},
	}
	st := raftpb.State{Term: 1, Vote: -1, Commit: 1, LastIndex: 2}

	want := Ready{
		State: emptyState,
		// commit upto index commit index in st
		CommittedEntries: entries[:st.Commit],
	}

	n := Restart(1, []int64{1}, 0, 0, st, entries)
	if g := <-n.Ready(); !reflect.DeepEqual(g, want) {
		t.Errorf("g = %+v,\n             w   %+v", g, want)
	}

	select {
	case rd := <-n.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	default:
	}
}
