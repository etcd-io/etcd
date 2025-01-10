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
	"path"
	"path/filepath"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
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

func TestCreateV2SnapshotFromV3Store(t *testing.T) {
	testCases := []struct {
		name               string
		initialIndex       uint64
		initialTerm        uint64
		initialCommitIndex uint64
		consistentIndex    uint64
		term               uint64
		clusterVersion     string
		members            []uint64
		learners           []uint64
		removedMembers     []uint64
		expectedErrMsg     string
	}{
		{
			name:               "unexpected term: less than the last snapshot.term",
			initialTerm:        4,
			initialIndex:       2,
			initialCommitIndex: 2,
			consistentIndex:    10,
			term:               2,
			expectedErrMsg:     "less than the latest snapshot",
		},
		{
			name:               "unexpected consistent index: less than the last snapshot.index",
			initialTerm:        2,
			initialIndex:       20,
			initialCommitIndex: 20,
			consistentIndex:    10,
			term:               3,
			expectedErrMsg:     "less than the latest snapshot",
		},
		{
			name:               "normal case with a smaller initial commit index",
			initialTerm:        3,
			initialIndex:       4,
			initialCommitIndex: 20,
			consistentIndex:    32,
			term:               4,
			clusterVersion:     "3.5.0",
			members:            []uint64{100, 200},
			learners:           []uint64{300},
			removedMembers:     []uint64{400, 500},
		},
		{
			name:               "normal case with a bigger initial commit index",
			initialTerm:        3,
			initialIndex:       4,
			initialCommitIndex: 40,
			consistentIndex:    32,
			term:               4,
			clusterVersion:     "3.5.0",
			members:            []uint64{100, 200},
			learners:           []uint64{300},
			removedMembers:     []uint64{400, 500},
		},
		{
			name:               "empty cluster version",
			initialTerm:        3,
			initialIndex:       4,
			initialCommitIndex: 20,
			consistentIndex:    45,
			term:               4,
			clusterVersion:     "",
			members:            []uint64{110, 200},
			learners:           []uint64{350},
			removedMembers:     []uint64{450, 500},
		},
		{
			name:               "no learner",
			initialTerm:        3,
			initialIndex:       4,
			initialCommitIndex: 8,
			consistentIndex:    7,
			term:               5,
			clusterVersion:     "3.5.0",
			members:            []uint64{150, 200},
			removedMembers:     []uint64{450, 550},
		},
		{
			name:               "no removed members",
			initialTerm:        6,
			initialIndex:       40,
			initialCommitIndex: 40,
			consistentIndex:    41,
			term:               6,
			clusterVersion:     "3.7.0",
			members:            []uint64{160, 200},
			learners:           []uint64{300},
		},
		{
			name:               "no learner and removed members",
			initialTerm:        6,
			initialIndex:       18,
			initialCommitIndex: 40,
			consistentIndex:    19,
			term:               6,
			clusterVersion:     "3.6.0",
			members:            []uint64{120, 220},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			lg := zap.NewNop()

			require.NoError(t, fileutil.TouchDirAll(lg, datadir.ToMemberDir(dataDir)))
			require.NoError(t, fileutil.TouchDirAll(lg, datadir.ToWALDir(dataDir)))
			require.NoError(t, fileutil.TouchDirAll(lg, datadir.ToSnapDir(dataDir)))

			// generate the initial state for wal and v2 snapshot,
			t.Log("Populate the wal file")
			w, err := wal.Create(lg, datadir.ToWALDir(dataDir), pbutil.MustMarshal(
				&etcdserverpb.Metadata{
					NodeID:    1,
					ClusterID: 2,
				},
			))
			require.NoError(t, err)
			err = w.SaveSnapshot(walpb.Snapshot{Index: tc.initialIndex, Term: tc.initialTerm, ConfState: &raftpb.ConfState{Voters: []uint64{1}}})
			require.NoError(t, err)
			err = w.Save(raftpb.HardState{Term: tc.initialTerm, Commit: tc.initialCommitIndex, Vote: 1}, nil)
			require.NoError(t, err)
			err = w.Close()
			require.NoError(t, err)

			t.Log("Generate a v2 snapshot file")
			ss := snap.New(lg, datadir.ToSnapDir(dataDir))
			err = ss.SaveSnap(raftpb.Snapshot{Metadata: raftpb.SnapshotMetadata{Index: tc.initialIndex, Term: tc.initialTerm, ConfState: raftpb.ConfState{Voters: []uint64{1}}}})
			require.NoError(t, err)

			t.Log("Load and verify the latest v2 snapshot file")
			oldV2Snap, err := getLatestV2Snapshot(lg, dataDir)
			require.NoError(t, err)
			require.Equal(t, raftpb.SnapshotMetadata{Index: tc.initialIndex, Term: tc.initialTerm, ConfState: raftpb.ConfState{Voters: []uint64{1}}}, oldV2Snap.Metadata)

			t.Log("Prepare the bbolt db")
			be := backend.NewDefaultBackend(lg, filepath.Join(dataDir, "member/snap/db"))
			schema.CreateMetaBucket(be.BatchTx())
			schema.NewMembershipBackend(lg, be).MustCreateBackendBuckets()

			if len(tc.clusterVersion) > 0 {
				t.Logf("Populate the cluster version: %s", tc.clusterVersion)
				schema.NewMembershipBackend(lg, be).MustSaveClusterVersionToBackend(semver.New(tc.clusterVersion))
			} else {
				t.Log("Skip populating cluster version due to not provided")
			}

			tx := be.BatchTx()
			tx.LockOutsideApply()
			t.Log("Populate the consistent index and term")
			ci := cindex.NewConsistentIndex(be)
			ci.SetConsistentIndex(tc.consistentIndex, tc.term)
			ci.UnsafeSave(tx)
			tx.Unlock()

			t.Logf("Populate members: %d", len(tc.members))
			memberBackend := schema.NewMembershipBackend(lg, be)
			for _, mID := range tc.members {
				memberBackend.MustSaveMemberToBackend(&membership.Member{ID: types.ID(mID)})
			}

			t.Logf("Populate learner: %d", len(tc.learners))
			for _, mID := range tc.learners {
				memberBackend.MustSaveMemberToBackend(&membership.Member{ID: types.ID(mID), RaftAttributes: membership.RaftAttributes{IsLearner: true}})
			}

			t.Logf("Populate removed members: %d", len(tc.removedMembers))
			for _, mID := range tc.removedMembers {
				memberBackend.MustDeleteMemberFromBackend(types.ID(mID))
			}

			t.Log("Committing bbolt db")
			be.ForceCommit()
			require.NoError(t, be.Close())

			t.Log("Creating a new v2 snapshot file based on the v3 store")
			err = createV2SnapshotFromV3Store(dataDir, backend.NewDefaultBackend(lg, filepath.Join(dataDir, "member/snap/db")))
			if len(tc.expectedErrMsg) > 0 {
				require.ErrorContains(t, err, tc.expectedErrMsg)
				return
			}
			require.NoError(t, err)

			t.Log("Loading & verifying the new latest v2 snapshot file")
			newV2Snap, err := getLatestV2Snapshot(lg, dataDir)
			require.NoError(t, err)
			require.Equal(t, raftpb.SnapshotMetadata{Index: tc.consistentIndex, Term: tc.term, ConfState: raftpb.ConfState{Voters: tc.members, Learners: tc.learners}}, newV2Snap.Metadata)

			st := v2store.New(etcdserver.StoreClusterPrefix, etcdserver.StoreKeysPrefix)
			require.NoError(t, st.Recovery(newV2Snap.Data))

			cv, err := st.Get(path.Join(etcdserver.StoreClusterPrefix, "version"), false, false)
			if len(tc.clusterVersion) > 0 {
				require.NoError(t, err)
				if !semver.New(*cv.Node.Value).Equal(*semver.New(tc.clusterVersion)) {
					t.Fatalf("Unexpected cluster version, got %s, want %s", semver.New(*cv.Node.Value).String(), tc.clusterVersion)
				}
			} else {
				require.ErrorContains(t, err, "Key not found")
			}

			members, err := st.Get(path.Join(etcdserver.StoreClusterPrefix, "members"), true, true)
			require.NoError(t, err)
			require.Len(t, members.Node.Nodes, len(tc.members)+len(tc.learners))

			removedMembers, err := st.Get(path.Join(etcdserver.StoreClusterPrefix, "removed_members"), true, true)
			if len(tc.removedMembers) > 0 {
				require.NoError(t, err)
				require.Equal(t, len(tc.removedMembers), len(removedMembers.Node.Nodes))
			} else {
				require.ErrorContains(t, err, "Key not found")
			}
		})
	}
}
