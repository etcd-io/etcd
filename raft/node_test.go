package raft

import (
	"reflect"
	"runtime"
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

// TestBlockProposal ensures that node will block proposal when it does not
// know who is the current leader; node will accept proposal when it knows
// who is the current leader.
func TestBlockProposal(t *testing.T) {
	n := newNode()
	r := newRaft(1, []int64{1}, 10, 1)
	go n.run(r)
	defer n.Stop()

	errc := make(chan error, 1)
	go func() {
		errc <- n.Propose(context.TODO(), []byte("somedata"))
	}()

	forceGosched()
	select {
	case err := <-errc:
		t.Errorf("err = %v, want blocking", err)
	default:
	}

	n.Campaign(context.TODO())
	forceGosched()
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("err = %v, want %v", err, nil)
		}
	default:
		t.Errorf("blocking proposal, want unblocking")
	}
}

func TestReadyContainUpdates(t *testing.T) {
	tests := []struct {
		rd       Ready
		wcontain bool
	}{
		{Ready{}, false},
		{Ready{State: raftpb.State{Vote: 1}}, true},
		{Ready{Entries: make([]raftpb.Entry, 1, 1)}, true},
		{Ready{CommittedEntries: make([]raftpb.Entry, 1, 1)}, true},
		{Ready{Messages: make([]raftpb.Message, 1, 1)}, true},
	}

	for i, tt := range tests {
		if tt.rd.containsUpdates() != tt.wcontain {
			t.Errorf("#%d: containUpdates = %v, want %v", i, tt.rd.containsUpdates(), tt.wcontain)
		}
	}
}

func TestNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wants := []Ready{
		{
			State:            raftpb.State{Term: 1, Commit: 1, LastIndex: 1},
			Entries:          []raftpb.Entry{{}, {Term: 1, Index: 1}},
			CommittedEntries: []raftpb.Entry{{Term: 1, Index: 1}},
		},
		{
			State:            raftpb.State{Term: 1, Commit: 2, LastIndex: 2},
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
	st := raftpb.State{Term: 1, Commit: 1, LastIndex: 2}

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

func TestIsStateEqual(t *testing.T) {
	tests := []struct {
		st raftpb.State
		we bool
	}{
		{emptyState, true},
		{raftpb.State{Vote: 1}, false},
		{raftpb.State{Commit: 1}, false},
		{raftpb.State{Term: 1}, false},
		{raftpb.State{LastIndex: 1}, false},
	}

	for i, tt := range tests {
		if isStateEqual(tt.st, emptyState) != tt.we {
			t.Errorf("#%d, equal = %v, want %v", i, isStateEqual(tt.st, emptyState), tt.we)
		}
	}
}

// WARNING: This is a hack.
// Remove this when we are able to block/check the status of the go-routines.
func forceGosched() {
	// possibility enough to sched upto 10 go routines.
	for i := 0; i < 10000; i++ {
		runtime.Gosched()
	}
}
