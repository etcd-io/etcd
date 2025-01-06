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

package etcdutl

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/raft/v3/raftpb"
)

func TestGetLatestWalSnap(t *testing.T) {
	testCases := []struct {
		name                  string
		walSnaps              []walpb.Snapshot
		snapshots             []raftpb.Snapshot
		expectedLatestWALSnap walpb.Snapshot
	}{
		{
			name: "wal snapshot records match the snapshot files",
			walSnaps: []walpb.Snapshot{
				{Index: 10, Term: 2},
				{Index: 20, Term: 3},
				{Index: 30, Term: 5},
			},
			snapshots: []raftpb.Snapshot{
				{Metadata: raftpb.SnapshotMetadata{Index: 10, Term: 2}},
				{Metadata: raftpb.SnapshotMetadata{Index: 20, Term: 3}},
				{Metadata: raftpb.SnapshotMetadata{Index: 30, Term: 5}},
			},
			expectedLatestWALSnap: walpb.Snapshot{Index: 30, Term: 5},
		},
		{
			name: "there are orphan snapshot files",
			walSnaps: []walpb.Snapshot{
				{Index: 10, Term: 2},
				{Index: 20, Term: 3},
				{Index: 35, Term: 5},
			},
			snapshots: []raftpb.Snapshot{
				{Metadata: raftpb.SnapshotMetadata{Index: 10, Term: 2}},
				{Metadata: raftpb.SnapshotMetadata{Index: 20, Term: 3}},
				{Metadata: raftpb.SnapshotMetadata{Index: 35, Term: 5}},
				{Metadata: raftpb.SnapshotMetadata{Index: 40, Term: 6}},
				{Metadata: raftpb.SnapshotMetadata{Index: 50, Term: 7}},
			},
			expectedLatestWALSnap: walpb.Snapshot{Index: 35, Term: 5},
		},
		{
			name: "there are orphan snapshot records in wal file",
			walSnaps: []walpb.Snapshot{
				{Index: 10, Term: 2},
				{Index: 20, Term: 3},
				{Index: 30, Term: 4},
				{Index: 45, Term: 5},
				{Index: 55, Term: 6},
			},
			snapshots: []raftpb.Snapshot{
				{Metadata: raftpb.SnapshotMetadata{Index: 10, Term: 2}},
				{Metadata: raftpb.SnapshotMetadata{Index: 20, Term: 3}},
				{Metadata: raftpb.SnapshotMetadata{Index: 30, Term: 4}},
			},
			expectedLatestWALSnap: walpb.Snapshot{Index: 30, Term: 4},
		},
		{
			name: "wal snapshot records do not match the snapshot files at all",
			walSnaps: []walpb.Snapshot{
				{Index: 10, Term: 2},
				{Index: 20, Term: 3},
				{Index: 30, Term: 4},
			},
			snapshots: []raftpb.Snapshot{
				{Metadata: raftpb.SnapshotMetadata{Index: 40, Term: 5}},
				{Metadata: raftpb.SnapshotMetadata{Index: 50, Term: 6}},
			},
			expectedLatestWALSnap: walpb.Snapshot{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			lg := zap.NewNop()

			require.NoError(t, fileutil.TouchDirAll(lg, datadir.ToMemberDir(dataDir)))
			require.NoError(t, fileutil.TouchDirAll(lg, datadir.ToWALDir(dataDir)))
			require.NoError(t, fileutil.TouchDirAll(lg, datadir.ToSnapDir(dataDir)))

			// populate wal file
			w, err := wal.Create(lg, datadir.ToWALDir(dataDir), pbutil.MustMarshal(
				&etcdserverpb.Metadata{
					NodeID:    1,
					ClusterID: 2,
				},
			))
			require.NoError(t, err)

			for _, walSnap := range tc.walSnaps {
				walSnap.ConfState = &raftpb.ConfState{Voters: []uint64{1}}
				walErr := w.SaveSnapshot(walSnap)
				require.NoError(t, walErr)
				walErr = w.Save(raftpb.HardState{Term: walSnap.Term, Commit: walSnap.Index, Vote: 1}, nil)
				require.NoError(t, walErr)
			}
			err = w.Close()
			require.NoError(t, err)

			// generate snapshot files
			ss := snap.New(lg, datadir.ToSnapDir(dataDir))
			for _, snap := range tc.snapshots {
				snap.Metadata.ConfState = raftpb.ConfState{Voters: []uint64{1}}
				snapErr := ss.SaveSnap(snap)
				require.NoError(t, snapErr)
			}

			walSnap, err := getLatestWALSnap(lg, dataDir)
			require.NoError(t, err)

			require.Equal(t, tc.expectedLatestWALSnap, walSnap)
		})
	}
}
