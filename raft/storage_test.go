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
	"reflect"
	"testing"

	pb "github.com/coreos/etcd/raft/raftpb"
)

// TODO(xiangli): Test panic cases

func TestStorageTerm(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tests := []struct {
		i uint64

		werr  error
		wterm uint64
	}{
		{2, ErrCompacted, 0},
		{3, nil, 3},
		{4, nil, 4},
		{5, nil, 5},
	}

	for i, tt := range tests {
		s := &MemoryStorage{ents: ents}
		term, err := s.Term(tt.i)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if term != tt.wterm {
			t.Errorf("#%d: term = %d, want %d", i, term, tt.wterm)
		}
	}
}

func TestStorageEntries(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tests := []struct {
		lo, hi uint64

		werr     error
		wentries []pb.Entry
	}{
		{2, 6, ErrCompacted, nil},
		{3, 4, ErrCompacted, nil},
		{4, 5, nil, []pb.Entry{{Index: 4, Term: 4}}},
		{4, 6, nil, []pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 5}}},
	}

	for i, tt := range tests {
		s := &MemoryStorage{ents: ents}
		entries, err := s.Entries(tt.lo, tt.hi)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if !reflect.DeepEqual(entries, tt.wentries) {
			t.Errorf("#%d: entries = %v, want %v", i, entries, tt.wentries)
		}
	}
}

func TestStorageLastIndex(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	s := &MemoryStorage{ents: ents}

	last, err := s.LastIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if last != 5 {
		t.Errorf("term = %d, want %d", last, 5)
	}

	s.Append([]pb.Entry{{Index: 6, Term: 5}})
	last, err = s.LastIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if last != 6 {
		t.Errorf("last = %d, want %d", last, 5)
	}
}

func TestStorageFirstIndex(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	s := &MemoryStorage{ents: ents}

	first, err := s.FirstIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if first != 4 {
		t.Errorf("first = %d, want %d", first, 4)
	}

	s.Compact(4)
	first, err = s.FirstIndex()
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if first != 5 {
		t.Errorf("first = %d, want %d", first, 5)
	}
}

func TestStorageCompact(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tests := []struct {
		i uint64

		werr   error
		windex uint64
		wterm  uint64
		wlen   int
	}{
		{2, ErrCompacted, 3, 3, 3},
		{3, ErrCompacted, 3, 3, 3},
		{4, nil, 4, 4, 2},
		{5, nil, 5, 5, 1},
	}

	for i, tt := range tests {
		s := &MemoryStorage{ents: ents}
		err := s.Compact(tt.i)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if s.ents[0].Index != tt.windex {
			t.Errorf("#%d: index = %d, want %d", i, s.ents[0].Index, tt.windex)
		}
		if s.ents[0].Term != tt.wterm {
			t.Errorf("#%d: term = %d, want %d", i, s.ents[0].Term, tt.wterm)
		}
		if len(s.ents) != tt.wlen {
			t.Errorf("#%d: len = %d, want %d", i, len(s.ents), tt.wlen)
		}
	}
}

func TestStorageCreateSnapshot(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	cs := &pb.ConfState{Nodes: []uint64{1, 2, 3}}
	data := []byte("data")

	tests := []struct {
		i uint64

		werr  error
		wsnap pb.Snapshot
	}{
		{4, nil, pb.Snapshot{Data: data, Metadata: pb.SnapshotMetadata{Index: 4, Term: 4, ConfState: *cs}}},
		{5, nil, pb.Snapshot{Data: data, Metadata: pb.SnapshotMetadata{Index: 5, Term: 5, ConfState: *cs}}},
	}

	for i, tt := range tests {
		s := &MemoryStorage{ents: ents}
		snap, err := s.CreateSnapshot(tt.i, cs, data)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if !reflect.DeepEqual(snap, tt.wsnap) {
			t.Errorf("#%d: snap = %+v, want %+v", i, snap, tt.wsnap)
		}
	}
}

func TestStorageAppend(t *testing.T) {
	ents := []pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}}
	tests := []struct {
		entries []pb.Entry

		werr     error
		wentries []pb.Entry
	}{
		{
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}},
		},
		{
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 6}, {Index: 5, Term: 6}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 6}, {Index: 5, Term: 6}},
		},
		{
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
		},
		{
			[]pb.Entry{{Index: 6, Term: 5}},
			nil,
			[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 5}, {Index: 6, Term: 5}},
		},
	}

	for i, tt := range tests {
		s := &MemoryStorage{ents: ents}
		err := s.Append(tt.entries)
		if err != tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, err, tt.werr)
		}
		if !reflect.DeepEqual(s.ents, tt.wentries) {
			t.Errorf("#%d: entries = %v, want %v", i, s.ents, tt.wentries)
		}
	}
}
