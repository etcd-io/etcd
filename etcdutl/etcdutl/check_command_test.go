// Copyright 2024 The etcd Authors
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
	"os"
	"path/filepath"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
)

// TestCheckV2StoreDetectsWALContent verifies that checkV2StoreDataDir catches
// custom v2store content written only in WAL entries (not in the snapshot).
func TestCheckV2StoreDetectsWALContent(t *testing.T) {
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "snap")
	walDir := filepath.Join(dir, "wal")

	if err := os.MkdirAll(snapDir, 0755); err != nil {
		t.Fatal(err)
	}
	// walDir must NOT be pre-created; wal.Create creates it atomically.

	lg := zap.NewNop()

	// Build a clean (metadata-only) v2store and persist it as the base snapshot.
	st := v2store.New(etcdserver.StoreClusterPrefix, etcdserver.StoreKeysPrefix)
	snapshotData, err := st.Save()
	if err != nil {
		t.Fatal(err)
	}

	// Create the WAL and record a snapshot marker at {Index:1, Term:1}.
	walMeta := pbutil.MustMarshal(&etcdserverpb.Metadata{NodeID: 1, ClusterID: 1})
	w, err := wal.Create(lg, walDir, walMeta)
	if err != nil {
		t.Fatal(err)
	}
	walsnap := walpb.Snapshot{Index: 1, Term: 1, ConfState: &raftpb.ConfState{Voters: []uint64{1}}}

	if err := w.SaveSnapshot(walsnap); err != nil {
		t.Fatal(err)
	}

	// Append a v2 PUT entry for a non-metadata key *after* the snapshot.
	v2req := &etcdserverpb.Request{
		Method: "PUT",
		Path:   "/custom/key",
		Val:    "custom-value",
	}
	entry := raftpb.Entry{
		Index: 2,
		Term:  1,
		Type:  raftpb.EntryNormal,
		Data:  pbutil.MustMarshal(v2req),
	}
	if err := w.Save(raftpb.HardState{Term: 1, Commit: 2}, []raftpb.Entry{entry}); err != nil {
		t.Fatal(err)
	}
	w.Close()

	// Persist the snap file matching the WAL snapshot marker.
	snapshot := raftpb.Snapshot{
		Data: snapshotData,
		Metadata: raftpb.SnapshotMetadata{
			Index:     1,
			Term:      1,
			ConfState: raftpb.ConfState{Voters: []uint64{1}},
		},
	}
	ss := snap.New(lg, snapDir)
	if err := ss.SaveSnap(snapshot); err != nil {
		t.Fatal(err)
	}

	// The fix must detect the custom content written via WAL.
	if err := checkV2StoreDataDir(snapDir, walDir); err == nil {
		t.Fatal("expected error detecting custom v2store content in WAL, got nil")
	}
}
