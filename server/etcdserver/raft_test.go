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

func TestProcessMessagesKeepsReadyOrder(t *testing.T) {
	r := newRaftNode(raftNodeConfig{
		lg:          zaptest.NewLogger(t),
		isIDRemoved: func(id uint64) bool { return id == 5 },
		Node:        newNopReadyNode(),
		storage:     mockstorage.NewStorageRecorder(""),
		raftStorage: raft.NewMemoryStorage(),
		transport:   newNopTransporter(),
	})

	const (
		lead  = uint64(1)
		peer2 = uint64(2)
		peer3 = uint64(3)
		peer5 = uint64(5)
	)
	app := func(to, index uint64) *raftpb.Message {
		return &raftpb.Message{
			Type:  raftpb.MsgApp.Enum(),
			From:  new(lead),
			To:    new(to),
			Term:  new(uint64(1)),
			Index: new(index),
		}
	}
	resp := func(index uint64) *raftpb.Message {
		return &raftpb.Message{
			Type:  raftpb.MsgAppResp.Enum(),
			From:  new(peer2),
			To:    new(lead),
			Term:  new(uint64(1)),
			Index: new(index),
		}
	}
	type msgFields struct {
		typ   raftpb.MessageType
		to    uint64
		index uint64
	}
	snapshot := func(ms []*raftpb.Message) []msgFields {
		out := make([]msgFields, len(ms))
		for i, m := range ms {
			out[i] = msgFields{typ: m.GetType(), to: m.GetTo(), index: m.GetIndex()}
		}
		return out
	}
	unchanged := func(t *testing.T, ms []*raftpb.Message, before []msgFields) {
		t.Helper()
		if len(ms) != len(before) {
			t.Fatalf("input len=%d, want %d", len(ms), len(before))
		}
		for i, m := range ms {
			if m.GetType() != before[i].typ || m.GetTo() != before[i].to || m.GetIndex() != before[i].index {
				t.Fatalf("input %d mutated: type=%s to=%d index=%d, was type=%s to=%d index=%d",
					i, m.GetType(), m.GetTo(), m.GetIndex(), before[i].typ, before[i].to, before[i].index)
			}
		}
	}

	t.Run("MsgApp", func(t *testing.T) {
		ms := []*raftpb.Message{
			app(peer2, 10),
			app(peer2, 11),
			app(peer2, 12),
			app(peer3, 10),
			app(peer5, 10),
		}
		before := snapshot(ms)
		got := r.processMessages(ms)
		want := []*raftpb.Message{ms[0], ms[1], ms[2], ms[3]}
		if len(got) != len(want) {
			t.Fatalf("len=%d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got[%d] is not the Ready pointer", i)
			}
		}
		unchanged(t, ms, before)
	})

	t.Run("MsgAppResp", func(t *testing.T) {
		ms := []*raftpb.Message{
			resp(1),
			app(peer2, 10),
			resp(2),
			resp(3),
		}
		before := snapshot(ms)
		got := r.processMessages(ms)
		want := []*raftpb.Message{ms[1], ms[3]}
		if len(got) != len(want) {
			t.Fatalf("len=%d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got[%d] is not the Ready pointer", i)
			}
		}
		unchanged(t, ms, before)
	})

	t.Run("mixed", func(t *testing.T) {
		ms := []*raftpb.Message{
			app(peer2, 10),
			app(peer2, 11),
			app(peer2, 12),
			resp(1),
			resp(2),
			app(peer5, 10),
		}
		before := snapshot(ms)
		got := r.processMessages(ms)
		want := []*raftpb.Message{ms[0], ms[1], ms[2], ms[4]}
		if len(got) != len(want) {
			t.Fatalf("len=%d, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got[%d] is not the Ready pointer", i)
			}
		}
		unchanged(t, ms, before)
	})
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
