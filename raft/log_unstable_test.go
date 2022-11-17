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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	pb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestUnstableMaybeFirstIndex(t *testing.T) {
	tests := []struct {
		entries []pb.Entry
		offset  uint64
		snap    *pb.Snapshot

		wok    bool
		windex uint64
	}{
		// no snapshot
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			false, 0,
		},
		{
			[]pb.Entry{}, 0, nil,
			false, 0,
		},
		// has snapshot
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			true, 5,
		},
		{
			[]pb.Entry{}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			true, 5,
		},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			u := unstable{
				entries:  tt.entries,
				offset:   tt.offset,
				snapshot: tt.snap,
				logger:   raftLogger,
			}
			index, ok := u.maybeFirstIndex()
			require.Equal(t, tt.wok, ok)
			require.Equal(t, tt.windex, index)
		})
	}
}

func TestMaybeLastIndex(t *testing.T) {
	tests := []struct {
		entries []pb.Entry
		offset  uint64
		snap    *pb.Snapshot

		wok    bool
		windex uint64
	}{
		// last in entries
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			true, 5,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			true, 5,
		},
		// last in snapshot
		{
			[]pb.Entry{}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			true, 4,
		},
		// empty unstable
		{
			[]pb.Entry{}, 0, nil,
			false, 0,
		},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			u := unstable{
				entries:  tt.entries,
				offset:   tt.offset,
				snapshot: tt.snap,
				logger:   raftLogger,
			}
			index, ok := u.maybeLastIndex()
			require.Equal(t, tt.wok, ok)
			require.Equal(t, tt.windex, index)
		})
	}
}

func TestUnstableMaybeTerm(t *testing.T) {
	tests := []struct {
		entries []pb.Entry
		offset  uint64
		snap    *pb.Snapshot
		index   uint64

		wok   bool
		wterm uint64
	}{
		// term from entries
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			5,
			true, 1,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			6,
			false, 0,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			4,
			false, 0,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			true, 1,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			6,
			false, 0,
		},
		// term from snapshot
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			4,
			true, 1,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			3,
			false, 0,
		},
		{
			[]pb.Entry{}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			5,
			false, 0,
		},
		{
			[]pb.Entry{}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			4,
			true, 1,
		},
		{
			[]pb.Entry{}, 0, nil,
			5,
			false, 0,
		},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			u := unstable{
				entries:  tt.entries,
				offset:   tt.offset,
				snapshot: tt.snap,
				logger:   raftLogger,
			}
			term, ok := u.maybeTerm(tt.index)
			require.Equal(t, tt.wok, ok)
			require.Equal(t, tt.wterm, term)
		})
	}
}

func TestUnstableRestore(t *testing.T) {
	u := unstable{
		entries:  []pb.Entry{{Index: 5, Term: 1}},
		offset:   5,
		snapshot: &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
		logger:   raftLogger,
	}
	s := pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 6, Term: 2}}
	u.restore(s)

	require.Equal(t, s.Metadata.Index+1, u.offset)
	require.Zero(t, len(u.entries))
	require.Equal(t, &s, u.snapshot)
}

func TestUnstableStableTo(t *testing.T) {
	tests := []struct {
		entries     []pb.Entry
		offset      uint64
		snap        *pb.Snapshot
		index, term uint64

		woffset uint64
		wlen    int
	}{
		{
			[]pb.Entry{}, 0, nil,
			5, 1,
			0, 0,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			5, 1, // stable to the first entry
			6, 0,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}}, 5, nil,
			5, 1, // stable to the first entry
			6, 1,
		},
		{
			[]pb.Entry{{Index: 6, Term: 2}}, 6, nil,
			6, 1, // stable to the first entry and term mismatch
			6, 1,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			4, 1, // stable to old entry
			5, 1,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			4, 2, // stable to old entry
			5, 1,
		},
		// with snapshot
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			5, 1, // stable to the first entry
			6, 0,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			5, 1, // stable to the first entry
			6, 1,
		},
		{
			[]pb.Entry{{Index: 6, Term: 2}}, 6, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 5, Term: 1}},
			6, 1, // stable to the first entry and term mismatch
			6, 1,
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 1}},
			4, 1, // stable to snapshot
			5, 1,
		},
		{
			[]pb.Entry{{Index: 5, Term: 2}}, 5, &pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: 4, Term: 2}},
			4, 1, // stable to old entry
			5, 1,
		},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			u := unstable{
				entries:  tt.entries,
				offset:   tt.offset,
				snapshot: tt.snap,
				logger:   raftLogger,
			}
			u.stableTo(tt.index, tt.term)
			require.Equal(t, tt.woffset, u.offset)
			require.Equal(t, tt.wlen, len(u.entries))
		})
	}
}

func TestUnstableTruncateAndAppend(t *testing.T) {
	tests := []struct {
		entries  []pb.Entry
		offset   uint64
		snap     *pb.Snapshot
		toappend []pb.Entry

		woffset  uint64
		wentries []pb.Entry
	}{
		// append to the end
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			[]pb.Entry{{Index: 6, Term: 1}, {Index: 7, Term: 1}},
			5, []pb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 1}},
		},
		// replace the unstable entries
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			[]pb.Entry{{Index: 5, Term: 2}, {Index: 6, Term: 2}},
			5, []pb.Entry{{Index: 5, Term: 2}, {Index: 6, Term: 2}},
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}}, 5, nil,
			[]pb.Entry{{Index: 4, Term: 2}, {Index: 5, Term: 2}, {Index: 6, Term: 2}},
			4, []pb.Entry{{Index: 4, Term: 2}, {Index: 5, Term: 2}, {Index: 6, Term: 2}},
		},
		// truncate the existing entries and append
		{
			[]pb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 1}}, 5, nil,
			[]pb.Entry{{Index: 6, Term: 2}},
			5, []pb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 2}},
		},
		{
			[]pb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 1}}, 5, nil,
			[]pb.Entry{{Index: 7, Term: 2}, {Index: 8, Term: 2}},
			5, []pb.Entry{{Index: 5, Term: 1}, {Index: 6, Term: 1}, {Index: 7, Term: 2}, {Index: 8, Term: 2}},
		},
	}

	for i, tt := range tests {
		tt := tt
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			u := unstable{
				entries:  tt.entries,
				offset:   tt.offset,
				snapshot: tt.snap,
				logger:   raftLogger,
			}
			u.truncateAndAppend(tt.toappend)
			require.Equal(t, tt.woffset, u.offset)
			require.Equal(t, tt.wentries, u.entries)
		})
	}
}
