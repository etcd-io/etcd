// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/raft/raftpb"
)

// TestMultiNodeStep ensures that multiNode.Step sends MsgProp to propc
// chan and other kinds of messages to recvc chan.
func TestMultiNodeStep(t *testing.T) {
	for i, msgn := range raftpb.MessageType_name {
		mn := &multiNode{
			propc: make(chan multiMessage, 1),
			recvc: make(chan multiMessage, 1),
		}
		msgt := raftpb.MessageType(i)
		mn.Step(context.TODO(), 1, raftpb.Message{Type: msgt})
		// Proposal goes to proc chan. Others go to recvc chan.
		if msgt == raftpb.MsgProp {
			select {
			case <-mn.propc:
			default:
				t.Errorf("%d: cannot receive %s on propc chan", msgt, msgn)
			}
		} else {
			if msgt == raftpb.MsgBeat || msgt == raftpb.MsgHup || msgt == raftpb.MsgUnreachable || msgt == raftpb.MsgSnapStatus {
				select {
				case <-mn.recvc:
					t.Errorf("%d: step should ignore %s", msgt, msgn)
				default:
				}
			} else {
				select {
				case <-mn.recvc:
				default:
					t.Errorf("%d: cannot receive %s on recvc chan", msgt, msgn)
				}
			}
		}
	}
}

// Cancel and Stop should unblock Step()
func TestMultiNodeStepUnblock(t *testing.T) {
	// a node without buffer to block step
	mn := &multiNode{
		propc: make(chan multiMessage),
		done:  make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopFunc := func() { close(mn.done) }

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
			err := mn.Step(ctx, 1, raftpb.Message{Type: raftpb.MsgProp})
			errc <- err
		}()
		tt.unblock()
		select {
		case err := <-errc:
			if err != tt.werr {
				t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
			}
			//clean up side-effect
			if ctx.Err() != nil {
				ctx = context.TODO()
			}
			select {
			case <-mn.done:
				mn.done = make(chan struct{})
			default:
			}
		case <-time.After(time.Millisecond * 100):
			t.Errorf("#%d: failed to unblock step", i)
		}
	}
}

// TestMultiNodePropose ensures that node.Propose sends the given proposal to the underlying raft.
func TestMultiNodePropose(t *testing.T) {
	mn := newMultiNode(1, 10, 1)
	go mn.run()
	s := NewMemoryStorage()
	mn.CreateGroup(1, []Peer{{ID: 1}}, s)
	mn.Campaign(context.TODO(), 1)
	proposed := false
	for {
		rds := <-mn.Ready()
		rd := rds[1]
		s.Append(rd.Entries)
		// Once we are the leader, propose a command.
		if !proposed && rd.SoftState.Lead == mn.id {
			mn.Propose(context.TODO(), 1, []byte("somedata"))
			proposed = true
		}
		mn.Advance(rds)

		// Exit when we have three entries: one ConfChange, one no-op for the election,
		// and our proposed command.
		lastIndex, err := s.LastIndex()
		if err != nil {
			t.Fatal(err)
		}
		if lastIndex >= 3 {
			break
		}
	}
	mn.Stop()

	lastIndex, err := s.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := s.Entries(lastIndex, lastIndex+1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want %d", len(entries), 1)
	}
	if !bytes.Equal(entries[0].Data, []byte("somedata")) {
		t.Errorf("entries[0].Data = %v, want %v", entries[0].Data, []byte("somedata"))
	}
}

// TestMultiNodeProposeConfig ensures that multiNode.ProposeConfChange
// sends the given configuration proposal to the underlying raft.
func TestMultiNodeProposeConfig(t *testing.T) {
	mn := newMultiNode(1, 10, 1)
	go mn.run()
	s := NewMemoryStorage()
	mn.CreateGroup(1, []Peer{{ID: 1}}, s)
	mn.Campaign(context.TODO(), 1)
	proposed := false
	var lastIndex uint64
	var ccdata []byte
	for {
		rds := <-mn.Ready()
		rd := rds[1]
		s.Append(rd.Entries)
		// change the step function to appendStep until this raft becomes leader
		if !proposed && rd.SoftState.Lead == mn.id {
			cc := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1}
			var err error
			ccdata, err = cc.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			mn.ProposeConfChange(context.TODO(), 1, cc)
			proposed = true
		}
		mn.Advance(rds)

		var err error
		lastIndex, err = s.LastIndex()
		if err != nil {
			t.Fatal(err)
		}
		if lastIndex >= 3 {
			break
		}
	}
	mn.Stop()

	entries, err := s.Entries(lastIndex, lastIndex+1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want %d", len(entries), 1)
	}
	if entries[0].Type != raftpb.EntryConfChange {
		t.Fatalf("type = %v, want %v", entries[0].Type, raftpb.EntryConfChange)
	}
	if !bytes.Equal(entries[0].Data, ccdata) {
		t.Errorf("data = %v, want %v", entries[0].Data, ccdata)
	}
}

// TestBlockProposal from node_test.go has no equivalent in multiNode
// because we cannot block proposals based on individual group leader status.

// TestNodeTick from node_test.go has no equivalent in multiNode because
// it reaches into the raft object which is not exposed.

// TestMultiNodeStop ensures that multiNode.Stop() blocks until the node has stopped
// processing, and that it is idempotent
func TestMultiNodeStop(t *testing.T) {
	mn := newMultiNode(1, 10, 1)
	donec := make(chan struct{})

	go func() {
		mn.run()
		close(donec)
	}()

	mn.Tick()
	mn.Stop()

	select {
	case <-donec:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for node to stop!")
	}

	// Further ticks should have no effect, the node is stopped.
	// There is no way to verify this in multinode but at least we can test
	// it doesn't block or panic.
	mn.Tick()
	// Subsequent Stops should have no effect.
	mn.Stop()
}

// TestMultiNodeStart ensures that a node can be started correctly. The node should
// start with correct configuration change entries, and can accept and commit
// proposals.
func TestMultiNodeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cc := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1}
	ccdata, err := cc.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	wants := []Ready{
		{
			SoftState: &SoftState{Lead: 1, RaftState: StateLeader},
			HardState: raftpb.HardState{Term: 2, Commit: 2, Vote: 1},
			Entries: []raftpb.Entry{
				{Type: raftpb.EntryConfChange, Term: 1, Index: 1, Data: ccdata},
				{Term: 2, Index: 2},
			},
			CommittedEntries: []raftpb.Entry{
				{Type: raftpb.EntryConfChange, Term: 1, Index: 1, Data: ccdata},
				{Term: 2, Index: 2},
			},
		},
		{
			HardState:        raftpb.HardState{Term: 2, Commit: 3, Vote: 1},
			Entries:          []raftpb.Entry{{Term: 2, Index: 3, Data: []byte("foo")}},
			CommittedEntries: []raftpb.Entry{{Term: 2, Index: 3, Data: []byte("foo")}},
		},
	}
	mn := StartMultiNode(1, 10, 1)
	storage := NewMemoryStorage()
	mn.CreateGroup(1, []Peer{{ID: 1}}, storage)
	mn.Campaign(ctx, 1)
	gs := <-mn.Ready()
	g := gs[1]
	if !reflect.DeepEqual(g, wants[0]) {
		t.Fatalf("#%d: g = %+v,\n             w   %+v", 1, g, wants[0])
	} else {
		storage.Append(g.Entries)
		mn.Advance(gs)
	}

	mn.Propose(ctx, 1, []byte("foo"))
	if gs2 := <-mn.Ready(); !reflect.DeepEqual(gs2[1], wants[1]) {
		t.Errorf("#%d: g = %+v,\n             w   %+v", 2, gs2[1], wants[1])
	} else {
		storage.Append(gs2[1].Entries)
		mn.Advance(gs2)
	}

	select {
	case rd := <-mn.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	case <-time.After(time.Millisecond):
	}
}

func TestMultiNodeRestart(t *testing.T) {
	entries := []raftpb.Entry{
		{Term: 1, Index: 1},
		{Term: 1, Index: 2, Data: []byte("foo")},
	}
	st := raftpb.HardState{Term: 1, Commit: 1}

	want := Ready{
		HardState: emptyState,
		// commit up to index commit index in st
		CommittedEntries: entries[:st.Commit],
	}

	storage := NewMemoryStorage()
	storage.SetHardState(st)
	storage.Append(entries)
	mn := StartMultiNode(1, 10, 1)
	mn.CreateGroup(1, nil, storage)
	gs := <-mn.Ready()
	if !reflect.DeepEqual(gs[1], want) {
		t.Errorf("g = %+v,\n             w   %+v", gs[1], want)
	}
	mn.Advance(gs)

	select {
	case rd := <-mn.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	case <-time.After(time.Millisecond):
	}
	mn.Stop()
}

func TestMultiNodeRestartFromSnapshot(t *testing.T) {
	snap := raftpb.Snapshot{
		Metadata: raftpb.SnapshotMetadata{
			ConfState: raftpb.ConfState{Nodes: []uint64{1, 2}},
			Index:     2,
			Term:      1,
		},
	}
	entries := []raftpb.Entry{
		{Term: 1, Index: 3, Data: []byte("foo")},
	}
	st := raftpb.HardState{Term: 1, Commit: 3}

	want := Ready{
		HardState: emptyState,
		// commit up to index commit index in st
		CommittedEntries: entries,
	}

	s := NewMemoryStorage()
	s.SetHardState(st)
	s.ApplySnapshot(snap)
	s.Append(entries)
	mn := StartMultiNode(1, 10, 1)
	mn.CreateGroup(1, nil, s)
	if gs := <-mn.Ready(); !reflect.DeepEqual(gs[1], want) {
		t.Errorf("g = %+v,\n             w   %+v", gs[1], want)
	} else {
		mn.Advance(gs)
	}

	select {
	case rd := <-mn.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	case <-time.After(time.Millisecond):
	}
}

func TestMultiNodeAdvance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storage := NewMemoryStorage()
	mn := StartMultiNode(1, 10, 1)
	mn.CreateGroup(1, []Peer{{ID: 1}}, storage)
	mn.Campaign(ctx, 1)
	rd1 := <-mn.Ready()
	mn.Propose(ctx, 1, []byte("foo"))
	select {
	case rd2 := <-mn.Ready():
		t.Fatalf("unexpected Ready before Advance: %+v", rd2)
	case <-time.After(time.Millisecond):
	}
	storage.Append(rd1[1].Entries)
	mn.Advance(rd1)
	select {
	case <-mn.Ready():
	case <-time.After(time.Millisecond):
		t.Errorf("expect Ready after Advance, but there is no Ready available")
	}
}
