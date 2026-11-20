// Copyright 2026 The etcd Authors
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
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/mock/mockstorage"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/verify"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

func TestAsyncStorageWritesStopDrainsQueuedCommit(t *testing.T) {
	t.Setenv("ETCD_VERIFY", "all")
	lg := zaptest.NewLogger(t)
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "member", "snap")
	if err := os.MkdirAll(snapDir, 0o700); err != nil {
		t.Fatal(err)
	}
	w, err := wal.Create(lg, filepath.Join(dir, "member", "wal"), nil)
	if err != nil {
		t.Fatal(err)
	}
	entries := []*raftpb.Entry{
		{Index: new(uint64(1)), Term: new(uint64(1))},
		{Index: new(uint64(2)), Term: new(uint64(1))},
	}
	if err := w.Save(&raftpb.HardState{Term: new(uint64(1)), Vote: new(uint64(1))}, entries); err != nil {
		t.Fatal(err)
	}

	// Both entries are stable and applied. Their commit metadata is still queued.
	be := backend.NewDefaultBackend(lg, filepath.Join(snapDir, "db"))
	tx := be.BatchTx()
	tx.LockOutsideApply()
	schema.UnsafeCreateMetaBucket(tx)
	schema.UnsafeUpdateConsistentIndex(tx, 2, 1)
	tx.Unlock()
	if err := be.Close(); err != nil {
		t.Fatal(err)
	}

	saveStartedC := make(chan struct{}, 1)
	allowSaveC := make(chan struct{})
	releaseSave := sync.OnceFunc(func() { close(allowSaveC) })
	r := newRaftNode(raftNodeConfig{
		lg:                 lg,
		Node:               newAsyncReadyNode(),
		localID:            1,
		asyncStorageWrites: true,
		storage: &blockingSaveStorage{
			Storage:      serverstorage.NewStorage(lg, w, snap.New(lg, snapDir)),
			saveStartedC: saveStartedC,
			allowSaveC:   allowSaveC,
		},
		raftStorage: raft.NewMemoryStorage(),
		transport:   newNopTransporter(),
	})
	r.start(newTestRaftReadyHandler())
	defer func() {
		releaseSave()
		r.stop()
	}()
	for commit := uint64(1); commit <= 2; commit++ {
		r.appendc <- &raftpb.Message{
			Type:   raftpb.MsgStorageAppend.Enum(),
			To:     new(raft.LocalAppendThread),
			From:   new(uint64(1)),
			Term:   new(uint64(1)),
			Vote:   new(uint64(1)),
			Commit: new(commit),
		}
		if commit == 1 {
			<-saveStartedC
		}
	}

	stoppedC := make(chan struct{})
	go func() {
		r.stop()
		close(stoppedC)
	}()
	<-r.asyncStopc
	releaseSave()
	select {
	case <-stoppedC:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown blocked while draining queued commit metadata")
	}
	if err := verify.Verify(verify.Config{DataDir: dir, Logger: lg}); err != nil {
		t.Fatal(err)
	}
}

func TestAsyncStorageWritesStopSkipsUnappliedSnapshot(t *testing.T) {
	n := newBlockingStepNode()
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               n,
		localID:            1,
		asyncStorageWrites: true,
		storage:            mockstorage.NewStorageRecorder(""),
		raftStorage:        raft.NewMemoryStorage(),
		transport:          newNopTransporter(),
	})
	r.appendc <- newStorageAppendMessage(1, 1)
	r.start(newTestRaftReadyHandler())
	defer r.stop()
	<-n.stepStartedC

	snapshot := raftpb.EnsureSnapshot(nil)
	snapshot.Metadata.Index = new(uint64(5))
	snapshot.Metadata.Term = new(uint64(2))
	snapshot.Metadata.ConfState = &raftpb.ConfState{Voters: []uint64{1}}
	r.appendc <- &raftpb.Message{
		Type:     raftpb.MsgStorageAppend.Enum(),
		To:       new(raft.LocalAppendThread),
		From:     new(uint64(1)),
		Snapshot: snapshot,
	}
	r.appendc <- newStorageAppendMessage(6, 2)
	r.stop()
	last, err := r.raftStorage.LastIndex()
	if err != nil {
		t.Fatal(err)
	}
	if last != 1 {
		t.Fatalf("last index = %d, want 1 without the unapplied snapshot or its later entries", last)
	}
}
