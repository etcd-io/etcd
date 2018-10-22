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
	"reflect"
	"testing"

	"go.etcd.io/etcd/raft/raftpb"
)

// TestRawNodeStep ensures that RawNode.Step ignore local message.
func TestRawNodeStep(t *testing.T) {
	for i, msgn := range raftpb.MessageType_name {
		s := NewMemoryStorage()
		rawNode, err := NewRawNode(newTestConfig(1, nil, 10, 1, s), []Peer{{ID: 1}})
		if err != nil {
			t.Fatal(err)
		}
		msgt := raftpb.MessageType(i)
		err = rawNode.Step(raftpb.Message{Type: msgt})
		// LocalMsg should be ignored.
		if IsLocalMsg(msgt) {
			if err != ErrStepLocalMsg {
				t.Errorf("%d: step should ignore %s", msgt, msgn)
			}
		}
	}
}

// TestNodeStepUnblock from node_test.go has no equivalent in rawNode because there is
// no goroutine in RawNode.

// TestRawNodeProposeAndConfChange ensures that RawNode.Propose and RawNode.ProposeConfChange
// send the given proposal and ConfChange to the underlying raft.
func TestRawNodeProposeAndConfChange(t *testing.T) {
	s := NewMemoryStorage()
	var err error
	rawNode, err := NewRawNode(newTestConfig(1, nil, 10, 1, s), []Peer{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rd := rawNode.Ready()
	s.Append(rd.Entries)
	rawNode.Advance(rd)

	if d := rawNode.Ready(); d.MustSync || !IsEmptyHardState(d.HardState) || len(d.Entries) > 0 {
		t.Fatalf("expected empty hard state with must-sync=false: %#v", d)
	}

	rawNode.Campaign()
	proposed := false
	var (
		lastIndex uint64
		ccdata    []byte
	)
	for {
		rd = rawNode.Ready()
		s.Append(rd.Entries)
		// Once we are the leader, propose a command and a ConfChange.
		if !proposed && rd.SoftState.Lead == rawNode.raft.id {
			rawNode.Propose([]byte("somedata"))

			cc := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1}
			ccdata, err = cc.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			rawNode.ProposeConfChange(cc)

			proposed = true
		}
		rawNode.Advance(rd)

		// Exit when we have four entries: one ConfChange, one no-op for the election,
		// our proposed command and proposed ConfChange.
		lastIndex, err = s.LastIndex()
		if err != nil {
			t.Fatal(err)
		}
		if lastIndex >= 4 {
			break
		}
	}

	entries, err := s.Entries(lastIndex-1, lastIndex+1, noLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want %d", len(entries), 2)
	}
	if !bytes.Equal(entries[0].Data, []byte("somedata")) {
		t.Errorf("entries[0].Data = %v, want %v", entries[0].Data, []byte("somedata"))
	}
	if entries[1].Type != raftpb.EntryConfChange {
		t.Fatalf("type = %v, want %v", entries[1].Type, raftpb.EntryConfChange)
	}
	if !bytes.Equal(entries[1].Data, ccdata) {
		t.Errorf("data = %v, want %v", entries[1].Data, ccdata)
	}
}

// TestRawNodeProposeAddDuplicateNode ensures that two proposes to add the same node should
// not affect the later propose to add new node.
func TestRawNodeProposeAddDuplicateNode(t *testing.T) {
	s := NewMemoryStorage()
	rawNode, err := NewRawNode(newTestConfig(1, nil, 10, 1, s), []Peer{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rd := rawNode.Ready()
	s.Append(rd.Entries)
	rawNode.Advance(rd)

	rawNode.Campaign()
	for {
		rd = rawNode.Ready()
		s.Append(rd.Entries)
		if rd.SoftState.Lead == rawNode.raft.id {
			rawNode.Advance(rd)
			break
		}
		rawNode.Advance(rd)
	}

	proposeConfChangeAndApply := func(cc raftpb.ConfChange) {
		rawNode.ProposeConfChange(cc)
		rd = rawNode.Ready()
		s.Append(rd.Entries)
		for _, entry := range rd.CommittedEntries {
			if entry.Type == raftpb.EntryConfChange {
				var cc raftpb.ConfChange
				cc.Unmarshal(entry.Data)
				rawNode.ApplyConfChange(cc)
			}
		}
		rawNode.Advance(rd)
	}

	cc1 := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1}
	ccdata1, err := cc1.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	proposeConfChangeAndApply(cc1)

	// try to add the same node again
	proposeConfChangeAndApply(cc1)

	// the new node join should be ok
	cc2 := raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 2}
	ccdata2, err := cc2.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	proposeConfChangeAndApply(cc2)

	lastIndex, err := s.LastIndex()
	if err != nil {
		t.Fatal(err)
	}

	// the last three entries should be: ConfChange cc1, cc1, cc2
	entries, err := s.Entries(lastIndex-2, lastIndex+1, noLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want %d", len(entries), 3)
	}
	if !bytes.Equal(entries[0].Data, ccdata1) {
		t.Errorf("entries[0].Data = %v, want %v", entries[0].Data, ccdata1)
	}
	if !bytes.Equal(entries[2].Data, ccdata2) {
		t.Errorf("entries[2].Data = %v, want %v", entries[2].Data, ccdata2)
	}
}

// TestRawNodeReadIndex ensures that Rawnode.ReadIndex sends the MsgReadIndex message
// to the underlying raft. It also ensures that ReadState can be read out.
func TestRawNodeReadIndex(t *testing.T) {
	msgs := []raftpb.Message{}
	appendStep := func(r *raft, m raftpb.Message) error {
		msgs = append(msgs, m)
		return nil
	}
	wrs := []ReadState{{Index: uint64(1), RequestCtx: []byte("somedata")}}

	s := NewMemoryStorage()
	c := newTestConfig(1, nil, 10, 1, s)
	rawNode, err := NewRawNode(c, []Peer{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rawNode.raft.readStates = wrs
	// ensure the ReadStates can be read out
	hasReady := rawNode.HasReady()
	if !hasReady {
		t.Errorf("HasReady() returns %t, want %t", hasReady, true)
	}
	rd := rawNode.Ready()
	if !reflect.DeepEqual(rd.ReadStates, wrs) {
		t.Errorf("ReadStates = %d, want %d", rd.ReadStates, wrs)
	}
	s.Append(rd.Entries)
	rawNode.Advance(rd)
	// ensure raft.readStates is reset after advance
	if rawNode.raft.readStates != nil {
		t.Errorf("readStates = %v, want %v", rawNode.raft.readStates, nil)
	}

	wrequestCtx := []byte("somedata2")
	rawNode.Campaign()
	for {
		rd = rawNode.Ready()
		s.Append(rd.Entries)

		if rd.SoftState.Lead == rawNode.raft.id {
			rawNode.Advance(rd)

			// Once we are the leader, issue a ReadIndex request
			rawNode.raft.step = appendStep
			rawNode.ReadIndex(wrequestCtx)
			break
		}
		rawNode.Advance(rd)
	}
	// ensure that MsgReadIndex message is sent to the underlying raft
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

// TestBlockProposal from node_test.go has no equivalent in rawNode because there is
// no leader check in RawNode.

// TestNodeTick from node_test.go has no equivalent in rawNode because
// it reaches into the raft object which is not exposed.

// TestNodeStop from node_test.go has no equivalent in rawNode because there is
// no goroutine in RawNode.

// TestRawNodeStart ensures that a node can be started correctly. The node should
// start with correct configuration change entries, and can accept and commit
// proposals.
func TestRawNodeStart(t *testing.T) {
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
	rawNode, err := NewRawNode(newTestConfig(1, nil, 10, 1, storage), []Peer{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rd := rawNode.Ready()
	t.Logf("rd %v", rd)
	if !reflect.DeepEqual(rd, wants[0]) {
		t.Fatalf("#%d: g = %+v,\n             w   %+v", 1, rd, wants[0])
	} else {
		storage.Append(rd.Entries)
		rawNode.Advance(rd)
	}
	storage.Append(rd.Entries)
	rawNode.Advance(rd)

	rawNode.Campaign()
	rd = rawNode.Ready()
	storage.Append(rd.Entries)
	rawNode.Advance(rd)

	rawNode.Propose([]byte("foo"))
	if rd = rawNode.Ready(); !reflect.DeepEqual(rd, wants[1]) {
		t.Errorf("#%d: g = %+v,\n             w   %+v", 2, rd, wants[1])
	} else {
		storage.Append(rd.Entries)
		rawNode.Advance(rd)
	}

	if rawNode.HasReady() {
		t.Errorf("unexpected Ready: %+v", rawNode.Ready())
	}
}

func TestRawNodeRestart(t *testing.T) {
	entries := []raftpb.Entry{
		{Term: 1, Index: 1},
		{Term: 1, Index: 2, Data: []byte("foo")},
	}
	st := raftpb.HardState{Term: 1, Commit: 1}

	want := Ready{
		HardState: emptyState,
		// commit up to commit index in st
		CommittedEntries: entries[:st.Commit],
		MustSync:         false,
	}

	storage := NewMemoryStorage()
	storage.SetHardState(st)
	storage.Append(entries)
	rawNode, err := NewRawNode(newTestConfig(1, nil, 10, 1, storage), nil)
	if err != nil {
		t.Fatal(err)
	}
	rd := rawNode.Ready()
	if !reflect.DeepEqual(rd, want) {
		t.Errorf("g = %+v,\n             w   %+v", rd, want)
	}
	rawNode.Advance(rd)
	if rawNode.HasReady() {
		t.Errorf("unexpected Ready: %+v", rawNode.Ready())
	}
}

func TestRawNodeRestartFromSnapshot(t *testing.T) {
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
		// commit up to commit index in st
		CommittedEntries: entries,
		MustSync:         false,
	}

	s := NewMemoryStorage()
	s.SetHardState(st)
	s.ApplySnapshot(snap)
	s.Append(entries)
	rawNode, err := NewRawNode(newTestConfig(1, nil, 10, 1, s), nil)
	if err != nil {
		t.Fatal(err)
	}
	if rd := rawNode.Ready(); !reflect.DeepEqual(rd, want) {
		t.Errorf("g = %+v,\n             w   %+v", rd, want)
	} else {
		rawNode.Advance(rd)
	}
	if rawNode.HasReady() {
		t.Errorf("unexpected Ready: %+v", rawNode.HasReady())
	}
}

// TestNodeAdvance from node_test.go has no equivalent in rawNode because there is
// no dependency check between Ready() and Advance()

func TestRawNodeStatus(t *testing.T) {
	storage := NewMemoryStorage()
	rawNode, err := NewRawNode(newTestConfig(1, nil, 10, 1, storage), []Peer{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	status := rawNode.Status()
	if status == nil {
		t.Errorf("expected status struct, got nil")
	}
}

// TestRawNodeCommitPaginationAfterRestart is the RawNode version of
// TestNodeCommitPaginationAfterRestart. The anomaly here was even worse as the
// Raft group would forget to apply entries:
//
// - node learns that index 11 is committed
// - nextEnts returns index 1..10 in CommittedEntries (but index 10 already
//   exceeds maxBytes), which isn't noticed internally by Raft
// - Commit index gets bumped to 10
// - the node persists the HardState, but crashes before applying the entries
// - upon restart, the storage returns the same entries, but `slice` takes a
//   different code path and removes the last entry.
// - Raft does not emit a HardState, but when the app calls Advance(), it bumps
//   its internal applied index cursor to 10 (when it should be 9)
// - the next Ready asks the app to apply index 11 (omitting index 10), losing a
//    write.
func TestRawNodeCommitPaginationAfterRestart(t *testing.T) {
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

	s.ents = append(s.ents, raftpb.Entry{
		Term:  1,
		Index: uint64(11),
		Type:  raftpb.EntryNormal,
		Data:  []byte("boom"),
	})

	rawNode, err := NewRawNode(cfg, []Peer{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}

	for highestApplied := uint64(0); highestApplied != 11; {
		rd := rawNode.Ready()
		n := len(rd.CommittedEntries)
		if n == 0 {
			t.Fatalf("stopped applying entries at index %d", highestApplied)
		}
		if next := rd.CommittedEntries[0].Index; highestApplied != 0 && highestApplied+1 != next {
			t.Fatalf("attempting to apply index %d after index %d, leaving a gap", next, highestApplied)
		}
		highestApplied = rd.CommittedEntries[n-1].Index
		rawNode.Advance(rd)
		rawNode.Step(raftpb.Message{
			Type:   raftpb.MsgHeartbeat,
			To:     1,
			From:   1, // illegal, but we get away with it
			Term:   1,
			Commit: 11,
		})
	}
}

// TestRawNodeBoundedLogGrowthWithPartition tests a scenario where a leader is
// partitioned from a quorum of nodes. It verifies that the leader's log is
// protected from unbounded growth even as new entries continue to be proposed.
// This protection is provided by the MaxUncommittedEntriesSize configuration.
func TestRawNodeBoundedLogGrowthWithPartition(t *testing.T) {
	const maxEntries = 16
	data := []byte("testdata")
	testEntry := raftpb.Entry{Data: data}
	maxEntrySize := uint64(maxEntries * PayloadSize(testEntry))

	s := NewMemoryStorage()
	cfg := newTestConfig(1, []uint64{1}, 10, 1, s)
	cfg.MaxUncommittedEntriesSize = maxEntrySize
	rawNode, err := NewRawNode(cfg, []Peer{{ID: 1}})
	if err != nil {
		t.Fatal(err)
	}
	rd := rawNode.Ready()
	s.Append(rd.Entries)
	rawNode.Advance(rd)

	// Become the leader.
	rawNode.Campaign()
	for {
		rd = rawNode.Ready()
		s.Append(rd.Entries)
		if rd.SoftState.Lead == rawNode.raft.id {
			rawNode.Advance(rd)
			break
		}
		rawNode.Advance(rd)
	}

	// Simulate a network partition while we make our proposals by never
	// committing anything. These proposals should not cause the leader's
	// log to grow indefinitely.
	for i := 0; i < 1024; i++ {
		rawNode.Propose(data)
	}

	// Check the size of leader's uncommitted log tail. It should not exceed the
	// MaxUncommittedEntriesSize limit.
	checkUncommitted := func(exp uint64) {
		t.Helper()
		if a := rawNode.raft.uncommittedSize; exp != a {
			t.Fatalf("expected %d uncommitted entry bytes, found %d", exp, a)
		}
	}
	checkUncommitted(maxEntrySize)

	// Recover from the partition. The uncommitted tail of the Raft log should
	// disappear as entries are committed.
	rd = rawNode.Ready()
	if len(rd.CommittedEntries) != maxEntries {
		t.Fatalf("expected %d entries, got %d", maxEntries, len(rd.CommittedEntries))
	}
	s.Append(rd.Entries)
	rawNode.Advance(rd)
	checkUncommitted(0)
}
