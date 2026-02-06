// Copyright 2025 The etcd Authors
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

package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3/raftpb"
)

func TestMergeMemberEntries(t *testing.T) {
	tcs := []struct {
		name           string
		minCommitIndex uint64
		memberEntries  [][]raftpb.Entry
		expectErr      string
		expectEntries  []raftpb.Entry
	}{
		{
			name:          "Error when empty data dir",
			memberEntries: [][]raftpb.Entry{},
			expectErr:     "no WAL entries matched",
		},
		{
			name: "Success when no entries",
			memberEntries: [][]raftpb.Entry{
				{},
			},
			expectErr: "no WAL entries matched",
		},
		{
			name: "Error when one member cluster didn't observe the index",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectErr: "no entry for raft index 2",
		},
		{
			name: "Error when entries index unordered",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 3, Data: []byte("c")},
					raftpb.Entry{Index: 1, Data: []byte("a")},
				},
			},
			expectErr: "raft index should increase, got: 1, previous: 3",
		},
		{
			name: "Error when entries index duplicated",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 1, Data: []byte("a")},
				},
			},
			expectErr: "raft index should increase, got: 1, previous: 1",
		},
		{
			name: "Success when one member cluster",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Success when three members agree on entries",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Success when three members have no entries",
			memberEntries: [][]raftpb.Entry{
				{}, {}, {},
			},
			expectErr: "no WAL entries matched",
		},
		{
			name: "Success when one member has no entries in three node cluster",
			memberEntries: [][]raftpb.Entry{
				{},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Success if two members have no entries in three node cluster",
			memberEntries: [][]raftpb.Entry{
				{},
				{},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Success if members didn't observe the whole history",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
				},
				{
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Success if members observed only one part of history",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
				},
				{
					raftpb.Entry{Index: 2, Data: []byte("b")},
				},
				{
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Error when in three member cluster if no members observed index",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectErr: "no entry for raft index 2",
		},
		{
			name: "Success if only one member observed history",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{},
				{},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Success when one member observed different last entry",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("x")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Data: []byte("a")},
				{Index: 2, Data: []byte("b")},
				{Index: 3, Data: []byte("c")},
			},
		},
		{
			name: "Error when one member didn't observe whole history and others observed different last entry",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("x")},
				},
			},
			expectErr: "mismatching entries on raft index 3",
		},
		{
			name: "Error when three members observed different last entry",
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("x")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("y")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Data: []byte("z")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectErr: "mismatching entries on raft index 2",
		},
		{
			name: "Error when one member observed empty history and others differ on last entry",
			memberEntries: [][]raftpb.Entry{
				{},
				{
					raftpb.Entry{Index: 1, Data: []byte("x")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
				{
					raftpb.Entry{Index: 1, Data: []byte("y")},
					raftpb.Entry{Index: 2, Data: []byte("b")},
					raftpb.Entry{Index: 3, Data: []byte("c")},
				},
			},
			expectErr: "mismatching entries on raft index 1",
		},
		{
			name:           "Error if entries mismatch on index before minCommitIndex",
			minCommitIndex: 2,
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Term: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Term: 1, Data: []byte("b")},
				},
				{
					raftpb.Entry{Index: 1, Term: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Term: 2, Data: []byte("c")},
				},
			},
			expectErr: "mismatching entries on raft index 2",
		},
		{
			name:           "Select entry with higher term if they conflict on uncommitted index",
			minCommitIndex: 1,
			memberEntries: [][]raftpb.Entry{
				{
					raftpb.Entry{Index: 1, Term: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Term: 1, Data: []byte("b")},
				},
				{
					raftpb.Entry{Index: 1, Term: 1, Data: []byte("a")},
					raftpb.Entry{Index: 2, Term: 2, Data: []byte("x")},
				},
			},
			expectEntries: []raftpb.Entry{
				{Index: 1, Term: 1, Data: []byte("a")},
				{Index: 2, Term: 2, Data: []byte("x")},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := mergeMembersEntries(tc.minCommitIndex, tc.memberEntries)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectErr)
			}
			require.Equal(t, tc.expectEntries, entries)
		})
	}
}

func TestWriteReadWAL(t *testing.T) {
	type batch struct {
		state    *raftpb.HardState
		entries  []raftpb.Entry
		snapshot *walpb.Snapshot
	}
	type want struct {
		wantState   raftpb.HardState
		wantEntries []raftpb.Entry
		wantError   string
	}

	tcs := []struct {
		name           string
		operations     []batch
		readAt         walpb.Snapshot
		walReadAll     want
		readAllEntries want
	}{
		{
			name: "single batch",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 5},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
				},
			},
			walReadAll: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
		},
		{
			name: "multiple committed batches",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 2},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
				},
				{
					state:   &raftpb.HardState{Commit: 4},
					entries: []raftpb.Entry{{Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}},
				},
				{
					state:   &raftpb.HardState{Commit: 5},
					entries: []raftpb.Entry{{Index: 5, Data: []byte("e")}},
				},
			},
			walReadAll: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
		},
		{
			name: "uncommitted ovewritten entries",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 1},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("a")}},
				},
				{
					state:   &raftpb.HardState{Commit: 3},
					entries: []raftpb.Entry{{Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("b")}, {Index: 4, Data: []byte("b")}},
				},
				{
					state:   &raftpb.HardState{Commit: 4},
					entries: []raftpb.Entry{{Index: 4, Data: []byte("c")}, {Index: 5, Data: []byte("c")}},
				},
			},
			walReadAll: want{
				wantState:   raftpb.HardState{Commit: 4},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("b")}, {Index: 4, Data: []byte("c")}, {Index: 5, Data: []byte("c")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 4},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("b")}, {Index: 4, Data: []byte("c")}, {Index: 5, Data: []byte("c")}},
			},
		},
		{
			name: "entries in bad order",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 2},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
				},
				{
					state:   &raftpb.HardState{Commit: 6},
					entries: []raftpb.Entry{{Index: 5, Data: []byte("e")}, {Index: 6, Data: []byte("f")}},
				},
				{
					state:   &raftpb.HardState{Commit: 4},
					entries: []raftpb.Entry{{Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}},
				},
			},
			walReadAll: want{
				wantError:   "slice bounds out of range",
				wantState:   raftpb.HardState{Commit: 2},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 4},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}},
			},
		},
		{
			name: "read before snapshot",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 1},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
				},
				{
					snapshot: &walpb.Snapshot{Index: new(uint64(3)), ConfState: &raftpb.ConfState{}},
				},
				{
					state:   &raftpb.HardState{Commit: 5},
					entries: []raftpb.Entry{{Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
				},
			},
			walReadAll: want{
				wantError:   "slice bounds out of range",
				wantState:   raftpb.HardState{Commit: 1},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
		},
		{
			name: "read at snapshot",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 1},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
				},
				{
					snapshot: &walpb.Snapshot{Index: new(uint64(3)), ConfState: &raftpb.ConfState{}},
				},
				{
					state:   &raftpb.HardState{Commit: 5},
					entries: []raftpb.Entry{{Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
				},
			},
			readAt: walpb.Snapshot{Index: new(uint64(3))},
			walReadAll: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
		},
		{
			name: "uncommitted entries before snapshot",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 1},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
				},
				{
					state:   &raftpb.HardState{Commit: 3},
					entries: []raftpb.Entry{{Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}},
				},
				{
					snapshot: &walpb.Snapshot{Index: new(uint64(3)), ConfState: &raftpb.ConfState{}},
				},
				{
					state:   &raftpb.HardState{Commit: 4},
					entries: []raftpb.Entry{{Index: 4, Data: []byte("e")}, {Index: 5, Data: []byte("f")}},
				},
			},
			walReadAll: want{
				wantState:   raftpb.HardState{Commit: 4},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("e")}, {Index: 5, Data: []byte("f")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 4},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("e")}, {Index: 5, Data: []byte("f")}},
			},
		},
		{
			name: "entries preceding snapshot",
			operations: []batch{
				{
					snapshot: &walpb.Snapshot{Index: new(uint64(4)), ConfState: &raftpb.ConfState{}},
				},
				{
					state:   &raftpb.HardState{Commit: 2},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
				},
				{
					state:   &raftpb.HardState{Commit: 4},
					entries: []raftpb.Entry{{Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}},
				},
				{
					state:   &raftpb.HardState{Commit: 6},
					entries: []raftpb.Entry{{Index: 5, Data: []byte("e")}, {Index: 6, Data: []byte("f")}},
				},
			},
			readAt: walpb.Snapshot{Index: new(uint64(4))},
			walReadAll: want{
				wantState:   raftpb.HardState{Commit: 6},
				wantEntries: []raftpb.Entry{{Index: 5, Data: []byte("e")}, {Index: 6, Data: []byte("f")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 6},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 3, Data: []byte("c")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}, {Index: 6, Data: []byte("f")}},
			},
		},
		{
			name: "read after snapshot",
			operations: []batch{
				{
					state:   &raftpb.HardState{Commit: 1},
					entries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}},
				},
				{
					snapshot: &walpb.Snapshot{Index: new(uint64(3)), ConfState: &raftpb.ConfState{}},
				},
				{
					state:   &raftpb.HardState{Commit: 5},
					entries: []raftpb.Entry{{Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
				},
			},
			readAt: walpb.Snapshot{Index: new(uint64(4))},
			walReadAll: want{
				wantError:   "snapshot not found",
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 5, Data: []byte("e")}},
			},
			readAllEntries: want{
				wantState:   raftpb.HardState{Commit: 5},
				wantEntries: []raftpb.Entry{{Index: 1, Data: []byte("a")}, {Index: 2, Data: []byte("b")}, {Index: 4, Data: []byte("d")}, {Index: 5, Data: []byte("e")}},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			lg := zaptest.NewLogger(t)
			w, err := wal.Create(lg, dir, nil)
			require.NoError(t, err)
			for _, op := range tc.operations {
				if op.state != nil {
					err = w.Save(*op.state, op.entries)
					require.NoError(t, err)
				}
				if op.snapshot != nil {
					err = w.SaveSnapshot(*op.snapshot)
					require.NoError(t, err)
				}
			}
			w.Close()

			w2, err := wal.OpenForRead(lg, dir, tc.readAt)
			require.NoError(t, err)
			defer w2.Close()
			t.Run("wal.ReadAll", func(t *testing.T) {
				_, state, entries, err := w2.ReadAll()
				if tc.walReadAll.wantError != "" {
					require.ErrorContains(t, err, tc.walReadAll.wantError)
				} else {
					require.NoError(t, err)
				}
				assert.Equal(t, tc.walReadAll.wantState, state)
				assert.Equal(t, tc.walReadAll.wantEntries, entries)
			})
			t.Run("ReadAllEntries", func(t *testing.T) {
				state, entries, err := ReadAllWALEntries(lg, dir)
				if tc.readAllEntries.wantError != "" {
					require.ErrorContains(t, err, tc.walReadAll.wantError)
				} else {
					require.NoError(t, err)
				}
				assert.Equal(t, tc.readAllEntries.wantState, state)
				assert.Equal(t, tc.readAllEntries.wantEntries, entries)
			})
		})
	}
}
