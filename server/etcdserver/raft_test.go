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

package etcdserver

import (
	"context"
	"encoding/json"
	"expvar"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/testing/protocmp"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/mock/mockstorage"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

func TestGetIDs(t *testing.T) {
	lg := zaptest.NewLogger(t)
	addcc := &raftpb.ConfChange{Type: raftpb.ConfChangeAddNode.Enum(), NodeId: new(uint64(2))}
	addEntry := &raftpb.Entry{Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(addcc)}
	removecc := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode.Enum(), NodeId: new(uint64(2))}
	removeEntry := &raftpb.Entry{Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc)}
	normalEntry := &raftpb.Entry{Type: raftpb.EntryNormal.Enum()}
	updatecc := &raftpb.ConfChange{Type: raftpb.ConfChangeUpdateNode.Enum(), NodeId: new(uint64(2))}
	updateEntry := &raftpb.Entry{Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(updatecc)}

	tests := []struct {
		confState *raftpb.ConfState
		ents      []*raftpb.Entry

		widSet []uint64
	}{
		{nil, []*raftpb.Entry{}, []uint64{}},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]*raftpb.Entry{},
			[]uint64{1},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]*raftpb.Entry{addEntry},
			[]uint64{1, 2},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]*raftpb.Entry{addEntry, removeEntry},
			[]uint64{1},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]*raftpb.Entry{addEntry, normalEntry},
			[]uint64{1, 2},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]*raftpb.Entry{addEntry, normalEntry, updateEntry},
			[]uint64{1, 2},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]*raftpb.Entry{addEntry, removeEntry, normalEntry},
			[]uint64{1},
		},
	}

	for i, tt := range tests {
		var snap raftpb.Snapshot
		if tt.confState != nil {
			snap.Metadata = &raftpb.SnapshotMetadata{ConfState: tt.confState}
		}
		idSet := serverstorage.GetEffectiveNodeIDsFromWALEntries(lg, &snap, tt.ents)
		if !reflect.DeepEqual(idSet, tt.widSet) {
			t.Errorf("#%d: idset = %#v, want %#v", i, idSet, tt.widSet)
		}
	}
}

func TestCreateConfigChangeEnts(t *testing.T) {
	lg := zaptest.NewLogger(t)
	m := membership.Member{
		ID:             types.ID(1),
		RaftAttributes: membership.RaftAttributes{PeerURLs: []string{"http://localhost:2380"}},
	}
	ctx, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	addcc1 := &raftpb.ConfChange{Type: raftpb.ConfChangeAddNode.Enum(), NodeId: new(uint64(1)), Context: ctx}
	removecc2 := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode.Enum(), NodeId: new(uint64(2))}
	removecc3 := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode.Enum(), NodeId: new(uint64(3))}
	tests := []struct {
		ids         []uint64
		self        uint64
		term, index uint64

		wents []*raftpb.Entry
	}{
		{
			[]uint64{1},
			1,
			1, 1,

			nil,
		},
		{
			[]uint64{1, 2},
			1,
			1, 1,

			[]*raftpb.Entry{{Term: new(uint64(1)), Index: new(uint64(2)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc2)}},
		},
		{
			[]uint64{1, 2},
			1,
			2, 2,

			[]*raftpb.Entry{{Term: new(uint64(2)), Index: new(uint64(3)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc2)}},
		},
		{
			[]uint64{1, 2, 3},
			1,
			2, 2,

			[]*raftpb.Entry{
				{Term: new(uint64(2)), Index: new(uint64(3)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc2)},
				{Term: new(uint64(2)), Index: new(uint64(4)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc3)},
			},
		},
		{
			[]uint64{2, 3},
			2,
			2, 2,

			[]*raftpb.Entry{
				{Term: new(uint64(2)), Index: new(uint64(3)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc3)},
			},
		},
		{
			[]uint64{2, 3},
			1,
			2, 2,

			[]*raftpb.Entry{
				{Term: new(uint64(2)), Index: new(uint64(3)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(addcc1)},
				{Term: new(uint64(2)), Index: new(uint64(4)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc2)},
				{Term: new(uint64(2)), Index: new(uint64(5)), Type: raftpb.EntryConfChange.Enum(), Data: pbutil.MustMarshalMessage(removecc3)},
			},
		},
	}

	for i, tt := range tests {
		gents := serverstorage.CreateConfigChangeEnts(lg, tt.ids, tt.self, tt.term, tt.index)
		if diff := cmp.Diff(tt.wents, gents, protocmp.Transform(), cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("#%d: unexpected entries (-want +got):\n%s", i, diff)
		}
	}
}

func TestStopRaftWhenWaitingForApplyDone(t *testing.T) {
	n := newNopReadyNode()
	r := newRaftNode(raftNodeConfig{
		lg:          zaptest.NewLogger(t),
		Node:        n,
		storage:     mockstorage.NewStorageRecorder(""),
		raftStorage: raft.NewMemoryStorage(),
		transport:   newNopTransporter(),
	})
	srv := &EtcdServer{lgMu: new(sync.RWMutex), lg: zaptest.NewLogger(t), r: *r}
	srv.r.start(nil)
	n.readyc <- raft.Ready{}

	stop := func() {
		srv.r.stopped <- struct{}{}
		select {
		case <-srv.r.done:
		case <-time.After(time.Second):
			t.Fatalf("failed to stop raft loop")
		}
	}

	select {
	case <-srv.r.applyc:
	case <-time.After(time.Second):
		stop()
		t.Fatalf("failed to receive toApply struct")
	}

	stop()
}

// TestConfigChangeBlocksApply ensures toApply blocks if committed entries contain config-change.
func TestConfigChangeBlocksApply(t *testing.T) {
	n := newNopReadyNode()

	r := newRaftNode(raftNodeConfig{
		lg:          zaptest.NewLogger(t),
		Node:        n,
		storage:     mockstorage.NewStorageRecorder(""),
		raftStorage: raft.NewMemoryStorage(),
		transport:   newNopTransporter(),
	})
	srv := &EtcdServer{lgMu: new(sync.RWMutex), lg: zaptest.NewLogger(t), r: *r}

	srv.r.start(&raftReadyHandler{
		getLead:          func() uint64 { return 0 },
		updateLead:       func(uint64) {},
		updateLeadership: func(bool) {},
	})
	defer srv.r.stop()

	n.readyc <- raft.Ready{
		SoftState:        &raft.SoftState{RaftState: raft.StateFollower},
		CommittedEntries: []*raftpb.Entry{{Type: raftpb.EntryConfChange.Enum()}},
	}
	ap := <-srv.r.applyc

	continueC := make(chan struct{})
	go func() {
		n.readyc <- raft.Ready{}
		<-srv.r.applyc
		close(continueC)
	}()

	select {
	case <-continueC:
		t.Fatalf("unexpected execution: raft routine should block waiting for toApply")
	case <-time.After(time.Second):
	}

	// finish toApply, unblock raft routine
	<-ap.notifyc

	select {
	case <-ap.raftAdvancedC:
		t.Log("received raft advance notification")
	}

	select {
	case <-continueC:
	case <-time.After(time.Second):
		t.Fatalf("unexpected blocking on execution")
	}
}

func TestProcessDuplicatedAppRespMessage(t *testing.T) {
	n := newNopReadyNode()
	cl := membership.NewCluster(zaptest.NewLogger(t))

	rs := raft.NewMemoryStorage()
	p := mockstorage.NewStorageRecorder("")
	tr, sendc := newSendMsgAppRespTransporter()
	r := newRaftNode(raftNodeConfig{
		lg:          zaptest.NewLogger(t),
		isIDRemoved: func(id uint64) bool { return cl.IsIDRemoved(types.ID(id)) },
		Node:        n,
		transport:   tr,
		storage:     p,
		raftStorage: rs,
	})

	s := &EtcdServer{
		lgMu:    new(sync.RWMutex),
		lg:      zaptest.NewLogger(t),
		r:       *r,
		cluster: cl,
	}

	s.start()
	defer s.Stop()

	lead := uint64(1)

	n.readyc <- raft.Ready{Messages: []*raftpb.Message{
		{Type: raftpb.MsgAppResp.Enum(), From: new(uint64(2)), To: &lead, Term: new(uint64(1)), Index: new(uint64(1))},
		{Type: raftpb.MsgAppResp.Enum(), From: new(uint64(2)), To: &lead, Term: new(uint64(1)), Index: new(uint64(2))},
		{Type: raftpb.MsgAppResp.Enum(), From: new(uint64(2)), To: &lead, Term: new(uint64(1)), Index: new(uint64(3))},
	}}

	got, want := <-sendc, 1
	if got != want {
		t.Errorf("count = %d, want %d", got, want)
	}
}

// TestExpvarWithNoRaftStatus to test that none of the expvars that get added during init panic.
// This matters if another package imports etcdserver, doesn't use it, but does use expvars.
func TestExpvarWithNoRaftStatus(t *testing.T) {
	defer func() {
		if err := recover(); err != nil {
			t.Fatal(err)
		}
	}()
	expvar.Do(func(kv expvar.KeyValue) {
		_ = kv.Value.String()
	})
}

func TestStopRaftNodeMoreThanOnce(t *testing.T) {
	n := newNopReadyNode()
	r := newRaftNode(raftNodeConfig{
		lg:          zaptest.NewLogger(t),
		Node:        n,
		storage:     mockstorage.NewStorageRecorder(""),
		raftStorage: raft.NewMemoryStorage(),
		transport:   newNopTransporter(),
	})
	r.start(&raftReadyHandler{})

	for i := 0; i < 2; i++ {
		stopped := make(chan struct{})
		go func() {
			r.stop()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Errorf("*raftNode.stop() is blocked !")
		}
	}
}

func TestAsyncStorageWritesBatchUsesFinalHardState(t *testing.T) {
	storage := &saveCaptureStorage{Storage: mockstorage.NewStorageRecorder("")}
	r := newRaftNode(raftNodeConfig{
		lg:          zaptest.NewLogger(t),
		Node:        newAsyncReadyNode(),
		localID:     1,
		storage:     storage,
		raftStorage: raft.NewMemoryStorage(),
		transport:   newNopTransporter(),
	})
	first := newStorageAppendMessage(1, 1)
	first.Term = new(uint64(1))
	first.Vote = new(uint64(1))
	first.Commit = new(uint64(1))
	second := newStorageAppendMessage(2, 2)
	second.Term = new(uint64(2))
	second.Vote = new(uint64(2))
	second.Commit = new(uint64(2))

	r.processStorageAppends([]*raftpb.Message{first, second})

	if storage.saveCount != 1 {
		t.Fatalf("save count = %d, want 1", storage.saveCount)
	}
	if got := storage.hardState.GetTerm(); got != 2 {
		t.Fatalf("saved hard state term = %d, want 2", got)
	}
	if got := storage.hardState.GetVote(); got != 2 {
		t.Fatalf("saved hard state vote = %d, want 2", got)
	}
	if got := storage.hardState.GetCommit(); got != 2 {
		t.Fatalf("saved hard state commit = %d, want 2", got)
	}
	if len(storage.entries) != 2 || storage.entries[0].GetIndex() != 1 || storage.entries[1].GetIndex() != 2 {
		t.Fatalf("saved entries = %v, want indexes [1 2]", storage.entries)
	}
}

func TestAsyncStorageWritesAppendFIFO(t *testing.T) {
	n := newAsyncReadyNode()
	saveStartedC := make(chan struct{}, 2)
	allowSaveC := make(chan struct{})
	storage := &blockingSaveStorage{
		Storage:      mockstorage.NewStorageRecorder(""),
		saveStartedC: saveStartedC,
		allowSaveC:   allowSaveC,
	}
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               n,
		localID:            1,
		asyncStorageWrites: true,
		storage:            storage,
		raftStorage:        raft.NewMemoryStorage(),
		transport:          newNopTransporter(),
	})
	r.appendc <- newStorageAppendMessage(1, 1)
	r.appendc <- newStorageAppendMessage(2, 2)
	r.start(newTestRaftReadyHandler())
	defer r.stop()

	<-saveStartedC
	select {
	case m := <-n.stepc:
		t.Fatalf("storage response delivered before append was durable: %v", m)
	default:
	}
	allowSaveC <- struct{}{}
	if got := (<-n.stepc).GetIndex(); got != 1 {
		t.Fatalf("first response index = %d, want 1", got)
	}
	if got := (<-n.stepc).GetIndex(); got != 2 {
		t.Fatalf("second response index = %d, want 2", got)
	}
	select {
	case <-saveStartedC:
		t.Fatal("contiguous appends were not coalesced into one storage save")
	default:
	}

	lastIndex, err := r.raftStorage.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if lastIndex != 2 {
		t.Fatalf("last index = %d, want 2", lastIndex)
	}
	select {
	case <-n.advancec:
		t.Fatal("Advance called with asynchronous storage writes enabled")
	default:
	}
}

func TestAsyncStorageWritesDoesNotBatchOverwrites(t *testing.T) {
	n := newAsyncReadyNode()
	saveStartedC := make(chan struct{}, 2)
	allowSaveC := make(chan struct{})
	storage := &blockingSaveStorage{
		Storage:      mockstorage.NewStorageRecorder(""),
		saveStartedC: saveStartedC,
		allowSaveC:   allowSaveC,
	}
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               n,
		localID:            1,
		asyncStorageWrites: true,
		storage:            storage,
		raftStorage:        raft.NewMemoryStorage(),
		transport:          newNopTransporter(),
	})
	r.appendc <- newStorageAppendMessage(1, 1)
	r.appendc <- newStorageAppendMessage(1, 2)
	r.start(newTestRaftReadyHandler())
	defer r.stop()

	<-saveStartedC
	allowSaveC <- struct{}{}
	if got := (<-n.stepc).GetLogTerm(); got != 1 {
		t.Fatalf("first response term = %d, want 1", got)
	}

	<-saveStartedC
	select {
	case m := <-n.stepc:
		t.Fatalf("overwrite response delivered before its storage save: %v", m)
	default:
	}
	allowSaveC <- struct{}{}
	if got := (<-n.stepc).GetLogTerm(); got != 2 {
		t.Fatalf("second response term = %d, want 2", got)
	}

	term, err := r.raftStorage.Term(1)
	if err != nil {
		t.Fatal(err)
	}
	if term != 2 {
		t.Fatalf("stored term = %d, want 2", term)
	}
}

func TestAsyncStorageWritesApplyResponseAfterApplication(t *testing.T) {
	n := newAsyncReadyNode()
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               n,
		localID:            1,
		asyncStorageWrites: true,
		storage:            mockstorage.NewStorageRecorder(""),
		raftStorage:        raft.NewMemoryStorage(),
		transport:          newNopTransporter(),
	})
	r.start(newTestRaftReadyHandler())
	defer r.stop()

	entry := &raftpb.Entry{Index: new(uint64(1)), Term: new(uint64(1))}
	response := &raftpb.Message{
		Type:    raftpb.MsgStorageApplyResp.Enum(),
		To:      new(uint64(1)),
		From:    new(uint64(raft.LocalApplyThread)),
		Entries: []*raftpb.Entry{entry},
	}
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{{
		Type:      raftpb.MsgStorageApply.Enum(),
		To:        new(uint64(raft.LocalApplyThread)),
		From:      new(uint64(1)),
		Entries:   []*raftpb.Entry{entry},
		Responses: []*raftpb.Message{response},
	}}}

	ap := <-r.applyc
	select {
	case m := <-n.stepc:
		t.Fatalf("apply response delivered before application completed: %v", m)
	default:
	}
	<-ap.notifyc
	ap.onApplied()

	if got := <-n.stepc; got.GetType() != raftpb.MsgStorageApplyResp {
		t.Fatalf("response type = %s, want %s", got.GetType(), raftpb.MsgStorageApplyResp)
	}
	select {
	case <-ap.raftAdvancedC:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Raft apply advancement")
	}
}

func TestAsyncStorageWritesSnapshotResponseAfterApplication(t *testing.T) {
	n := newAsyncReadyNode()
	storage := mockstorage.NewStorageRecorder("")
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               n,
		localID:            1,
		asyncStorageWrites: true,
		storage:            storage,
		raftStorage:        raft.NewMemoryStorage(),
		transport:          newNopTransporter(),
	})
	r.start(newTestRaftReadyHandler())
	defer r.stop()

	snapshot := raftpb.EnsureSnapshot(nil)
	snapshot.Metadata.Index = new(uint64(5))
	snapshot.Metadata.Term = new(uint64(2))
	snapshot.Metadata.ConfState = &raftpb.ConfState{Voters: []uint64{1}}
	response := &raftpb.Message{
		Type:     raftpb.MsgStorageAppendResp.Enum(),
		To:       new(uint64(1)),
		From:     new(uint64(raft.LocalAppendThread)),
		Snapshot: snapshot,
	}
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{{
		Type:      raftpb.MsgStorageAppend.Enum(),
		To:        new(uint64(raft.LocalAppendThread)),
		From:      new(uint64(1)),
		Snapshot:  snapshot,
		Responses: []*raftpb.Message{response},
	}}}

	ap := <-r.applyc
	// The first notification makes the persisted snapshot available to the
	// state machine restore. The second covers the in-memory Raft update and
	// WAL release.
	<-ap.notifyc
	<-ap.notifyc
	select {
	case m := <-n.stepc:
		t.Fatalf("snapshot response delivered before application completed: %v", m)
	default:
	}
	ap.onApplied()

	if got := <-n.stepc; got.GetType() != raftpb.MsgStorageAppendResp {
		t.Fatalf("response type = %s, want %s", got.GetType(), raftpb.MsgStorageAppendResp)
	}
	storedSnapshot, err := r.raftStorage.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if got := storedSnapshot.Metadata.GetIndex(); got != 5 {
		t.Fatalf("snapshot index = %d, want 5", got)
	}
	actions, err := storage.Wait(4)
	if err != nil {
		t.Fatal(err)
	}
	wantActions := []string{"SaveSnap", "Save", "Sync", "Release"}
	if len(actions) != len(wantActions) {
		t.Fatalf("storage actions = %v, want %v", actions, wantActions)
	}
	for i, want := range wantActions {
		if actions[i].Name != want {
			t.Fatalf("storage action %d = %q, want %q", i, actions[i].Name, want)
		}
	}
}

type asyncReadyNode struct {
	*readyNode
	stepc    chan *raftpb.Message
	advancec chan struct{}
}

func newAsyncReadyNode() *asyncReadyNode {
	return &asyncReadyNode{
		readyNode: newNopReadyNode(),
		stepc:     make(chan *raftpb.Message, 8),
		advancec:  make(chan struct{}, 1),
	}
}

func (n *asyncReadyNode) Step(_ context.Context, m *raftpb.Message) error {
	n.stepc <- m
	return nil
}

func (n *asyncReadyNode) Advance() {
	n.advancec <- struct{}{}
}

type blockingSaveStorage struct {
	serverstorage.Storage
	saveStartedC chan<- struct{}
	allowSaveC   <-chan struct{}
}

func (s *blockingSaveStorage) Save(st *raftpb.HardState, entries []*raftpb.Entry) error {
	s.saveStartedC <- struct{}{}
	<-s.allowSaveC
	return s.Storage.Save(st, entries)
}

type saveCaptureStorage struct {
	serverstorage.Storage
	saveCount int
	hardState *raftpb.HardState
	entries   []*raftpb.Entry
}

func (s *saveCaptureStorage) Save(st *raftpb.HardState, entries []*raftpb.Entry) error {
	s.saveCount++
	s.hardState = &raftpb.HardState{
		Term:   new(st.GetTerm()),
		Vote:   new(st.GetVote()),
		Commit: new(st.GetCommit()),
	}
	s.entries = append([]*raftpb.Entry(nil), entries...)
	return s.Storage.Save(st, entries)
}

func newStorageAppendMessage(index, term uint64) *raftpb.Message {
	entry := &raftpb.Entry{Index: new(index), Term: new(term)}
	return &raftpb.Message{
		Type:    raftpb.MsgStorageAppend.Enum(),
		To:      new(uint64(raft.LocalAppendThread)),
		From:    new(uint64(1)),
		Entries: []*raftpb.Entry{entry},
		Responses: []*raftpb.Message{{
			Type:    raftpb.MsgStorageAppendResp.Enum(),
			To:      new(uint64(1)),
			From:    new(uint64(raft.LocalAppendThread)),
			Index:   new(index),
			LogTerm: new(term),
		}},
	}
}

func newTestRaftReadyHandler() *raftReadyHandler {
	return &raftReadyHandler{
		getLead:              func() uint64 { return 0 },
		updateLead:           func(uint64) {},
		updateLeadership:     func(bool) {},
		updateCommittedIndex: func(uint64) {},
	}
}
