/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdserver

import (
	"reflect"
	"testing"

	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/raft/raftpb"
)

func TestGetIDset(t *testing.T) {
	addcc := &raftpb.ConfChange{Type: raftpb.ConfChangeAddNode, NodeID: 2}
	addEntry := raftpb.Entry{Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(addcc)}
	removecc := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode, NodeID: 2}
	removeEntry := raftpb.Entry{Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc)}
	normalEntry := raftpb.Entry{Type: raftpb.EntryNormal}

	tests := []struct {
		snap *raftpb.Snapshot
		ents []raftpb.Entry

		widSet map[uint64]bool
	}{
		{nil, []raftpb.Entry{}, map[uint64]bool{}},
		{&raftpb.Snapshot{Nodes: []uint64{1}}, []raftpb.Entry{}, map[uint64]bool{1: true}},
		{&raftpb.Snapshot{Nodes: []uint64{1}}, []raftpb.Entry{addEntry}, map[uint64]bool{1: true, 2: true}},
		{&raftpb.Snapshot{Nodes: []uint64{1}}, []raftpb.Entry{addEntry, removeEntry}, map[uint64]bool{1: true}},
		{&raftpb.Snapshot{Nodes: []uint64{1}}, []raftpb.Entry{addEntry, normalEntry}, map[uint64]bool{1: true, 2: true}},
		{&raftpb.Snapshot{Nodes: []uint64{1}}, []raftpb.Entry{addEntry, removeEntry, normalEntry}, map[uint64]bool{1: true}},
	}

	for i, tt := range tests {
		idSet := getIDset(tt.snap, tt.ents)
		if !reflect.DeepEqual(idSet, tt.widSet) {
			t.Errorf("#%d: idset = %v, want %v", i, idSet, tt.widSet)
		}
	}
}

func TestCreateConfigChangeEnts(t *testing.T) {
	removecc2 := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode, NodeID: 2}
	removecc3 := &raftpb.ConfChange{Type: raftpb.ConfChangeRemoveNode, NodeID: 3}
	tests := []struct {
		ids         map[uint64]bool
		self        uint64
		term, index uint64

		wents []raftpb.Entry
	}{
		{
			map[uint64]bool{1: true},
			1,
			1, 1,

			[]raftpb.Entry{},
		},
		{
			map[uint64]bool{1: true, 2: true},
			1,
			1, 1,

			[]raftpb.Entry{{Term: 1, Index: 2, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc2)}},
		},
		{
			map[uint64]bool{1: true, 2: true},
			1,
			2, 2,

			[]raftpb.Entry{{Term: 2, Index: 3, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc2)}},
		},
		{
			map[uint64]bool{1: true, 2: true, 3: true},
			1,
			2, 2,

			[]raftpb.Entry{
				{Term: 2, Index: 3, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc2)},
				{Term: 2, Index: 4, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc3)},
			},
		},
		{
			map[uint64]bool{2: true, 3: true},
			2,
			2, 2,

			[]raftpb.Entry{
				{Term: 2, Index: 3, Type: raftpb.EntryConfChange, Data: pbutil.MustMarshal(removecc3)},
			},
		},
	}

	for i, tt := range tests {
		gents := createConfigChangeEnts(tt.ids, tt.self, tt.term, tt.index)
		if !reflect.DeepEqual(gents, tt.wents) {
			t.Errorf("#%d: ents = %v, want %v", i, gents, tt.wents)
		}
	}
}
