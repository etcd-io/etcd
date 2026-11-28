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
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/pkg/v3/wait"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/mock/mockstorage"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/verify"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

func TestAsyncStorageWritesSavesContiguousAppendsOnce(t *testing.T) {
	storage := &saveCaptureStorage{Storage: mockstorage.NewStorageRecorder("")}
	n := newAsyncReadyNode()
	r := newAsyncRaftNode(t, n, storage)
	first := newStorageAppendMessage(1, 1)
	setHardState(first, 1, 1, 1)
	second := newStorageAppendMessage(2, 2)
	setHardState(second, 2, 2, 2)

	r.processStorageAppends([]*raftpb.Message{first, second})

	require.Equal(t, 1, storage.saveCount)
	require.Equal(t, uint64(2), storage.hardState.GetTerm())
	require.Equal(t, uint64(2), storage.hardState.GetVote())
	require.Equal(t, uint64(2), storage.hardState.GetCommit())
	require.Equal(t, []uint64{1, 2}, entryIndexes(storage.entries))
	require.Equal(t, uint64(1), receive(t, n.stepc).GetIndex())
	require.Equal(t, uint64(2), receive(t, n.stepc).GetIndex())
}

func TestAsyncStorageWritesAppendResponsesAfterSave(t *testing.T) {
	n := newAsyncReadyNode()
	saveStartedC := make(chan struct{}, 2)
	allowSaveC := make(chan struct{})
	r := newAsyncRaftNode(t, n, &blockingSaveStorage{
		Storage:      mockstorage.NewStorageRecorder(""),
		saveStartedC: saveStartedC,
		allowSaveC:   allowSaveC,
	})
	r.appendWorker.appendc <- newStorageAppendMessage(1, 1)
	r.appendWorker.appendc <- newStorageAppendMessage(2, 2)
	r.start(newTestRaftReadyHandler())
	defer r.stop()
	defer close(allowSaveC)

	receive(t, saveStartedC)
	select {
	case m := <-n.stepc:
		t.Fatalf("storage response delivered before the append was saved: %v", m)
	default:
	}
	allowSaveC <- struct{}{}
	require.Equal(t, uint64(1), receive(t, n.stepc).GetIndex())
	require.Equal(t, uint64(2), receive(t, n.stepc).GetIndex())
	select {
	case <-saveStartedC:
		t.Fatal("contiguous queued appends were not saved together")
	default:
	}

	lastIndex, err := r.raftStorage.LastIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(2), lastIndex)
	select {
	case <-n.advancec:
		t.Fatal("Advance called with asynchronous storage writes")
	default:
	}
}

func TestAsyncStorageWritesDoesNotSaveOverwritesTogether(t *testing.T) {
	n := newAsyncReadyNode()
	saveStartedC := make(chan struct{}, 2)
	allowSaveC := make(chan struct{})
	r := newAsyncRaftNode(t, n, &blockingSaveStorage{
		Storage:      mockstorage.NewStorageRecorder(""),
		saveStartedC: saveStartedC,
		allowSaveC:   allowSaveC,
	})
	r.appendWorker.appendc <- newStorageAppendMessage(1, 1)
	r.appendWorker.appendc <- newStorageAppendMessage(1, 2)
	r.start(newTestRaftReadyHandler())
	defer r.stop()
	defer close(allowSaveC)

	receive(t, saveStartedC)
	allowSaveC <- struct{}{}
	require.Equal(t, uint64(1), receive(t, n.stepc).GetLogTerm())

	receive(t, saveStartedC)
	select {
	case m := <-n.stepc:
		t.Fatalf("overwrite response delivered before its save: %v", m)
	default:
	}
	allowSaveC <- struct{}{}
	require.Equal(t, uint64(2), receive(t, n.stepc).GetLogTerm())

	term, err := r.raftStorage.Term(1)
	require.NoError(t, err)
	require.Equal(t, uint64(2), term)
}

func TestAsyncStorageWritesDoesNotSaveSnapshotWithPrecedingAppend(t *testing.T) {
	hardStateOnly := &raftpb.Message{
		Type: raftpb.MsgStorageAppend.Enum(),
		To:   new(raft.LocalAppendThread),
		From: new(uint64(1)),
	}
	setHardState(hardStateOnly, 1, 1, 0)
	for name, previous := range map[string]*raftpb.Message{
		"entries":    newStorageAppendMessage(1, 1),
		"hard state": hardStateOnly,
	} {
		t.Run(name, func(t *testing.T) {
			storage := mockstorage.NewStorageRecorder("")
			r := newAsyncRaftNode(t, newAsyncReadyNode(), storage)
			r.appendWorker.appendc <- previous
			r.appendWorker.appendc <- newSnapshotAppendMessage(5, 2)
			r.start(newTestRaftReadyHandler())
			defer r.stop()

			receive(t, receive(t, r.applyc).notifyc)
			require.Equal(t, []string{"Save", "SaveSnap", "Save", "Sync", "Release"}, actionNames(storage))
		})
	}
}

func TestAsyncStorageWritesTracksRaftCommitIndex(t *testing.T) {
	n := newAsyncReadyNode()
	r := newAsyncRaftNode(t, n, mockstorage.NewStorageRecorder(""))
	committedC := make(chan uint64, 2)
	rh := newTestRaftReadyHandler()
	rh.updateCommittedIndex = func(ci uint64) { committedC <- ci }
	r.start(rh)
	defer r.stop()

	// Only entry 1 is handed to the applier, while Raft has committed up to 7.
	appendMessage := newStorageAppendMessage(7, 1)
	for index := uint64(6); index > 0; index-- {
		appendMessage.Entries = append([]*raftpb.Entry{{Index: new(index), Term: new(uint64(1))}}, appendMessage.Entries...)
	}
	setHardState(appendMessage, 1, 1, 7)
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{
		appendMessage,
		newStorageApplyMessage(appendMessage.Entries[0]),
	}}
	require.Equal(t, uint64(7), receive(t, committedC))
	receive(t, r.applyc)
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{newSnapshotAppendMessage(20, 2)}}
	require.Equal(t, uint64(20), receive(t, committedC))
}

func TestAsyncStorageWritesSendsPeerMessagesWithoutWaitingForAppend(t *testing.T) {
	n := newAsyncReadyNode()
	saveStartedC := make(chan struct{}, 1)
	allowSaveC := make(chan struct{})
	tr := newRecordingTransporter()
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		isIDRemoved:        func(uint64) bool { return false },
		Node:               n,
		raftStorage:        raft.NewMemoryStorage(),
		storage:            &blockingSaveStorage{Storage: mockstorage.NewStorageRecorder(""), saveStartedC: saveStartedC, allowSaveC: allowSaveC},
		transport:          tr,
		asyncStorageWrites: true,
		localID:            1,
	})
	r.start(newTestRaftReadyHandler())
	defer r.stop()
	defer close(allowSaveC)

	appendMessage := newStorageAppendMessage(1, 1)
	appendMessage.Responses = append([]*raftpb.Message{{
		Type:  raftpb.MsgAppResp.Enum(),
		To:    new(uint64(2)),
		From:  new(uint64(1)),
		Index: new(uint64(1)),
	}}, appendMessage.Responses...)
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{
		{Type: raftpb.MsgHeartbeat.Enum(), To: new(uint64(3)), From: new(uint64(1))},
		appendMessage,
	}}

	receive(t, saveStartedC)
	sent := receive(t, tr.sentc)
	require.Len(t, sent, 1)
	require.Equal(t, raftpb.MsgHeartbeat, sent[0].GetType())

	allowSaveC <- struct{}{}
	sent = receive(t, tr.sentc)
	require.Len(t, sent, 1)
	require.Equal(t, raftpb.MsgAppResp, sent[0].GetType())
	require.Equal(t, uint64(2), sent[0].GetTo())
	require.Equal(t, raftpb.MsgStorageAppendResp, receive(t, n.stepc).GetType())
}

func TestAsyncStorageWritesSendsPeerMessagesBeforeQueuingAppend(t *testing.T) {
	n := newAsyncReadyNode()
	saveStartedC := make(chan struct{}, 1)
	allowSaveC := make(chan struct{})
	tr := newRecordingTransporter()
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		isIDRemoved:        func(uint64) bool { return false },
		Node:               n,
		raftStorage:        raft.NewMemoryStorage(),
		storage:            &blockingSaveStorage{Storage: mockstorage.NewStorageRecorder(""), saveStartedC: saveStartedC, allowSaveC: allowSaveC},
		transport:          tr,
		asyncStorageWrites: true,
		localID:            1,
	})
	r.start(newTestRaftReadyHandler())
	defer r.stop()
	defer close(allowSaveC)

	// Park the worker in Save, then fill appendc, so that the Ready loop blocks
	// queuing the next append.
	r.appendWorker.appendc <- newHardStateAppendMessage(1)
	receive(t, saveStartedC)
	for len(r.appendWorker.appendc) < asyncAppendQueueSize {
		r.appendWorker.appendc <- newHardStateAppendMessage(1)
	}
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{
		{Type: raftpb.MsgHeartbeat.Enum(), To: new(uint64(3)), From: new(uint64(1))},
		newHardStateAppendMessage(1),
	}}

	sent := receive(t, tr.sentc)
	require.Len(t, sent, 1)
	require.Equal(t, raftpb.MsgHeartbeat, sent[0].GetType())
}

func TestAsyncStorageWritesApplyDoesNotWaitForAppend(t *testing.T) {
	n := &stepOrderNode{asyncReadyNode: newAsyncReadyNode()}
	saveStartedC := make(chan struct{}, 1)
	allowSaveC := make(chan struct{})
	r := newAsyncRaftNode(t, n, &blockingSaveStorage{
		Storage:      mockstorage.NewStorageRecorder(""),
		saveStartedC: saveStartedC,
		allowSaveC:   allowSaveC,
	})
	r.start(newTestRaftReadyHandler())
	defer r.stop()
	defer close(allowSaveC)

	// Raft asks to apply entry 1 only once it is stable; the queued append
	// only persists its commit index.
	entry := &raftpb.Entry{Index: new(uint64(1)), Term: new(uint64(1))}
	require.NoError(t, r.raftStorage.Append([]*raftpb.Entry{entry}))
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{newHardStateAppendMessage(1), newStorageApplyMessage(entry)}}

	receive(t, saveStartedC)
	ap := receive(t, r.applyc)
	require.Equal(t, []uint64{1}, entryIndexes(ap.entries))
	requireClosed(t, ap.notifyc, "notifyc of async apply")
	select {
	case m := <-n.stepc:
		t.Fatalf("apply response delivered before application: %v", m)
	default:
	}

	// The append is still blocked in Save.
	n.watch(ap.raftAdvancedC)
	ap.onApplyCompleted()
	require.Equal(t, raftpb.MsgStorageApplyResp, receive(t, n.stepc).GetType())
	require.Equalf(t, []bool{false}, n.watchClosedAtStep(), "raftAdvancedC closed before MsgStorageApplyResp was stepped")
	requireClosed(t, ap.raftAdvancedC, "raftAdvancedC")
}

func TestAsyncStorageWritesKeepsAppendingWhileSnapshotInstalls(t *testing.T) {
	n := newAsyncReadyNode()
	storage := &saveCaptureStorage{Storage: mockstorage.NewStorageRecorder("")}
	tr := newRecordingTransporter()
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		isIDRemoved:        func(uint64) bool { return false },
		Node:               n,
		raftStorage:        raft.NewMemoryStorage(),
		storage:            storage,
		transport:          tr,
		asyncStorageWrites: true,
		localID:            1,
	})
	r.start(newTestRaftReadyHandler())
	defer r.stop()

	snapshotAppend := newSnapshotAppendMessage(5, 2)
	snapshotAppend.Responses = append([]*raftpb.Message{newAppResp(2, 5)}, snapshotAppend.Responses...)
	nextAppend := newStorageAppendMessage(6, 2)
	nextAppend.Responses = append([]*raftpb.Message{newAppResp(2, 6)}, nextAppend.Responses...)
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{snapshotAppend}}

	// The applier gets the snapshot first, and notifyc tells it when the
	// snapshot is persisted, including the HardState and MemoryStorage.
	ap := receive(t, r.applyc)
	require.Equal(t, uint64(5), ap.snapshot.Metadata.GetIndex())
	receive(t, ap.notifyc)
	require.Equal(t, uint64(5), storage.hardState.GetCommit())
	stored, err := r.raftStorage.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(5), stored.Metadata.GetIndex())
	r.waitForCommitSaved(5)
	require.Equal(t, uint64(5), receive(t, tr.sentc)[0].GetIndex())

	// While the applier installs it, the following appends are persisted and
	// acknowledged to the leader, but not to the local node.
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{nextAppend}}
	require.Equal(t, uint64(6), receive(t, tr.sentc)[0].GetIndex())
	lastIndex, err := r.raftStorage.LastIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(6), lastIndex)
	select {
	case m := <-n.stepc:
		t.Fatalf("response delivered to the local node before the snapshot was installed: %v", m)
	default:
	}

	ap.onApplyCompleted()
	require.Equal(t, raftpb.MsgStorageAppendResp, receive(t, n.stepc).GetType())
	require.Equal(t, uint64(6), receive(t, n.stepc).GetIndex())
}

func TestAsyncStorageWritesSnapshotWaitsForPreviousInstall(t *testing.T) {
	n := newAsyncReadyNode()
	storage := mockstorage.NewStorageRecorder("")
	r := newAsyncRaftNode(t, n, storage)
	r.start(newTestRaftReadyHandler())
	defer r.stop()

	n.readyc <- raft.Ready{Messages: []*raftpb.Message{newSnapshotAppendMessage(5, 2), newSnapshotAppendMessage(10, 3)}}
	first := receive(t, r.applyc)
	receive(t, first.notifyc)

	// Persisting the second snapshot would release the .snap.db file of the
	// first one, which the applier has not installed yet.
	select {
	case ap := <-r.applyc:
		t.Fatalf("snapshot %d handed to the applier before the previous one was installed", ap.snapshot.Metadata.GetIndex())
	case <-time.After(100 * time.Millisecond):
	}
	require.Equal(t, []string{"SaveSnap", "Save", "Sync", "Release"}, actionNames(storage))

	first.onApplyCompleted()
	require.Equal(t, uint64(5), receive(t, n.stepc).GetSnapshot().GetMetadata().GetIndex())
	second := receive(t, r.applyc)
	require.Equal(t, uint64(10), second.snapshot.Metadata.GetIndex())
}

func TestAsyncStorageWritesWaitForCommitSaved(t *testing.T) {
	saveStartedC := make(chan struct{}, 1)
	allowSaveC := make(chan struct{})
	releaseSave := sync.OnceFunc(func() { close(allowSaveC) })
	raftStorage := raft.NewMemoryStorage()
	require.NoError(t, raftStorage.SetHardState(&raftpb.HardState{Term: new(uint64(1)), Commit: new(uint64(2))}))
	r := newRaftNode(raftNodeConfig{
		lg:          zaptest.NewLogger(t),
		Node:        newAsyncReadyNode(),
		raftStorage: raftStorage,
		storage: &blockingSaveStorage{
			Storage:      mockstorage.NewStorageRecorder(""),
			saveStartedC: saveStartedC,
			allowSaveC:   allowSaveC,
		},
		transport:          newNopTransporter(),
		asyncStorageWrites: true,
		localID:            1,
	})
	r.start(newTestRaftReadyHandler())
	defer func() {
		releaseSave()
		r.stop()
	}()

	// The commit index Raft restarts from is already saved.
	r.waitForCommitSaved(2)

	r.appendWorker.appendc <- newHardStateAppendMessage(3)
	receive(t, saveStartedC)
	waitedC := make(chan struct{})
	go func() {
		r.waitForCommitSaved(3)
		close(waitedC)
	}()
	select {
	case <-waitedC:
		t.Fatal("waitForCommitSaved returned before the commit index was saved")
	case <-time.After(100 * time.Millisecond):
	}
	releaseSave()
	receive(t, waitedC)
}

func TestAsyncStorageWritesStopUnblocksWaitForCommitSaved(t *testing.T) {
	r := newAsyncRaftNode(t, newAsyncReadyNode(), mockstorage.NewStorageRecorder(""))
	r.start(newTestRaftReadyHandler())

	waitedC := make(chan struct{})
	go func() {
		r.waitForCommitSaved(1)
		close(waitedC)
	}()
	requireStops(t, r)
	receive(t, waitedC)
}

func TestAsyncStorageWritesDiskSnapshotWaitsForCommitSaved(t *testing.T) {
	saveStartedC := make(chan struct{}, 1)
	allowSaveC := make(chan struct{})
	releaseSave := sync.OnceFunc(func() { close(allowSaveC) })
	storage := &snapshotRecordingStorage{Storage: &blockingSaveStorage{
		Storage:      mockstorage.NewStorageRecorder(""),
		saveStartedC: saveStartedC,
		allowSaveC:   allowSaveC,
	}}
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               newAsyncReadyNode(),
		raftStorage:        newMemoryStorage(t, 3, 1),
		storage:            storage,
		transport:          newNopTransporter(),
		asyncStorageWrites: true,
		localID:            1,
	})
	r.start(newTestRaftReadyHandler())
	defer func() {
		releaseSave()
		r.stop()
	}()
	s := newSnapshotTestServer(t, r, 3)

	// Entries up to 3 are applied, while their commit index is still queued.
	r.appendWorker.appendc <- newHardStateAppendMessage(3)
	receive(t, saveStartedC)
	doneC := make(chan struct{})
	go func() {
		s.snapshot(&etcdProgress{appliedi: 3, confState: &raftpb.ConfState{Voters: []uint64{1}}}, true)
		close(doneC)
	}()
	select {
	case <-doneC:
		t.Fatal("snapshot saved to the WAL before the commit index")
	case <-time.After(100 * time.Millisecond):
	}
	releaseSave()
	receive(t, doneC)
	require.Equal(t, []savedSnapshot{{index: 3, savedCommit: 3}}, storage.saved())
}

func TestAsyncStorageWritesDiskSnapshotOrderedWithIncomingSnapshot(t *testing.T) {
	incomingSavedC := make(chan struct{})
	localSavedC := make(chan struct{})
	storage := &snapshotRecordingStorage{
		Storage: mockstorage.NewStorageRecorder(""),
		afterSaveSnap: func(index uint64) {
			if index != 10 {
				close(localSavedC)
				return
			}
			close(incomingSavedC)
			// Give a snapshot of the local state machine the chance to be saved
			// after the incoming one, before MemoryStorage has the latter.
			select {
			case <-localSavedC:
			case <-time.After(200 * time.Millisecond):
			}
		},
	}
	r := newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               newAsyncReadyNode(),
		raftStorage:        newMemoryStorage(t, 3, 3),
		storage:            storage,
		transport:          newNopTransporter(),
		asyncStorageWrites: true,
		localID:            1,
	})
	s := newSnapshotTestServer(t, r, 3)
	r.appendWorker.appendc <- newSnapshotAppendMessage(10, 2)
	r.start(newTestRaftReadyHandler())
	defer r.stop()
	go func() { <-r.applyc }()

	receive(t, incomingSavedC)
	doneC := make(chan struct{})
	go func() {
		s.snapshot(&etcdProgress{appliedi: 3, confState: &raftpb.ConfState{Voters: []uint64{1}}}, true)
		close(doneC)
	}()
	receive(t, doneC)
	// The snapshot of the local state machine became out of date instead of
	// being saved after the incoming one.
	require.Equal(t, []savedSnapshot{{index: 10, savedCommit: 0}}, storage.saved())
}

func TestAsyncStorageWritesStopCancelsBlockedStorageResponse(t *testing.T) {
	n := newBlockingStepNode()
	r := newAsyncRaftNode(t, n, mockstorage.NewStorageRecorder(""))
	r.appendWorker.appendc <- newStorageAppendMessage(1, 1)
	r.start(newTestRaftReadyHandler())

	receive(t, n.stepStartedC)
	requireStops(t, r)
}

func TestAsyncStorageWritesStopDrainsQueuedCommit(t *testing.T) {
	t.Setenv("ETCD_VERIFY", "all")
	lg := zaptest.NewLogger(t)
	dir := t.TempDir()
	snapDir := filepath.Join(dir, "member", "snap")
	require.NoError(t, os.MkdirAll(snapDir, 0o700))
	w, err := wal.Create(lg, filepath.Join(dir, "member", "wal"), nil)
	require.NoError(t, err)
	entries := []*raftpb.Entry{
		{Index: new(uint64(1)), Term: new(uint64(1))},
		{Index: new(uint64(2)), Term: new(uint64(1))},
	}
	require.NoError(t, w.Save(&raftpb.HardState{Term: new(uint64(1)), Vote: new(uint64(1))}, entries))

	// Both entries are stable and applied, while their commit index is still
	// queued for the append worker.
	be := backend.NewDefaultBackend(lg, filepath.Join(snapDir, "db"))
	tx := be.BatchTx()
	tx.LockOutsideApply()
	schema.UnsafeCreateMetaBucket(tx)
	schema.UnsafeUpdateConsistentIndex(tx, 2, 1)
	tx.Unlock()
	require.NoError(t, be.Close())

	saveStartedC := make(chan struct{}, 1)
	allowSaveC := make(chan struct{})
	releaseSave := sync.OnceFunc(func() { close(allowSaveC) })
	r := newAsyncRaftNode(t, newAsyncReadyNode(), &blockingSaveStorage{
		Storage:      serverstorage.NewStorage(lg, w, snap.New(lg, snapDir)),
		saveStartedC: saveStartedC,
		allowSaveC:   allowSaveC,
	})
	r.start(newTestRaftReadyHandler())
	defer func() {
		releaseSave()
		r.stop()
	}()
	r.appendWorker.appendc <- newHardStateAppendMessage(1)
	receive(t, saveStartedC)
	r.appendWorker.appendc <- newHardStateAppendMessage(2)

	stoppedC := make(chan struct{})
	go func() {
		r.stop()
		close(stoppedC)
	}()
	receive(t, r.appendWorker.stopc)
	releaseSave()
	receive(t, stoppedC)
	require.NoError(t, verify.Verify(verify.Config{DataDir: dir, Logger: lg}))
}

func TestAsyncStorageWritesStopSkipsUnappliedSnapshot(t *testing.T) {
	n := newBlockingStepNode()
	r := newAsyncRaftNode(t, n, mockstorage.NewStorageRecorder(""))
	r.appendWorker.appendc <- newStorageAppendMessage(1, 1)
	r.start(newTestRaftReadyHandler())
	receive(t, n.stepStartedC)

	r.appendWorker.appendc <- newSnapshotAppendMessage(10, 3)
	r.appendWorker.appendc <- newStorageAppendMessage(11, 3)
	requireStops(t, r)

	lastIndex, err := r.raftStorage.LastIndex()
	require.NoError(t, err)
	require.Equalf(t, uint64(1), lastIndex, "the snapshot or the entries after it were persisted on stop")
}

func TestAsyncStorageWritesStopWhileSnapshotInstalls(t *testing.T) {
	n := newAsyncReadyNode()
	r := newAsyncRaftNode(t, n, mockstorage.NewStorageRecorder(""))
	r.start(newTestRaftReadyHandler())

	n.readyc <- raft.Ready{Messages: []*raftpb.Message{newSnapshotAppendMessage(5, 2), newStorageAppendMessage(6, 2)}}
	receive(t, receive(t, r.applyc).notifyc)
	requireStops(t, r)

	select {
	case m := <-n.stepc:
		t.Fatalf("response delivered to the local node for a snapshot that was not installed: %v", m)
	default:
	}
}

func TestAsyncStorageWritesStopBeforeSnapshotHandOff(t *testing.T) {
	storage := mockstorage.NewStorageRecorder("")
	r := newAsyncRaftNode(t, newAsyncReadyNode(), storage)
	// Nothing receives the snapshot, like once EtcdServer.run has returned.
	// Queue it before the worker starts, so that appendc only empties once the
	// worker took it and is handing it to the applier.
	r.appendWorker.appendc <- newSnapshotAppendMessage(5, 2)
	r.start(newTestRaftReadyHandler())
	require.Eventually(t, func() bool { return len(r.appendWorker.appendc) == 0 }, time.Second, time.Millisecond)
	requireStops(t, r)

	require.Emptyf(t, actionNames(storage), "a snapshot the applier never got was persisted")
	lastIndex, err := r.raftStorage.LastIndex()
	require.NoError(t, err)
	require.Zero(t, lastIndex)
}

func TestAsyncStorageWritesStopWhileApplyHandOffBlocked(t *testing.T) {
	n := newAsyncReadyNode()
	r := newAsyncRaftNode(t, n, mockstorage.NewStorageRecorder(""))
	r.start(newTestRaftReadyHandler())

	// Nothing receives the apply, like once EtcdServer.run has returned.
	n.readyc <- raft.Ready{Messages: []*raftpb.Message{newStorageApplyMessage(&raftpb.Entry{Index: new(uint64(1)), Term: new(uint64(1))})}}
	require.Eventually(t, func() bool { return len(n.readyc) == 0 }, time.Second, time.Millisecond)
	requireStops(t, r)

	select {
	case m := <-n.stepc:
		t.Fatalf("response delivered for an apply that was never handed off: %v", m)
	default:
	}
}

func TestApplyAllCallsOnApplyCompletedAfterApplication(t *testing.T) {
	s := &EtcdServer{lgMu: new(sync.RWMutex), lg: zaptest.NewLogger(t), applyWait: wait.NewTimeList()}
	appliedC := s.applyWait.Wait(1)
	notifyc := make(chan struct{})
	calledC := make(chan struct{})
	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		s.applyAll(&etcdProgress{appliedi: 1, diskSnapshotIndex: 1, memorySnapshotIndex: 1}, &toApply{
			snapshot: raftpb.EnsureSnapshot(nil),
			notifyc:  notifyc,
			onApplyCompleted: func() {
				requireClosed(t, appliedC, "applied index notification")
				close(calledC)
			},
		})
	}()

	// applyAll waits for the raft node before calling it.
	select {
	case <-calledC:
		t.Fatal("onApplyCompleted called before notifyc")
	case <-time.After(100 * time.Millisecond):
	}
	close(notifyc)
	receive(t, calledC)
	receive(t, doneC)
}

// newAsyncRaftNode returns a raftNode with asynchronous storage writes and
// local node ID 1.
func newAsyncRaftNode(t *testing.T, n raft.Node, storage serverstorage.Storage) *raftNode {
	return newRaftNode(raftNodeConfig{
		lg:                 zaptest.NewLogger(t),
		Node:               n,
		raftStorage:        raft.NewMemoryStorage(),
		storage:            storage,
		transport:          newNopTransporter(),
		asyncStorageWrites: true,
		localID:            1,
	})
}

// newMemoryStorage returns a MemoryStorage with entries 1 to lastIndex of
// term 1 and the given commit index.
func newMemoryStorage(t *testing.T, lastIndex, commit uint64) *raft.MemoryStorage {
	storage := raft.NewMemoryStorage()
	var entries []*raftpb.Entry
	for index := uint64(1); index <= lastIndex; index++ {
		entries = append(entries, &raftpb.Entry{Index: new(index), Term: new(uint64(1))})
	}
	require.NoError(t, storage.Append(entries))
	require.NoError(t, storage.SetHardState(&raftpb.HardState{Term: new(uint64(1)), Commit: new(commit)}))
	return storage
}

// newSnapshotTestServer returns an EtcdServer on top of r that can save
// snapshots of its state machine, which has applied up to appliedIndex.
func newSnapshotTestServer(t *testing.T, r *raftNode, appliedIndex uint64) *EtcdServer {
	lg := zaptest.NewLogger(t)
	be, _ := betesting.NewDefaultTmpBackend(t)
	t.Cleanup(func() { betesting.Close(t, be) })
	ci := cindex.NewConsistentIndex(be)
	ci.SetConsistentIndex(appliedIndex, 1)
	s := &EtcdServer{
		lgMu:         new(sync.RWMutex),
		lg:           lg,
		r:            *r,
		be:           be,
		consistIndex: ci,
		cluster:      membership.NewCluster(lg),
		kv:           mvcc.New(lg, be, &lease.FakeLessor{}, mvcc.StoreConfig{}),
	}
	t.Cleanup(func() { require.NoError(t, s.kv.Close()) })
	return s
}

func receive[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case v := <-c:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting on a channel")
	}
	var zero T
	return zero
}

func requireStops(t *testing.T, r *raftNode) {
	t.Helper()
	stoppedC := make(chan struct{})
	go func() {
		r.stop()
		close(stoppedC)
	}()
	select {
	case <-stoppedC:
	case <-time.After(time.Second):
		t.Fatal("raft node stop blocked")
	}
}

func requireClosed(t *testing.T, c <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-c:
	default:
		t.Fatalf("%s would block", name)
	}
}

func setHardState(m *raftpb.Message, term, vote, commit uint64) {
	m.Term = new(term)
	m.Vote = new(vote)
	m.Commit = new(commit)
}

func entryIndexes(entries []*raftpb.Entry) []uint64 {
	indexes := make([]uint64, 0, len(entries))
	for _, e := range entries {
		indexes = append(indexes, e.GetIndex())
	}
	return indexes
}

// newStorageAppendMessage returns a MsgStorageAppend of one entry, with the
// MsgStorageAppendResp acknowledging it to local node 1.
func newStorageAppendMessage(index, term uint64) *raftpb.Message {
	return &raftpb.Message{
		Type:    raftpb.MsgStorageAppend.Enum(),
		To:      new(raft.LocalAppendThread),
		From:    new(uint64(1)),
		Entries: []*raftpb.Entry{{Index: new(index), Term: new(term)}},
		Responses: []*raftpb.Message{{
			Type:    raftpb.MsgStorageAppendResp.Enum(),
			To:      new(uint64(1)),
			From:    new(raft.LocalAppendThread),
			Index:   new(index),
			LogTerm: new(term),
		}},
	}
}

// newHardStateAppendMessage returns a MsgStorageAppend of only a HardState
// with the given commit index, without responses.
func newHardStateAppendMessage(commit uint64) *raftpb.Message {
	m := &raftpb.Message{
		Type: raftpb.MsgStorageAppend.Enum(),
		To:   new(raft.LocalAppendThread),
		From: new(uint64(1)),
	}
	setHardState(m, 1, 1, commit)
	return m
}

// newSnapshotAppendMessage returns a MsgStorageAppend of an incoming snapshot,
// with the HardState Raft sends along and the MsgStorageAppendResp
// acknowledging it to local node 1.
func newSnapshotAppendMessage(index, term uint64) *raftpb.Message {
	snapshot := raftpb.EnsureSnapshot(nil)
	snapshot.Metadata.Index = new(index)
	snapshot.Metadata.Term = new(term)
	snapshot.Metadata.ConfState = &raftpb.ConfState{Voters: []uint64{1}}
	m := &raftpb.Message{
		Type:     raftpb.MsgStorageAppend.Enum(),
		To:       new(raft.LocalAppendThread),
		From:     new(uint64(1)),
		Snapshot: snapshot,
		Responses: []*raftpb.Message{{
			Type:     raftpb.MsgStorageAppendResp.Enum(),
			To:       new(uint64(1)),
			From:     new(raft.LocalAppendThread),
			Snapshot: snapshot,
		}},
	}
	// Restoring a snapshot moves the commit index to it.
	setHardState(m, term, 0, index)
	return m
}

// newStorageApplyMessage returns a MsgStorageApply of the given entries, with
// the MsgStorageApplyResp to local node 1.
func newStorageApplyMessage(entries ...*raftpb.Entry) *raftpb.Message {
	return &raftpb.Message{
		Type:    raftpb.MsgStorageApply.Enum(),
		To:      new(raft.LocalApplyThread),
		From:    new(uint64(1)),
		Entries: entries,
		Responses: []*raftpb.Message{{
			Type:    raftpb.MsgStorageApplyResp.Enum(),
			To:      new(uint64(1)),
			From:    new(raft.LocalApplyThread),
			Entries: entries,
		}},
	}
}

// newAppResp returns a MsgAppResp of local node 1 to the given peer.
func newAppResp(to, index uint64) *raftpb.Message {
	return &raftpb.Message{
		Type:  raftpb.MsgAppResp.Enum(),
		To:    new(to),
		From:  new(uint64(1)),
		Index: new(index),
	}
}

func actionNames(storage *mockstorage.StorageRecorder) []string {
	var names []string
	for _, a := range storage.Action() {
		names = append(names, a.Name)
	}
	return names
}

func newTestRaftReadyHandler() *raftReadyHandler {
	return &raftReadyHandler{
		getLead:              func() uint64 { return 0 },
		updateLead:           func(uint64) {},
		updateLeadership:     func(bool) {},
		updateCommittedIndex: func(uint64) {},
	}
}

// asyncReadyNode records the messages stepped into it, and Advance calls.
type asyncReadyNode struct {
	*readyNode
	stepc    chan *raftpb.Message
	advancec chan struct{}
}

func newAsyncReadyNode() *asyncReadyNode {
	return &asyncReadyNode{
		readyNode: newNopReadyNode(),
		stepc:     make(chan *raftpb.Message, 8),
		advancec:  make(chan struct{}, 1),
	}
}

func (n *asyncReadyNode) Step(_ context.Context, m *raftpb.Message) error {
	n.stepc <- m
	return nil
}

func (n *asyncReadyNode) Advance() {
	n.advancec <- struct{}{}
}

// stepOrderNode records, for each stepped message, whether the watched
// channel was closed at that time.
type stepOrderNode struct {
	*asyncReadyNode
	mu       sync.Mutex
	watchedC <-chan struct{}
	closed   []bool
}

func (n *stepOrderNode) watch(c <-chan struct{}) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.watchedC = c
}

func (n *stepOrderNode) watchClosedAtStep() []bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.closed
}

func (n *stepOrderNode) Step(ctx context.Context, m *raftpb.Message) error {
	n.mu.Lock()
	if n.watchedC != nil {
		select {
		case <-n.watchedC:
			n.closed = append(n.closed, true)
		default:
			n.closed = append(n.closed, false)
		}
	}
	n.mu.Unlock()
	return n.asyncReadyNode.Step(ctx, m)
}

// blockingStepNode blocks Step until its context is canceled.
type blockingStepNode struct {
	*asyncReadyNode
	stepStartedC chan struct{}
}

func newBlockingStepNode() *blockingStepNode {
	return &blockingStepNode{
		asyncReadyNode: newAsyncReadyNode(),
		stepStartedC:   make(chan struct{}),
	}
}

func (n *blockingStepNode) Step(ctx context.Context, _ *raftpb.Message) error {
	close(n.stepStartedC)
	<-ctx.Done()
	return ctx.Err()
}

// blockingSaveStorage signals saveStartedC, unless it is full, and waits for
// allowSaveC on every Save.
type blockingSaveStorage struct {
	serverstorage.Storage
	saveStartedC chan<- struct{}
	allowSaveC   <-chan struct{}
}

func (s *blockingSaveStorage) Save(st *raftpb.HardState, entries []*raftpb.Entry) error {
	select {
	case s.saveStartedC <- struct{}{}:
	default:
	}
	<-s.allowSaveC
	return s.Storage.Save(st, entries)
}

// saveCaptureStorage records the number of Save calls and the last saved
// HardState and entries.
type saveCaptureStorage struct {
	serverstorage.Storage
	saveCount int
	hardState *raftpb.HardState
	entries   []*raftpb.Entry
}

func (s *saveCaptureStorage) Save(st *raftpb.HardState, entries []*raftpb.Entry) error {
	s.saveCount++
	s.hardState = &raftpb.HardState{Term: new(st.GetTerm()), Vote: new(st.GetVote()), Commit: new(st.GetCommit())}
	s.entries = append([]*raftpb.Entry(nil), entries...)
	return s.Storage.Save(st, entries)
}

// savedSnapshot is a snapshot saved to the WAL, with the highest commit index
// saved to the WAL before it.
type savedSnapshot struct {
	index       uint64
	savedCommit uint64
}

// snapshotRecordingStorage records the snapshots saved to the WAL.
type snapshotRecordingStorage struct {
	serverstorage.Storage
	// afterSaveSnap, if set, is called after each snapshot is saved.
	afterSaveSnap func(index uint64)

	mu          sync.Mutex
	savedCommit uint64
	snapshots   []savedSnapshot
}

func (s *snapshotRecordingStorage) Save(st *raftpb.HardState, entries []*raftpb.Entry) error {
	if err := s.Storage.Save(st, entries); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.savedCommit = max(s.savedCommit, st.GetCommit())
	return nil
}

func (s *snapshotRecordingStorage) SaveSnap(snapshot *raftpb.Snapshot) error {
	if err := s.Storage.SaveSnap(snapshot); err != nil {
		return err
	}
	s.mu.Lock()
	s.snapshots = append(s.snapshots, savedSnapshot{index: snapshot.Metadata.GetIndex(), savedCommit: s.savedCommit})
	s.mu.Unlock()
	if s.afterSaveSnap != nil {
		s.afterSaveSnap(snapshot.Metadata.GetIndex())
	}
	return nil
}

func (s *snapshotRecordingStorage) saved() []savedSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]savedSnapshot(nil), s.snapshots...)
}

// recordingTransporter records the messages sent to peers.
type recordingTransporter struct {
	nopTransporter
	sentc chan []*raftpb.Message
}

func newRecordingTransporter() *recordingTransporter {
	return &recordingTransporter{sentc: make(chan []*raftpb.Message, 8)}
}

func (s *recordingTransporter) Send(m []*raftpb.Message) {
	// The Ready loop sends once per Ready, even without messages.
	if len(m) != 0 {
		s.sentc <- m
	}
}
