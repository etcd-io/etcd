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

	"github.com/stretchr/testify/require"

	"go.etcd.io/raft/v3/raftpb"
)

func TestMergeMemberEntries(t *testing.T) {
	tcs := []struct {
		name          string
		memberEntries [][]raftpb.Entry
		expectErr     string
		expectEntries []raftpb.Entry
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
			name: "Error when one member cluster didn't observed index",
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
			name: "Success if members didn't observe whole history",
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
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := mergeMembersEntries(tc.memberEntries)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectErr)
			}
			require.Equal(t, tc.expectEntries, entries)
		})
	}
}
