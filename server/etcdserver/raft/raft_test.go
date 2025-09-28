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
	"context"
	"encoding/json"
	"expvar"
	"net/http"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/mock/mockstorage"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

func TestGetIDs(t *testing.T) {
	lg := zaptest.NewLogger(t)
	addcc := &raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 2}
	addEntry := raftpb.Entry{Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(addcc)}
	removecc := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode, NodeID: 2}
	removeEntry := raftpb.Entry{Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc)}
	normalEntry := raftpb.Entry{Type: raftpb.EntryNormal}
	updatecc := &raftpb.ConfChange{Type: raftpb.ConfChangeUpdateNode, NodeID: 2}
	updateEntry := raftpb.Entry{Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(updatecc)}

	tests := []struct {
		confState *raftpb.ConfState
		ents      []raftpb.Entry

		widSet []uint64
	}{
		{nil, []raftpb.Entry{}, []uint64{}},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]raftpb.Entry{},
			[]uint64{1},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]raftpb.Entry{addEntry},
			[]uint64{1, 2},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]raftpb.Entry{addEntry, removeEntry},
			[]uint64{1},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]raftpb.Entry{addEntry, normalEntry},
			[]uint64{1, 2},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]raftpb.Entry{addEntry, normalEntry, updateEntry},
			[]uint64{1, 2},
		},
		{
			&raftpb.ConfState{Voters: []uint64{1}},
			[]raftpb.Entry{addEntry, removeEntry, normalEntry},
			[]uint64{1},
		},
	}

	for i, tt := range tests {
		var snap raftpb.Snapshot
		if tt.confState != nil {
			snap.Metadata.ConfState = *tt.confState
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
	addcc1 := &raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 1, Context: ctx}
	removecc2 := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode, NodeID: 2}
	removecc3 := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode, NodeID: 3}
	tests := []struct {
		ids         []uint64
		self        uint64
		term, index uint64

		wents []raftpb.Entry
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

			[]raftpb.Entry{{Term: 1, Index: 2, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc2)}},
		},
		{
			[]uint64{1, 2},
			1,
			2, 2,

			[]raftpb.Entry{{Term: 2, Index: 3, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc2)}},
		},
		{
			[]uint64{1, 2, 3},
			1,
			2, 2,

			[]raftpb.Entry{
				{Term: 2, Index: 3, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc2)},
				{Term: 2, Index: 4, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc3)},
			},
		},
		{
			[]uint64{2, 3},
			2,
			2, 2,

			[]raftpb.Entry{
				{Term: 2, Index: 3, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc3)},
			},
		},
		{
			[]uint64{2, 3},
			1,
			2, 2,

			[]raftpb.Entry{
				{Term: 2, Index: 3, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(addcc1)},
				{Term: 2, Index: 4, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc2)},
				{Term: 2, Index: 5, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc3)},
			},
		},
	}

	for i, tt := range tests {
		gents := serverstorage.CreateConfigChangeEnts(lg, tt.ids, tt.self, tt.term, tt.index)
		if !reflect.DeepEqual(gents, tt.wents) {
			t.Errorf("#%d: ents = %v, want %v", i, gents, tt.wents)
		}
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
	r := NewNodeFromConfig(Config{
		Lg:          zaptest.NewLogger(t),
		Node:        n,
		Storage:     mockstorage.NewStorageRecorder(""),
		RaftStorage: raft.NewMemoryStorage(),
		Transport:   newNopTransporter(),
	})
	r.Start(&ReadyHandler{})

	for i := 0; i < 2; i++ {
		stopped := make(chan struct{})
		go func() {
			r.Stop()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Errorf("*raftNode.stop() is blocked !")
		}
	}
}

type nodeRecorder struct{ testutil.Recorder }

func newNodeRecorder() *nodeRecorder { return &nodeRecorder{&testutil.RecorderBuffered{}} }

func (n *nodeRecorder) Tick() { n.Record(testutil.Action{Name: "Tick"}) }
func (n *nodeRecorder) Campaign(ctx context.Context) error {
	n.Record(testutil.Action{Name: "Campaign"})
	return nil
}

func (n *nodeRecorder) Propose(ctx context.Context, data []byte) error {
	n.Record(testutil.Action{Name: "Propose", Params: []any{data}})
	return nil
}

func (n *nodeRecorder) ProposeConfChange(ctx context.Context, conf raftpb.ConfChangeI) error {
	n.Record(testutil.Action{Name: "ProposeConfChange"})
	return nil
}

func (n *nodeRecorder) Step(ctx context.Context, msg raftpb.Message) error {
	n.Record(testutil.Action{Name: "Step"})
	return nil
}
func (n *nodeRecorder) Status() raft.Status                                             { return raft.Status{} }
func (n *nodeRecorder) Ready() <-chan raft.Ready                                        { return nil }
func (n *nodeRecorder) TransferLeadership(ctx context.Context, lead, transferee uint64) {}
func (n *nodeRecorder) ReadIndex(ctx context.Context, rctx []byte) error                { return nil }
func (n *nodeRecorder) Advance()                                                        {}
func (n *nodeRecorder) ApplyConfChange(conf raftpb.ConfChangeI) *raftpb.ConfState {
	n.Record(testutil.Action{Name: "ApplyConfChange", Params: []any{conf}})
	return &raftpb.ConfState{}
}

func (n *nodeRecorder) Stop() {
	n.Record(testutil.Action{Name: "Stop"})
}

func (n *nodeRecorder) ReportUnreachable(id uint64) {}

func (n *nodeRecorder) ReportSnapshot(id uint64, status raft.SnapshotStatus) {}

func (n *nodeRecorder) Compact(index uint64, nodes []uint64, d []byte) {
	n.Record(testutil.Action{Name: "Compact"})
}

func (n *nodeRecorder) ForgetLeader(ctx context.Context) error {
	return nil
}

// readyNode is a nodeRecorder with a user-writeable ready channel
type readyNode struct {
	nodeRecorder
	readyc chan raft.Ready
}

func newNopReadyNode() *readyNode {
	return &readyNode{*newNodeRecorder(), make(chan raft.Ready, 1)}
}

func (n *readyNode) Ready() <-chan raft.Ready { return n.readyc }

type nopTransporter struct{}

func newNopTransporter() rafthttp.Transporter {
	return &nopTransporter{}
}

func (s *nopTransporter) Start() error                        { return nil }
func (s *nopTransporter) Handler() http.Handler               { return nil }
func (s *nopTransporter) Send(m []raftpb.Message)             {}
func (s *nopTransporter) SendSnapshot(m snap.Message)         {}
func (s *nopTransporter) AddRemote(id types.ID, us []string)  {}
func (s *nopTransporter) AddPeer(id types.ID, us []string)    {}
func (s *nopTransporter) RemovePeer(id types.ID)              {}
func (s *nopTransporter) RemoveAllPeers()                     {}
func (s *nopTransporter) UpdatePeer(id types.ID, us []string) {}
func (s *nopTransporter) ActiveSince(id types.ID) time.Time   { return time.Time{} }
func (s *nopTransporter) ActivePeers() int                    { return 0 }
func (s *nopTransporter) Stop()                               {}
func (s *nopTransporter) Pause()                              {}
func (s *nopTransporter) Resume()                             {}
