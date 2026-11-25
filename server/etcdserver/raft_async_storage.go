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
	"sync"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

// This file contains the asynchronous storage write path of the Ready loop,
// used when the RaftAsyncStorageWrites feature gate enables
// raft.Config.AsyncStorageWrites.
//
// Without it, the Ready loop persists every Ready, hands its committed entries
// to the applier and calls Advance before Raft emits the next Ready. With it,
// Raft does not wait for Advance. Each Ready carries local storage messages
// instead, and Raft only moves forward once their responses are delivered:
//
//   - MsgStorageAppend asks to persist entries, the HardState and an incoming
//     snapshot. The append worker persists these in order, and delivers the
//     responses, such as MsgStorageAppendResp to the local node or MsgAppResp
//     to the leader, only once the WAL and MemoryStorage are updated.
//   - MsgStorageApply asks to apply committed entries. They reach the applier
//     through applyc, like on the synchronous path, and onApplyCompleted
//     delivers the MsgStorageApplyResp once they are applied.
//
// Appending and applying are not ordered against each other. Raft only asks
// to apply entries after it has received their MsgStorageAppendResp, so they
// are already in the WAL and in MemoryStorage. Only the WAL commit index can
// lag behind what the backend has applied:
//
//   - After a crash, the backend consistent index can be ahead of the WAL
//     commit index, which the synchronous path does not prevent either. The
//     entries are committed and durable, and entries up to the consistent
//     index are not applied to the backend again.
//   - On shutdown, the queued appends are persisted before the WAL is closed.
//   - A snapshot saved to the WAL waits for the commit index to be saved, as
//     Raft cannot restart from a snapshot ahead of it, see waitForCommitSaved.
//
// An incoming snapshot is handed to the applier before it is persisted. Until
// the applier has installed it, the append worker keeps persisting the appends
// that follow it and acknowledging them to the leader, but holds back the
// responses to the local node: Raft considers the snapshot applied once it
// gets its MsgStorageAppendResp. Another incoming snapshot waits for the
// install, as persisting it releases the .snap.db files of the earlier ones.

// asyncAppendQueueSize bounds the MsgStorageAppend messages queued for the
// append worker, which saves contiguous queued appends with a single WAL
// write. It is a performance tuning value, not a Raft protocol limit.
const asyncAppendQueueSize = 64

// appendWorker is the state of the goroutine that acts as Raft's
// LocalAppendThread: it persists MsgStorageAppend messages in order, to the WAL
// and MemoryStorage, and delivers their responses.
type appendWorker struct {
	// appendc carries MsgStorageAppend messages to the append worker in order.
	appendc chan *raftpb.Message
	// stopc asks the append worker to persist the queued appends and exit.
	stopc chan struct{}
	// donec is closed when the append worker exits.
	donec chan struct{}
	// responseCtx is canceled on stop, so that delivering a storage response
	// to the Raft node does not block shutdown.
	responseCtx    context.Context
	cancelResponse context.CancelFunc

	commitMu sync.Mutex
	// savedCommit is the highest commit index saved to the WAL.
	savedCommit uint64
	// savedCommitC is closed and replaced when savedCommit advances.
	savedCommitC chan struct{}

	// installing is the incoming snapshot the applier is installing, if any.
	// Only the append worker goroutine uses it.
	installing *snapshotInstall
}

// snapshotInstall is an incoming snapshot handed to the applier.
type snapshotInstall struct {
	// installedC is closed once the applier has installed the snapshot.
	installedC chan struct{}
	// responses are the responses to the local node held until then, in order,
	// starting with the MsgStorageAppendResp of the snapshot.
	responses []*raftpb.Message
}

func newAppendWorker(savedCommit uint64) *appendWorker {
	responseCtx, cancelResponse := context.WithCancel(context.Background())
	return &appendWorker{
		appendc:        make(chan *raftpb.Message, asyncAppendQueueSize),
		stopc:          make(chan struct{}),
		donec:          make(chan struct{}),
		responseCtx:    responseCtx,
		cancelResponse: cancelResponse,
		savedCommit:    savedCommit,
		savedCommitC:   make(chan struct{}),
	}
}

func (w *appendWorker) setSavedCommit(hardState *raftpb.HardState) {
	if raft.IsEmptyHardState(hardState) {
		return
	}
	w.commitMu.Lock()
	defer w.commitMu.Unlock()
	if hardState.GetCommit() > w.savedCommit {
		w.savedCommit = hardState.GetCommit()
		close(w.savedCommitC)
		w.savedCommitC = make(chan struct{})
	}
}

// waitForCommitSaved blocks until the WAL records a commit index of at least
// index, or the Raft node stops. Raft fails to restart from a snapshot saved
// to the WAL ahead of the WAL commit index, and with asynchronous storage
// writes the applier can get ahead of it. The HardState that catches up was
// queued before the applied entries were handed to the applier, so it is not
// queued behind anything that waits for the applier. After --force-new-cluster,
// the commit index Raft starts from is only saved with Raft's next HardState,
// so the first snapshot is not covered, like on the synchronous path.
// It returns right away without asynchronous storage writes.
func (r *raftNode) waitForCommitSaved(index uint64) {
	if !r.asyncStorageWrites {
		return
	}
	w := r.appendWorker
	for {
		w.commitMu.Lock()
		savedCommit, savedCommitC := w.savedCommit, w.savedCommitC
		w.commitMu.Unlock()
		if savedCommit >= index {
			return
		}
		select {
		case <-savedCommitC:
		case <-w.stopc:
			return
		}
	}
}

// processAsyncReady dispatches a Ready without persisting or applying it:
// MsgStorageAppend goes to the append worker, MsgStorageApply goes to the
// applier, and the other messages are sent right away, since Raft attaches
// the messages that must wait for a storage write to the local storage
// messages as responses. It returns false if the Raft node is stopping.
func (r *raftNode) processAsyncReady(rd raft.Ready, rh *raftReadyHandler) bool {
	// Send before handing off the storage work, which can block. Like on the
	// synchronous path, send once per Ready even without messages, so that the
	// send failpoints fire on a single member cluster too.
	var msgs []*raftpb.Message
	for _, m := range rd.Messages {
		if !raft.IsLocalMsgTarget(m.GetTo()) {
			msgs = append(msgs, m)
		}
	}
	r.send(r.processMessages(msgs))

	for _, m := range rd.Messages {
		switch m.GetTo() {
		case raft.LocalAppendThread:
			// Track the commit index of Raft, which can be far ahead of the
			// entries handed to the applier: Raft limits those to
			// MaxCommittedSizePerReady until they are applied.
			if commit := max(m.GetCommit(), m.GetSnapshot().GetMetadata().GetIndex()); commit != 0 {
				rh.updateCommittedIndex(commit)
			}
			select {
			case r.appendWorker.appendc <- m:
			case <-r.stopped:
				return false
			}
		case raft.LocalApplyThread:
			select {
			case r.applyc <- r.newAsyncToApply(m):
			case <-r.stopped:
				return false
			}
		default:
			if raft.IsLocalMsgTarget(m.GetTo()) {
				r.lg.Panic(
					"unexpected message to local Raft storage thread",
					zap.Stringer("message-type", m.GetType()),
					zap.Uint64("message-to", m.GetTo()),
				)
			}
		}
	}
	return true
}

// newAsyncToApply returns the toApply of a MsgStorageApply. Its entries are
// already in the WAL and in MemoryStorage, so notifyc does not block.
func (r *raftNode) newAsyncToApply(m *raftpb.Message) toApply {
	responses := m.GetResponses()
	raftAdvancedC := make(chan struct{})
	return toApply{
		entries:       m.GetEntries(),
		snapshot:      raftpb.EnsureSnapshot(nil),
		notifyc:       closedNotifyc(),
		raftAdvancedC: raftAdvancedC,
		onApplyCompleted: func() {
			r.sendStorageResponses(responses)
			// MsgStorageApplyResp has advanced 'raftLog.applied'. Close, so that
			// every configuration change of the batch is notified.
			close(raftAdvancedC)
		},
	}
}

// closedNotifyc returns a notifyc for a toApply whose Raft storage work is
// already done, so that applyAll does not wait for the raft node.
func closedNotifyc() chan struct{} {
	notifyc := make(chan struct{})
	close(notifyc)
	return notifyc
}

// runAppendWorker is the append worker. It persists MsgStorageAppend messages
// in the order Raft emitted them, and delivers the responses of each message
// only once it is persisted.
func (r *raftNode) runAppendWorker() {
	w := r.appendWorker
	defer close(w.donec)
	for {
		select {
		case <-w.stopc:
			r.drainAppends()
			return
		default:
		}

		var installedC <-chan struct{}
		if w.installing != nil {
			installedC = w.installing.installedC
		}
		var msgs []*raftpb.Message
		select {
		case m := <-w.appendc:
			msgs = append(msgs, m)
		case <-installedC:
			r.finishSnapshotInstall()
			continue
		case <-w.stopc:
			r.drainAppends()
			return
		}
		// Take the other queued appends, so that contiguous ones share a single
		// WAL write. The worker is the only receiver, so this does not block.
		for len(msgs) < asyncAppendQueueSize && len(w.appendc) > 0 {
			msgs = append(msgs, <-w.appendc)
		}
		if !r.processStorageAppends(msgs) {
			return
		}
	}
}

// processStorageAppends persists MsgStorageAppend messages in order and
// delivers their responses. It returns false if the Raft node stopped before
// an incoming snapshot was persisted.
func (r *raftNode) processStorageAppends(msgs []*raftpb.Message) bool {
	for len(msgs) > 0 {
		if !raft.IsEmptySnap(msgs[0].GetSnapshot()) {
			if !r.waitForSnapshotInstall() || !r.processSnapshotAppend(msgs[0]) {
				return false
			}
			msgs = msgs[1:]
			continue
		}

		// Save the following appends together as long as their entries extend
		// the ones before. A later HardState supersedes an earlier one, so this
		// is equivalent to saving the appends one by one.
		n := 1
		entries := append([]*raftpb.Entry(nil), msgs[0].GetEntries()...)
		for n < len(msgs) && raft.IsEmptySnap(msgs[n].GetSnapshot()) && extendsEntries(entries, msgs[n].GetEntries()) {
			entries = append(entries, msgs[n].GetEntries()...)
			n++
		}
		var hardState *raftpb.HardState
		var responses []*raftpb.Message
		for _, m := range msgs[:n] {
			if hs := hardStateFromAppend(r.lg, m); hs != nil {
				hardState = hs
			}
			responses = append(responses, m.GetResponses()...)
		}
		r.persistRaftData(hardState, entries, nil, nil)
		r.appendWorker.setSavedCommit(hardState)
		r.sendAppendResponses(responses)
		msgs = msgs[n:]
	}
	return true
}

// extendsEntries returns true if next can be appended to entries without
// overwriting any of them.
func extendsEntries(entries, next []*raftpb.Entry) bool {
	return len(entries) == 0 || len(next) == 0 || next[0].GetIndex() == entries[len(entries)-1].GetIndex()+1
}

// processSnapshotAppend hands an incoming snapshot to the applier and persists
// it. The responses to the local node are held until the applier installed it.
// It returns false if the Raft node stopped before the hand-off.
func (r *raftNode) processSnapshotAppend(m *raftpb.Message) bool {
	w := r.appendWorker
	// Hand the snapshot to the applier before persisting it, like the
	// synchronous path does: once the WAL records the snapshot, the backend
	// has to install it.
	notifyc := make(chan struct{})
	installedC := make(chan struct{})
	ap := toApply{
		snapshot:         proto.Clone(m.GetSnapshot()).(*raftpb.Snapshot),
		notifyc:          notifyc,
		onApplyCompleted: func() { close(installedC) },
	}
	select {
	case r.applyc <- ap:
	case <-w.stopc:
		return false
	}

	hardState := hardStateFromAppend(r.lg, m)
	r.persistRaftData(hardState, m.GetEntries(), proto.Clone(m.GetSnapshot()).(*raftpb.Snapshot), nil)
	w.setSavedCommit(hardState)
	// The applier opens the snapshot backend, and may then create a snapshot
	// of, or compact, MemoryStorage, once the storage work is done.
	close(notifyc)

	// Raft emits committed entries after the snapshot only once it receives
	// the MsgStorageAppendResp, so the applier gets them after the snapshot.
	w.installing = &snapshotInstall{installedC: installedC}
	r.sendAppendResponses(m.GetResponses())
	return true
}

// waitForSnapshotInstall waits until the applier has installed the incoming
// snapshot handed to it last, if any, and delivers the responses held for it.
// It returns false if the Raft node stopped first.
func (r *raftNode) waitForSnapshotInstall() bool {
	w := r.appendWorker
	if w.installing == nil {
		return true
	}
	select {
	case <-w.installing.installedC:
		r.finishSnapshotInstall()
		return true
	case <-w.stopc:
		return false
	}
}

// finishSnapshotInstall delivers the responses held while the applier was
// installing an incoming snapshot.
func (r *raftNode) finishSnapshotInstall() {
	responses := r.appendWorker.installing.responses
	r.appendWorker.installing = nil
	r.sendStorageResponses(responses)
}

// sendAppendResponses delivers the responses of persisted MsgStorageAppend
// messages, holding the ones to the local node while an incoming snapshot is
// being installed.
func (r *raftNode) sendAppendResponses(responses []*raftpb.Message) {
	installing := r.appendWorker.installing
	if installing == nil {
		r.sendStorageResponses(responses)
		return
	}
	var msgs []*raftpb.Message
	for _, m := range responses {
		if m.GetTo() == r.localID {
			installing.responses = append(installing.responses, m)
		} else {
			msgs = append(msgs, m)
		}
	}
	r.sendStorageResponses(msgs)
}

// hardStateFromAppend returns the HardState carried by a MsgStorageAppend,
// or nil if it carries none.
func hardStateFromAppend(lg *zap.Logger, m *raftpb.Message) *raftpb.HardState {
	if m.Term == nil && m.Vote == nil && m.Commit == nil {
		return nil
	}
	if m.Term == nil || m.Vote == nil || m.Commit == nil {
		lg.Panic("incomplete hard state in Raft storage append", zap.Stringer("message", m))
	}
	return &raftpb.HardState{Term: m.Term, Vote: m.Vote, Commit: m.Commit}
}

// sendStorageResponses delivers the responses of a local storage message once
// its storage work is done. Responses to peers are sent first, then the ones
// to the local node are stepped into it in order.
func (r *raftNode) sendStorageResponses(responses []*raftpb.Message) {
	var msgs []*raftpb.Message
	for _, m := range responses {
		if m.GetTo() != r.localID {
			msgs = append(msgs, m)
		}
	}
	if len(msgs) != 0 {
		r.send(r.processMessages(msgs))
	}
	for _, m := range responses {
		if m.GetTo() != r.localID {
			continue
		}
		if err := r.Step(r.appendWorker.responseCtx, m); err != nil {
			// Step only fails once the Raft node is stopping.
			r.lg.Debug("dropped local Raft storage response", zap.Stringer("message-type", m.GetType()), zap.Error(err))
		}
	}
}

// drainAppends persists the appends left on appendc once the Ready loop has
// stopped, without delivering their responses. The applier may already have
// applied, and the backend persisted, entries whose commit index is only in
// this queue; persisting it before the WAL is closed keeps the WAL commit
// index at or ahead of the backend consistent index, which verify checks.
// Draining stops at an incoming snapshot: it can no longer be handed to the
// applier, and Raft emits no entries to apply beyond it until it receives the
// snapshot's MsgStorageAppendResp.
func (r *raftNode) drainAppends() {
	w := r.appendWorker
	for len(w.appendc) > 0 {
		m := <-w.appendc
		if !raft.IsEmptySnap(m.GetSnapshot()) {
			return
		}
		hardState := hardStateFromAppend(r.lg, m)
		r.persistRaftData(hardState, m.GetEntries(), nil, nil)
		w.setSavedCommit(hardState)
	}
}

// stopAppendWorker stops the append worker once it has persisted the queued
// appends. It must be called after the Ready loop exits.
func (r *raftNode) stopAppendWorker() {
	w := r.appendWorker
	// Close stopc first, so that a worker unblocked by the cancellation sees it.
	close(w.stopc)
	w.cancelResponse()
	<-w.donec
}
