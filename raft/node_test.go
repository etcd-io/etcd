// Copyright 2015 The etcd Authors
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
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/pkg/testutil"
	"go.etcd.io/etcd/raft/raftpb"
)

// readyWithTimeout selects from n.Ready() with a 1-second timeout. It
// panics on timeout, which is better than the indefinite wait that
// would occur if this channel were read without being wrapped in a
// select.
func readyWithTimeout(n Node) Ready {
	select {
	case rd := <-n.Ready():
		return rd
	case <-time.After(time.Second):
		panic("timed out waiting for ready")
	}
}

// TestNodeStep ensures that node.Step sends msgProp to propc chan
// and other kinds of messages to recvc chan.
func TestNodeStep(t *testing.T) {
	for i, msgn := range raftpb.MessageType_name {
		n := &node{
			propc: make(chan msgWithResult, 1),
			recvc: make(chan raftpb.Message, 1),
		}
		msgt := raftpb.MessageType(i)
		n.Step(context.TODO(), raftpb.Message{Type: msgt})
		// Proposal goes to proc chan. Others go to recvc chan.
		if msgt == raftpb.MsgProp {
			select {
			case <-n.propc:
			default:
				t.Errorf("%d: cannot receive %s on propc chan", msgt, msgn)
			}
		} else {
			if IsLocalMsg(msgt) {
				select {
				case <-n.recvc:
					t.Errorf("%d: step should ignore %s", msgt, msgn)
				default:
				}
			} else {
				select {
				case <-n.recvc:
				default:
					t.Errorf("%d: cannot receive %s on recvc chan", msgt, msgn)
				}
			}
		}
	}
}

// Cancel and Stop should unblock Step()
func TestNodeStepUnblock(t *testing.T) {
	// a node without buffer to block step
	n := &node{
		propc: make(chan msgWithResult),
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
			err := n.Step(ctx, raftpb.Message{Type: raftpb.MsgProp})
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
			case <-n.done:
				n.done = make(chan struct{})
			default:
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("#%d: failed to unblock step", i)
		}
	}
}

// TestNodePropose ensures that node.Propose sends the given proposal to the underlying raft.
func TestNodePropose(t *testing.T) {
	msgs := []raftpb.Message{}
	appendStep := func(r *raft, m raftpb.Message) error {
		msgs = append(msgs, m)
		return nil
	}

	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	go n.run(r)
	n.Campaign(context.TODO())
	for {
		rd := <-n.Ready()
		s.Append(rd.Entries)
		// change the step function to appendStep until this raft becomes leader
		if rd.SoftState.Lead == r.id {
			r.step = appendStep
			n.Advance()
			break
		}
		n.Advance()
	}
	n.Propose(context.TODO(), []byte("somedata"))
	n.Stop()

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want %d", len(msgs), 1)
	}
	if msgs[0].Type != raftpb.MsgProp {
		t.Errorf("msg type = %d, want %d", msgs[0].Type, raftpb.MsgProp)
	}
	if !bytes.Equal(msgs[0].Entries[0].Data, []byte("somedata")) {
		t.Errorf("data = %v, want %v", msgs[0].Entries[0].Data, []byte("somedata"))
	}
}

// TestNodeReadIndex ensures that node.ReadIndex sends the MsgReadIndex message to the underlying raft.
// It also ensures that ReadState can be read out through ready chan.
func TestNodeReadIndex(t *testing.T) {
	msgs := []raftpb.Message{}
	appendStep := func(r *raft, m raftpb.Message) error {
		msgs = append(msgs, m)
		return nil
	}
	wrs := []ReadState{{Index: uint64(1), RequestCtx: []byte("somedata")}}

	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	r.readStates = wrs

	go n.run(r)
	n.Campaign(context.TODO())
	for {
		rd := <-n.Ready()
		if !reflect.DeepEqual(rd.ReadStates, wrs) {
			t.Errorf("ReadStates = %v, want %v", rd.ReadStates, wrs)
		}

		s.Append(rd.Entries)

		if rd.SoftState.Lead == r.id {
			n.Advance()
			break
		}
		n.Advance()
	}

	r.step = appendStep
	wrequestCtx := []byte("somedata2")
	n.ReadIndex(context.TODO(), wrequestCtx)
	n.Stop()

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want %d", len(msgs), 1)
	}
	if msgs[0].Type != raftpb.MsgReadIndex {
		t.Errorf("msg type = %d, want %d", msgs[0].Type, raftpb.MsgReadIndex)
	}
	if !bytes.Equal(msgs[0].Entries[0].Data, wrequestCtx) {
		t.Errorf("data = %v, want %v", msgs[0].Entries[0].Data, wrequestCtx)
	}
}

// TestDisableProposalForwarding ensures that proposals are not forwarded to
// the leader when DisableProposalForwarding is true.
func TestDisableProposalForwarding(t *testing.T) {
	r1 := newTestRaft(1, []uint64{1, 2, 3}, 10, 1, NewMemoryStorage())
	r2 := newTestRaft(2, []uint64{1, 2, 3}, 10, 1, NewMemoryStorage())
	cfg3 := newTestConfig(3, []uint64{1, 2, 3}, 10, 1, NewMemoryStorage())
	cfg3.DisableProposalForwarding = true
	r3 := newRaft(cfg3)
	nt := newNetwork(r1, r2, r3)

	// elect r1 as leader
	nt.send(raftpb.Message{From: 1, To: 1, Type: raftpb.MsgHup})

	var testEntries = []raftpb.Entry{{Data: []byte("testdata")}}

	// send proposal to r2(follower) where DisableProposalForwarding is false
	r2.Step(raftpb.Message{From: 2, To: 2, Type: raftpb.MsgProp, Entries: testEntries})

	// verify r2(follower) does forward the proposal when DisableProposalForwarding is false
	if len(r2.msgs) != 1 {
		t.Fatalf("len(r2.msgs) expected 1, got %d", len(r2.msgs))
	}

	// send proposal to r3(follower) where DisableProposalForwarding is true
	r3.Step(raftpb.Message{From: 3, To: 3, Type: raftpb.MsgProp, Entries: testEntries})

	// verify r3(follower) does not forward the proposal when DisableProposalForwarding is true
	if len(r3.msgs) != 0 {
		t.Fatalf("len(r3.msgs) expected 0, got %d", len(r3.msgs))
	}
}

// TestNodeReadIndexToOldLeader ensures that raftpb.MsgReadIndex to old leader
// gets forwarded to the new leader and 'send' method does not attach its term.
func TestNodeReadIndexToOldLeader(t *testing.T) {
	r1 := newTestRaft(1, []uint64{1, 2, 3}, 10, 1, NewMemoryStorage())
	r2 := newTestRaft(2, []uint64{1, 2, 3}, 10, 1, NewMemoryStorage())
	r3 := newTestRaft(3, []uint64{1, 2, 3}, 10, 1, NewMemoryStorage())

	nt := newNetwork(r1, r2, r3)

	// elect r1 as leader
	nt.send(raftpb.Message{From: 1, To: 1, Type: raftpb.MsgHup})

	var testEntries = []raftpb.Entry{{Data: []byte("testdata")}}

	// send readindex request to r2(follower)
	r2.Step(raftpb.Message{From: 2, To: 2, Type: raftpb.MsgReadIndex, Entries: testEntries})

	// verify r2(follower) forwards this message to r1(leader) with term not set
	if len(r2.msgs) != 1 {
		t.Fatalf("len(r2.msgs) expected 1, got %d", len(r2.msgs))
	}
	readIndxMsg1 := raftpb.Message{From: 2, To: 1, Type: raftpb.MsgReadIndex, Entries: testEntries}
	if !reflect.DeepEqual(r2.msgs[0], readIndxMsg1) {
		t.Fatalf("r2.msgs[0] expected %+v, got %+v", readIndxMsg1, r2.msgs[0])
	}

	// send readindex request to r3(follower)
	r3.Step(raftpb.Message{From: 3, To: 3, Type: raftpb.MsgReadIndex, Entries: testEntries})

	// verify r3(follower) forwards this message to r1(leader) with term not set as well.
	if len(r3.msgs) != 1 {
		t.Fatalf("len(r3.msgs) expected 1, got %d", len(r3.msgs))
	}
	readIndxMsg2 := raftpb.Message{From: 3, To: 1, Type: raftpb.MsgReadIndex, Entries: testEntries}
	if !reflect.DeepEqual(r3.msgs[0], readIndxMsg2) {
		t.Fatalf("r3.msgs[0] expected %+v, got %+v", readIndxMsg2, r3.msgs[0])
	}

	// now elect r3 as leader
	nt.send(raftpb.Message{From: 3, To: 3, Type: raftpb.MsgHup})

	// let r1 steps the two messages previously we got from r2, r3
	r1.Step(readIndxMsg1)
	r1.Step(readIndxMsg2)

	// verify r1(follower) forwards these messages again to r3(new leader)
	if len(r1.msgs) != 2 {
		t.Fatalf("len(r1.msgs) expected 1, got %d", len(r1.msgs))
	}
	readIndxMsg3 := raftpb.Message{From: 1, To: 3, Type: raftpb.MsgReadIndex, Entries: testEntries}
	if !reflect.DeepEqual(r1.msgs[0], readIndxMsg3) {
		t.Fatalf("r1.msgs[0] expected %+v, got %+v", readIndxMsg3, r1.msgs[0])
	}
	if !reflect.DeepEqual(r1.msgs[1], readIndxMsg3) {
		t.Fatalf("r1.msgs[1] expected %+v, got %+v", readIndxMsg3, r1.msgs[1])
	}
}

// TestNodeProposeConfig ensures that node.ProposeConfChange sends the given configuration proposal
// to the underlying raft.
func TestNodeProposeConfig(t *testing.T) {
	msgs := []raftpb.Message{}
	appendStep := func(r *raft, m raftpb.Message) error {
		msgs = append(msgs, m)
		return nil
	}

	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	go n.run(r)
	n.Campaign(context.TODO())
	for {
		rd := <-n.Ready()
		s.Append(rd.Entries)
		// change the step function to appendStep until this raft becomes leader
		if rd.SoftState.Lead == r.id {
			r.step = appendStep
			n.Advance()
			break
		}
		n.Advance()
	}
	cc := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1}
	ccdata, err := cc.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	n.ProposeConfChange(context.TODO(), cc)
	n.Stop()

	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want %d", len(msgs), 1)
	}
	if msgs[0].Type != raftpb.MsgProp {
		t.Errorf("msg type = %d, want %d", msgs[0].Type, raftpb.MsgProp)
	}
	if !bytes.Equal(msgs[0].Entries[0].Data, ccdata) {
		t.Errorf("data = %v, want %v", msgs[0].Entries[0].Data, ccdata)
	}
}

// TestNodeProposeAddDuplicateNode ensures that two proposes to add the same node should
// not affect the later propose to add new node.
func TestNodeProposeAddDuplicateNode(t *testing.T) {
	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	go n.run(r)
	n.Campaign(context.TODO())
	rdyEntries := make([]raftpb.Entry, 0)
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	done := make(chan struct{})
	stop := make(chan struct{})
	applyConfChan := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				n.Tick()
			case rd := <-n.Ready():
				s.Append(rd.Entries)
				applied := false
				for _, e := range rd.Entries {
					rdyEntries = append(rdyEntries, e)
					switch e.Type {
					case raftpb.EntryNormal:
					case raftpb.EntryConfChange:
						var cc raftpb.ConfChange
						cc.Unmarshal(e.Data)
						n.ApplyConfChange(cc)
						applied = true
					}
				}
				n.Advance()
				if applied {
					applyConfChan <- struct{}{}
				}
			}
		}
	}()

	cc1 := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1}
	ccdata1, _ := cc1.Marshal()
	n.ProposeConfChange(context.TODO(), cc1)
	<-applyConfChan

	// try add the same node again
	n.ProposeConfChange(context.TODO(), cc1)
	<-applyConfChan

	// the new node join should be ok
	cc2 := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 2}
	ccdata2, _ := cc2.Marshal()
	n.ProposeConfChange(context.TODO(), cc2)
	<-applyConfChan

	close(stop)
	<-done

	if len(rdyEntries) != 4 {
		t.Errorf("len(entry) = %d, want %d, %v\n", len(rdyEntries), 4, rdyEntries)
	}
	if !bytes.Equal(rdyEntries[1].Data, ccdata1) {
		t.Errorf("data = %v, want %v", rdyEntries[1].Data, ccdata1)
	}
	if !bytes.Equal(rdyEntries[3].Data, ccdata2) {
		t.Errorf("data = %v, want %v", rdyEntries[3].Data, ccdata2)
	}
	n.Stop()
}

// TestBlockProposal ensures that node will block proposal when it does not
// know who is the current leader; node will accept proposal when it knows
// who is the current leader.
func TestBlockProposal(t *testing.T) {
	n := newNode()
	r := newTestRaft(1, []uint64{1}, 10, 1, NewMemoryStorage())
	go n.run(r)
	defer n.Stop()

	errc := make(chan error, 1)
	go func() {
		errc <- n.Propose(context.TODO(), []byte("somedata"))
	}()

	testutil.WaitSchedule()
	select {
	case err := <-errc:
		t.Errorf("err = %v, want blocking", err)
	default:
	}

	n.Campaign(context.TODO())
	select {
	case err := <-errc:
		if err != nil {
			t.Errorf("err = %v, want %v", err, nil)
		}
	case <-time.After(10 * time.Second):
		t.Errorf("blocking proposal, want unblocking")
	}
}

func TestNodeProposeWaitDropped(t *testing.T) {
	msgs := []raftpb.Message{}
	droppingMsg := []byte("test_dropping")
	dropStep := func(r *raft, m raftpb.Message) error {
		if m.Type == raftpb.MsgProp && strings.Contains(m.String(), string(droppingMsg)) {
			t.Logf("dropping message: %v", m.String())
			return ErrProposalDropped
		}
		msgs = append(msgs, m)
		return nil
	}

	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	go n.run(r)
	n.Campaign(context.TODO())
	for {
		rd := <-n.Ready()
		s.Append(rd.Entries)
		// change the step function to dropStep until this raft becomes leader
		if rd.SoftState.Lead == r.id {
			r.step = dropStep
			n.Advance()
			break
		}
		n.Advance()
	}
	proposalTimeout := time.Millisecond * 100
	ctx, cancel := context.WithTimeout(context.Background(), proposalTimeout)
	// propose with cancel should be cancelled earyly if dropped
	err := n.Propose(ctx, droppingMsg)
	if err != ErrProposalDropped {
		t.Errorf("should drop proposal : %v", err)
	}
	cancel()

	n.Stop()
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want %d", len(msgs), 1)
	}
}

// TestNodeTick ensures that node.Tick() will increase the
// elapsed of the underlying raft state machine.
func TestNodeTick(t *testing.T) {
	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	go n.run(r)
	elapsed := r.electionElapsed
	n.Tick()

	for len(n.tickc) != 0 {
		time.Sleep(100 * time.Millisecond)
	}

	n.Stop()
	if r.electionElapsed != elapsed+1 {
		t.Errorf("elapsed = %d, want %d", r.electionElapsed, elapsed+1)
	}
}

// TestNodeStop ensures that node.Stop() blocks until the node has stopped
// processing, and that it is idempotent
func TestNodeStop(t *testing.T) {
	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	donec := make(chan struct{})

	go func() {
		n.run(r)
		close(donec)
	}()

	status := n.Status()
	n.Stop()

	select {
	case <-donec:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for node to stop!")
	}

	emptyStatus := Status{}

	if reflect.DeepEqual(status, emptyStatus) {
		t.Errorf("status = %v, want not empty", status)
	}
	// Further status should return be empty, the node is stopped.
	status = n.Status()
	if !reflect.DeepEqual(status, emptyStatus) {
		t.Errorf("status = %v, want empty", status)
	}
	// Subsequent Stops should have no effect.
	n.Stop()
}

func TestReadyContainUpdates(t *testing.T) {
	tests := []struct {
		rd       Ready
		wcontain bool
	}{
		{Ready{}, false},
		{Ready{SoftState: &SoftState{Lead: 1}}, true},
		{Ready{HardState: raftpb.HardState{Vote: 1}}, true},
		{Ready{Entries: make([]raftpb.Entry, 1)}, true},
		{Ready{CommittedEntries: make([]raftpb.Entry, 1)}, true},
		{Ready{Messages: make([]raftpb.Message, 1)}, true},
		{Ready{Snapshot: raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: 1}}}, true},
	}

	for i, tt := range tests {
		if g := tt.rd.containsUpdates(); g != tt.wcontain {
			t.Errorf("#%d: containUpdates = %v, want %v", i, g, tt.wcontain)
		}
	}
}

// TestNodeStart ensures that a node can be started correctly. The node should
// start with correct configuration change entries, and can accept and commit
// proposals.
func TestNodeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cc := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1}
	ccdata, err := cc.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	wants := []Ready{
		{
			HardState: raftpb.HardState{Term: 1, Commit: 1, Vote: 0},
			Entries: []raftpb.Entry{
				{Type: raftpb.EntryConfChange, Term: 1, Index: 1, Data: ccdata},
			},
			CommittedEntries: []raftpb.Entry{
				{Type: raftpb.EntryConfChange, Term: 1, Index: 1, Data: ccdata},
			},
			MustSync: true,
		},
		{
			HardState:        raftpb.HardState{Term: 2, Commit: 3, Vote: 1},
			Entries:          []raftpb.Entry{{Term: 2, Index: 3, Data: []byte("foo")}},
			CommittedEntries: []raftpb.Entry{{Term: 2, Index: 3, Data: []byte("foo")}},
			MustSync:         true,
		},
	}
	storage := NewMemoryStorage()
	c := &Config{
		ID:              1,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   noLimit,
		MaxInflightMsgs: 256,
	}
	n := StartNode(c, []Peer{{ID: 1}})
	defer n.Stop()
	g := <-n.Ready()
	if !reflect.DeepEqual(g, wants[0]) {
		t.Fatalf("#%d: g = %+v,\n             w   %+v", 1, g, wants[0])
	} else {
		storage.Append(g.Entries)
		n.Advance()
	}

	n.Campaign(ctx)
	rd := <-n.Ready()
	storage.Append(rd.Entries)
	n.Advance()

	n.Propose(ctx, []byte("foo"))
	if g2 := <-n.Ready(); !reflect.DeepEqual(g2, wants[1]) {
		t.Errorf("#%d: g = %+v,\n             w   %+v", 2, g2, wants[1])
	} else {
		storage.Append(g2.Entries)
		n.Advance()
	}

	select {
	case rd := <-n.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	case <-time.After(time.Millisecond):
	}
}

func TestNodeRestart(t *testing.T) {
	entries := []raftpb.Entry{
		{Term: 1, Index: 1},
		{Term: 1, Index: 2, Data: []byte("foo")},
	}
	st := raftpb.HardState{Term: 1, Commit: 1}

	want := Ready{
		HardState: st,
		// commit up to index commit index in st
		CommittedEntries: entries[:st.Commit],
		MustSync:         true,
	}

	storage := NewMemoryStorage()
	storage.SetHardState(st)
	storage.Append(entries)
	c := &Config{
		ID:              1,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   noLimit,
		MaxInflightMsgs: 256,
	}
	n := RestartNode(c)
	defer n.Stop()
	if g := <-n.Ready(); !reflect.DeepEqual(g, want) {
		t.Errorf("g = %+v,\n             w   %+v", g, want)
	}
	n.Advance()

	select {
	case rd := <-n.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	case <-time.After(time.Millisecond):
	}
}

func TestNodeRestartFromSnapshot(t *testing.T) {
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
		HardState: st,
		// commit up to index commit index in st
		CommittedEntries: entries,
		MustSync:         true,
	}

	s := NewMemoryStorage()
	s.SetHardState(st)
	s.ApplySnapshot(snap)
	s.Append(entries)
	c := &Config{
		ID:              1,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         s,
		MaxSizePerMsg:   noLimit,
		MaxInflightMsgs: 256,
	}
	n := RestartNode(c)
	defer n.Stop()
	if g := <-n.Ready(); !reflect.DeepEqual(g, want) {
		t.Errorf("g = %+v,\n             w   %+v", g, want)
	} else {
		n.Advance()
	}

	select {
	case rd := <-n.Ready():
		t.Errorf("unexpected Ready: %+v", rd)
	case <-time.After(time.Millisecond):
	}
}

func TestNodeAdvance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	storage := NewMemoryStorage()
	c := &Config{
		ID:              1,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   noLimit,
		MaxInflightMsgs: 256,
	}
	n := StartNode(c, []Peer{{ID: 1}})
	defer n.Stop()
	rd := <-n.Ready()
	storage.Append(rd.Entries)
	n.Advance()

	n.Campaign(ctx)
	<-n.Ready()

	n.Propose(ctx, []byte("foo"))
	select {
	case rd = <-n.Ready():
		t.Fatalf("unexpected Ready before Advance: %+v", rd)
	case <-time.After(time.Millisecond):
	}
	storage.Append(rd.Entries)
	n.Advance()
	select {
	case <-n.Ready():
	case <-time.After(100 * time.Millisecond):
		t.Errorf("expect Ready after Advance, but there is no Ready available")
	}
}

func TestSoftStateEqual(t *testing.T) {
	tests := []struct {
		st *SoftState
		we bool
	}{
		{&SoftState{}, true},
		{&SoftState{Lead: 1}, false},
		{&SoftState{RaftState: StateLeader}, false},
	}
	for i, tt := range tests {
		if g := tt.st.equal(&SoftState{}); g != tt.we {
			t.Errorf("#%d, equal = %v, want %v", i, g, tt.we)
		}
	}
}

func TestIsHardStateEqual(t *testing.T) {
	tests := []struct {
		st raftpb.HardState
		we bool
	}{
		{emptyState, true},
		{raftpb.HardState{Vote: 1}, false},
		{raftpb.HardState{Commit: 1}, false},
		{raftpb.HardState{Term: 1}, false},
	}

	for i, tt := range tests {
		if isHardStateEqual(tt.st, emptyState) != tt.we {
			t.Errorf("#%d, equal = %v, want %v", i, isHardStateEqual(tt.st, emptyState), tt.we)
		}
	}
}

func TestNodeProposeAddLearnerNode(t *testing.T) {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	n := newNode()
	s := NewMemoryStorage()
	r := newTestRaft(1, []uint64{1}, 10, 1, s)
	go n.run(r)
	n.Campaign(context.TODO())
	stop := make(chan struct{})
	done := make(chan struct{})
	applyConfChan := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				n.Tick()
			case rd := <-n.Ready():
				s.Append(rd.Entries)
				t.Logf("raft: %v", rd.Entries)
				for _, ent := range rd.Entries {
					if ent.Type != raftpb.EntryConfChange {
						continue
					}
					var cc raftpb.ConfChange
					cc.Unmarshal(ent.Data)
					state := n.ApplyConfChange(cc)
					if len(state.Learners) == 0 ||
						state.Learners[0] != cc.NodeID ||
						cc.NodeID != 2 {
						t.Errorf("apply conf change should return new added learner: %v", state.String())
					}

					if len(state.Nodes) != 1 {
						t.Errorf("add learner should not change the nodes: %v", state.String())
					}
					t.Logf("apply raft conf %v changed to: %v", cc, state.String())
					applyConfChan <- struct{}{}
				}
				n.Advance()
			}
		}
	}()
	cc := raftpb.ConfChange{Type: raftpb.ConfChangeAddLearnerNode, NodeID: 2}
	n.ProposeConfChange(context.TODO(), cc)
	<-applyConfChan
	close(stop)
	<-done
}

func TestAppendPagination(t *testing.T) {
	const maxSizePerMsg = 2048
	n := newNetworkWithConfig(func(c *Config) {
		c.MaxSizePerMsg = maxSizePerMsg
	}, nil, nil, nil)

	seenFullMessage := false
	// Inspect all messages to see that we never exceed the limit, but
	// we do see messages of larger than half the limit.
	n.msgHook = func(m raftpb.Message) bool {
		if m.Type == raftpb.MsgApp {
			size := 0
			for _, e := range m.Entries {
				size += len(e.Data)
			}
			if size > maxSizePerMsg {
				t.Errorf("sent MsgApp that is too large: %d bytes", size)
			}
			if size > maxSizePerMsg/2 {
				seenFullMessage = true
			}
		}
		return true
	}

	n.send(raftpb.Message{From: 1, To: 1, Type: raftpb.MsgHup})

	// Partition the network while we make our proposals. This forces
	// the entries to be batched into larger messages.
	n.isolate(1)
	blob := []byte(strings.Repeat("a", 1000))
	for i := 0; i < 5; i++ {
		n.send(raftpb.Message{From: 1, To: 1, Type: raftpb.MsgProp, Entries: []raftpb.Entry{{Data: blob}}})
	}
	n.recover()

	// After the partition recovers, tick the clock to wake everything
	// back up and send the messages.
	n.send(raftpb.Message{From: 1, To: 1, Type: raftpb.MsgBeat})
	if !seenFullMessage {
		t.Error("didn't see any messages more than half the max size; something is wrong with this test")
	}
}

func TestCommitPagination(t *testing.T) {
	s := NewMemoryStorage()
	cfg := newTestConfig(1, []uint64{1}, 10, 1, s)
	cfg.MaxCommittedSizePerReady = 2048
	r := newRaft(cfg)
	n := newNode()
	go n.run(r)
	n.Campaign(context.TODO())

	rd := readyWithTimeout(&n)
	if len(rd.CommittedEntries) != 1 {
		t.Fatalf("expected 1 (empty) entry, got %d", len(rd.CommittedEntries))
	}
	s.Append(rd.Entries)
	n.Advance()

	blob := []byte(strings.Repeat("a", 1000))
	for i := 0; i < 3; i++ {
		if err := n.Propose(context.TODO(), blob); err != nil {
			t.Fatal(err)
		}
	}

	// The 3 proposals will commit in two batches.
	rd = readyWithTimeout(&n)
	if len(rd.CommittedEntries) != 2 {
		t.Fatalf("expected 2 entries in first batch, got %d", len(rd.CommittedEntries))
	}
	s.Append(rd.Entries)
	n.Advance()
	rd = readyWithTimeout(&n)
	if len(rd.CommittedEntries) != 1 {
		t.Fatalf("expected 1 entry in second batch, got %d", len(rd.CommittedEntries))
	}
	s.Append(rd.Entries)
	n.Advance()
}

type ignoreSizeHintMemStorage struct {
	*MemoryStorage
}

func (s *ignoreSizeHintMemStorage) Entries(lo, hi uint64, maxSize uint64) ([]raftpb.Entry, error) {
	return s.MemoryStorage.Entries(lo, hi, math.MaxUint64)
}

// TestNodeCommitPaginationAfterRestart regression tests a scenario in which the
// Storage's Entries size limitation is slightly more permissive than Raft's
// internal one. The original bug was the following:
//
// - node learns that index 11 (or 100, doesn't matter) is committed
// - nextEnts returns index 1..10 in CommittedEntries due to size limiting. However,
//   index 10 already exceeds maxBytes, due to a user-provided impl of Entries.
// - Commit index gets bumped to 10
// - the node persists the HardState, but crashes before applying the entries
// - upon restart, the storage returns the same entries, but `slice` takes a different code path
//   (since it is now called with an upper bound of 10) and removes the last entry.
// - Raft emits a HardState with a regressing commit index.
//
// A simpler version of this test would have the storage return a lot less entries than dictated
// by maxSize (for example, exactly one entry) after the restart, resulting in a larger regression.
// This wouldn't need to exploit anything about Raft-internal code paths to fail.
func TestNodeCommitPaginationAfterRestart(t *testing.T) {
	s := &ignoreSizeHintMemStorage{
		MemoryStorage: NewMemoryStorage(),
	}
	persistedHardState := raftpb.HardState{
		Term:   1,
		Vote:   1,
		Commit: 10,
	}

	s.hardState = persistedHardState
	s.ents = make([]raftpb.Entry, 10)
	var size uint64
	for i := range s.ents {
		ent := raftpb.Entry{
			Term:  1,
			Index: uint64(i + 1),
			Type:  raftpb.EntryNormal,
			Data:  []byte("a"),
		}

		s.ents[i] = ent
		size += uint64(ent.Size())
	}

	cfg := newTestConfig(1, []uint64{1}, 10, 1, s)
	// Set a MaxSizePerMsg that would suggest to Raft that the last committed entry should
	// not be included in the initial rd.CommittedEntries. However, our storage will ignore
	// this and *will* return it (which is how the Commit index ended up being 10 initially).
	cfg.MaxSizePerMsg = size - uint64(s.ents[len(s.ents)-1].Size()) - 1

	r := newRaft(cfg)
	n := newNode()
	go n.run(r)
	defer n.Stop()

	rd := readyWithTimeout(&n)
	if !IsEmptyHardState(rd.HardState) && rd.HardState.Commit < persistedHardState.Commit {
		t.Errorf("HardState regressed: Commit %d -> %d\nCommitting:\n%+v",
			persistedHardState.Commit, rd.HardState.Commit,
			DescribeEntries(rd.CommittedEntries, func(data []byte) string { return fmt.Sprintf("%q", data) }),
		)
	}
}

// TestNodeBoundedLogGrowthWithPartition tests a scenario where a leader is
// partitioned from a quorum of nodes. It verifies that the leader's log is
// protected from unbounded growth even as new entries continue to be proposed.
// This protection is provided by the MaxUncommittedEntriesSize configuration.
func TestNodeBoundedLogGrowthWithPartition(t *testing.T) {
	const maxEntries = 16
	data := []byte("testdata")
	testEntry := raftpb.Entry{Data: data}
	maxEntrySize := uint64(maxEntries * PayloadSize(testEntry))

	s := NewMemoryStorage()
	cfg := newTestConfig(1, []uint64{1}, 10, 1, s)
	cfg.MaxUncommittedEntriesSize = maxEntrySize
	r := newRaft(cfg)
	n := newNode()
	go n.run(r)
	defer n.Stop()
	n.Campaign(context.TODO())

	rd := readyWithTimeout(&n)
	if len(rd.CommittedEntries) != 1 {
		t.Fatalf("expected 1 (empty) entry, got %d", len(rd.CommittedEntries))
	}
	s.Append(rd.Entries)
	n.Advance()

	// Simulate a network partition while we make our proposals by never
	// committing anything. These proposals should not cause the leader's
	// log to grow indefinitely.
	for i := 0; i < 1024; i++ {
		n.Propose(context.TODO(), data)
	}

	// Check the size of leader's uncommitted log tail. It should not exceed the
	// MaxUncommittedEntriesSize limit.
	checkUncommitted := func(exp uint64) {
		t.Helper()
		if a := r.uncommittedSize; exp != a {
			t.Fatalf("expected %d uncommitted entry bytes, found %d", exp, a)
		}
	}
	checkUncommitted(maxEntrySize)

	// Recover from the partition. The uncommitted tail of the Raft log should
	// disappear as entries are committed.
	rd = readyWithTimeout(&n)
	if len(rd.CommittedEntries) != maxEntries {
		t.Fatalf("expected %d entries, got %d", maxEntries, len(rd.CommittedEntries))
	}
	s.Append(rd.Entries)
	n.Advance()
	checkUncommitted(0)
}
