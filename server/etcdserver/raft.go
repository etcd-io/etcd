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

package etcdserver

import (
	"context"
	"expvar"
	"fmt"
	"log"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/contention"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/raft/v3"
	"go.etcd.io/raft/v3/raftpb"
)

const (
	// The max throughput of etcd will not exceed 100MB/s (100K * 1KB value).
	// Assuming the RTT is around 10ms, 1MB max size is large enough.
	maxSizePerMsg = 1 * 1024 * 1024
	// Never overflow the rafthttp buffer, which is 4096.
	// TODO: a better const?
	maxInflightMsgs = 4096 / 8
	// Bound the number of append messages accepted ahead of stable storage.
	// The append worker coalesces compatible messages into a single WAL sync.
	asyncAppendQueueSize = 64
)

var (
	// protects raftStatus
	raftStatusMu sync.Mutex
	// indirection for expvar func interface
	// expvar panics when publishing duplicate name
	// expvar does not support remove a registered name
	// so only register a func that calls raftStatus
	// and change raftStatus as we need.
	raftStatus func() raft.Status
)

func init() {
	expvar.Publish("raft.status", expvar.Func(func() any {
		raftStatusMu.Lock()
		defer raftStatusMu.Unlock()
		if raftStatus == nil {
			return nil
		}
		return raftStatus()
	}))
}

// toApply contains entries and a snapshot to apply. The application must read
// notifyc before assuming that the corresponding Raft storage work is complete.
type toApply struct {
	entries  []*raftpb.Entry
	snapshot *raftpb.Snapshot
	// notifyc synchronizes etcd server applies with the raft node
	notifyc chan struct{}
	// raftAdvancedC notifies EtcdServer.apply that raftLog.applied has advanced
	// through either r.Advance or a MsgStorageApplyResp.
	// it should be used only when entries contain raftpb.EntryConfChange
	raftAdvancedC <-chan struct{}
	// onApplied is called after the state machine work and its corresponding
	// Raft storage work have both completed.
	onApplied func()
}

type raftNode struct {
	lg *zap.Logger

	tickMu *sync.RWMutex
	// timestamp of the latest tick
	latestTickTs time.Time
	raftNodeConfig

	// a chan to send/receive snapshot
	msgSnapC chan *raftpb.Message

	// a chan to send out apply
	applyc chan toApply
	// appendc carries the reliable FIFO stream of MsgStorageAppend work when
	// asynchronous storage writes are enabled.
	appendc chan *raftpb.Message
	// asyncStopc stops asynchronous storage work before Raft storage is closed.
	asyncStopc       chan struct{}
	asyncAppendDonec chan struct{}

	// a chan to send out readState
	readStateC chan raft.ReadState

	// utility
	ticker *time.Ticker
	// contention detectors for raft heartbeat message
	td *contention.TimeoutDetector

	stopped chan struct{}
	done    chan struct{}
}

type raftNodeConfig struct {
	lg *zap.Logger

	// to check if msg receiver is removed from cluster
	isIDRemoved func(id uint64) bool
	raft.Node
	localID            uint64
	asyncStorageWrites bool
	raftStorage        *raft.MemoryStorage
	storage            serverstorage.Storage
	heartbeat          time.Duration // for logging
	// transport specifies the transport to send and receive msgs to members.
	// Sending messages MUST NOT block. It is okay to drop messages, since
	// clients should timeout and reissue their messages.
	// If transport is nil, server will panic.
	transport rafthttp.Transporter
}

func newRaftNode(cfg raftNodeConfig) *raftNode {
	var lg raft.Logger
	if cfg.lg != nil {
		lg = NewRaftLoggerZap(cfg.lg)
	} else {
		lcfg := logutil.DefaultZapLoggerConfig
		var err error
		lg, err = NewRaftLogger(&lcfg)
		if err != nil {
			log.Fatalf("cannot create raft logger %v", err)
		}
	}
	raft.SetLogger(lg)
	r := &raftNode{
		lg:             cfg.lg,
		tickMu:         new(sync.RWMutex),
		raftNodeConfig: cfg,
		latestTickTs:   time.Now(),
		// set up contention detectors for raft heartbeat message.
		// expect to send a heartbeat within 2 heartbeat intervals.
		td:               contention.NewTimeoutDetector(2 * cfg.heartbeat),
		readStateC:       make(chan raft.ReadState, 1),
		msgSnapC:         make(chan *raftpb.Message, maxInFlightMsgSnap),
		applyc:           make(chan toApply),
		appendc:          make(chan *raftpb.Message, asyncAppendQueueSize),
		asyncStopc:       make(chan struct{}),
		asyncAppendDonec: make(chan struct{}),
		stopped:          make(chan struct{}),
		done:             make(chan struct{}),
	}
	if r.heartbeat == 0 {
		r.ticker = &time.Ticker{}
	} else {
		r.ticker = time.NewTicker(r.heartbeat)
	}
	return r
}

// raft.Node does not have locks in Raft package
func (r *raftNode) tick() {
	r.tickMu.Lock()
	r.Tick()
	r.latestTickTs = time.Now()
	r.tickMu.Unlock()
}

func (r *raftNode) getLatestTickTs() time.Time {
	r.tickMu.RLock()
	defer r.tickMu.RUnlock()
	return r.latestTickTs
}

// start prepares and starts raftNode in a new goroutine. It is no longer safe
// to modify the fields after it has been started.
func (r *raftNode) start(rh *raftReadyHandler) {
	internalTimeout := time.Second

	if r.asyncStorageWrites {
		go r.runAsyncAppend()
	}
	go func() {
		defer func() {
			if r.asyncStorageWrites {
				close(r.asyncStopc)
				<-r.asyncAppendDonec
			}
			r.onStop()
		}()
		islead := false

		for {
			select {
			case <-r.ticker.C:
				r.tick()
			case rd := <-r.Ready():
				if rd.SoftState != nil {
					newLeader := rd.SoftState.Lead != raft.None && rh.getLead() != rd.SoftState.Lead
					if newLeader {
						leaderChanges.Inc()
					}

					if rd.SoftState.Lead == raft.None {
						hasLeader.Set(0)
					} else {
						hasLeader.Set(1)
					}

					rh.updateLead(rd.SoftState.Lead)
					islead = rd.RaftState == raft.StateLeader
					if islead {
						isLeader.Set(1)
					} else {
						isLeader.Set(0)
					}
					rh.updateLeadership(newLeader)
					r.td.Reset()
				}

				if len(rd.ReadStates) != 0 {
					select {
					case r.readStateC <- rd.ReadStates[len(rd.ReadStates)-1]:
					case <-time.After(internalTimeout):
						r.lg.Warn("timed out sending read state", zap.Duration("timeout", internalTimeout))
					case <-r.stopped:
						return
					}
				}
				if r.asyncStorageWrites {
					if !r.processAsyncReady(rd, rh) {
						return
					}
					continue
				}
				committedEntries := rd.CommittedEntries
				notifyc := make(chan struct{}, 1)
				raftAdvancedC := make(chan struct{}, 1)
				raftSnap := proto.Clone(rd.Snapshot).(*raftpb.Snapshot)
				ap := toApply{
					entries:       committedEntries,
					snapshot:      proto.Clone(rd.Snapshot).(*raftpb.Snapshot),
					notifyc:       notifyc,
					raftAdvancedC: raftAdvancedC,
				}

				updateCommittedIndex(&ap, rh)

				select {
				case r.applyc <- ap:
				case <-r.stopped:
					return
				}

				// the leader can write to its disk in parallel with replicating to the followers and then
				// writing to their disks.
				// For more details, check raft thesis 10.2.1
				if islead {
					// gofail: var raftBeforeLeaderSend struct{}
					r.transport.Send(r.processMessages(rd.Messages))
				}

				r.persistRaftData(rd.HardState, rd.Entries, raftSnap, notifyc)

				confChanged := false
				for _, ent := range rd.CommittedEntries {
					if ent.GetType() == raftpb.EntryConfChange {
						confChanged = true
						break
					}
				}

				if !islead {
					// finish processing incoming messages before we signal notifyc chan
					msgs := r.processMessages(rd.Messages)

					// now unblocks 'applyAll' that waits on Raft log disk writes before triggering snapshots
					notifyc <- struct{}{}

					// Candidate or follower needs to wait for all pending configuration
					// changes to be applied before sending messages.
					// Otherwise we might incorrectly count votes (e.g. votes from removed members).
					// Also slow machine's follower raft-layer could proceed to become the leader
					// on its own single-node cluster, before toApply-layer applies the config change.
					// We simply wait for ALL pending entries to be applied for now.
					// We might improve this later on if it causes unnecessary long blocking issues.

					if confChanged {
						// blocks until 'applyAll' calls 'applyWait.Trigger'
						// to be in sync with scheduled config-change job
						// (assume notifyc has cap of 1)
						select {
						case notifyc <- struct{}{}:
						case <-r.stopped:
							return
						}
					}

					// gofail: var raftBeforeFollowerSend struct{}
					r.transport.Send(msgs)
				} else {
					// leader already processed 'MsgSnap' and signaled
					notifyc <- struct{}{}
				}

				// gofail: var raftBeforeAdvance struct{}
				r.Advance()

				if confChanged {
					// notify etcdserver that raft has already been notified or advanced.
					raftAdvancedC <- struct{}{}
				}
			case <-r.stopped:
				return
			}
		}
	}()
}

// processAsyncReady dispatches Raft's local storage messages to two reliable
// FIFO lanes. All other messages are safe to send immediately; messages whose
// delivery depends on a storage write are carried as responses on the local
// storage messages instead.
func (r *raftNode) processAsyncReady(rd raft.Ready, rh *raftReadyHandler) bool {
	var messages []*raftpb.Message
	sendMessages := func() {
		if len(messages) == 0 {
			return
		}
		r.transport.Send(r.processMessages(messages))
		messages = nil
	}
	for _, m := range rd.Messages {
		switch m.GetTo() {
		case raft.LocalAppendThread:
			if m.GetType() != raftpb.MsgStorageAppend {
				r.lg.Panic("unexpected message to Raft append thread", zap.Stringer("message-type", m.GetType()))
			}
			if snap := m.GetSnapshot(); !raft.IsEmptySnap(snap) && rh != nil {
				updateCommittedIndex(&toApply{snapshot: snap}, rh)
			}
			sendMessages()
			select {
			case r.appendc <- m:
			case <-r.stopped:
				return false
			}
		case raft.LocalApplyThread:
			if m.GetType() != raftpb.MsgStorageApply {
				r.lg.Panic("unexpected message to Raft apply thread", zap.Stringer("message-type", m.GetType()))
			}
			notifyc := make(chan struct{}, 1)
			notifyc <- struct{}{}
			raftAdvancedC := make(chan struct{}, 1)
			responses := m.GetResponses()
			ap := toApply{
				entries:       m.GetEntries(),
				snapshot:      raftpb.EnsureSnapshot(nil),
				notifyc:       notifyc,
				raftAdvancedC: raftAdvancedC,
				onApplied: func() {
					r.sendStorageResponses(responses)
					raftAdvancedC <- struct{}{}
				},
			}
			if rh != nil {
				updateCommittedIndex(&ap, rh)
			}
			sendMessages()
			select {
			case r.applyc <- ap:
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
			messages = append(messages, m)
		}
	}
	sendMessages()
	return true
}

func (r *raftNode) runAsyncAppend() {
	defer close(r.asyncAppendDonec)
	for {
		select {
		case <-r.asyncStopc:
			return
		default:
		}
		select {
		case m := <-r.appendc:
			messages := []*raftpb.Message{m}
			draining := true
			for draining && len(messages) < asyncAppendQueueSize {
				select {
				case m = <-r.appendc:
					messages = append(messages, m)
				default:
					draining = false
				}
			}
			r.processStorageAppends(messages)
		case <-r.asyncStopc:
			return
		}
	}
}

func (r *raftNode) processStorageAppends(messages []*raftpb.Message) {
	for len(messages) > 0 {
		if !raft.IsEmptySnap(messages[0].GetSnapshot()) {
			r.processStorageAppend(messages[0])
			messages = messages[1:]
			continue
		}

		end := 1
		var entries []*raftpb.Entry
		entries = append(entries, messages[0].GetEntries()...)
		for end < len(messages) && raft.IsEmptySnap(messages[end].GetSnapshot()) && canBatchStorageAppend(entries, messages[end].GetEntries()) {
			entries = append(entries, messages[end].GetEntries()...)
			end++
		}
		r.processStorageAppendBatch(messages[:end], entries)
		messages = messages[end:]
	}
}

func canBatchStorageAppend(entries, next []*raftpb.Entry) bool {
	return len(entries) == 0 || len(next) == 0 || next[0].GetIndex() == entries[len(entries)-1].GetIndex()+1
}

func (r *raftNode) processStorageAppendBatch(messages []*raftpb.Message, entries []*raftpb.Entry) {
	var hardState *raftpb.HardState
	var responses []*raftpb.Message
	for _, m := range messages {
		if m.GetType() != raftpb.MsgStorageAppend {
			r.lg.Panic("unexpected Raft append work", zap.Stringer("message-type", m.GetType()))
		}
		if state := hardStateFromAppendMessage(r.lg, m); state != nil {
			hardState = state
		}
		responses = append(responses, m.GetResponses()...)
	}
	// A later HardState supersedes an earlier one, and processStorageAppends
	// only combines entry ranges that form one contiguous append. Persisting
	// that final state is therefore equivalent to persisting each message in
	// order. All responses remain blocked until the combined save is durable.
	r.persistRaftData(hardState, entries, raftpb.EnsureSnapshot(nil), nil)
	r.sendStorageResponses(responses)
}

func (r *raftNode) processStorageAppend(m *raftpb.Message) {
	if m.GetType() != raftpb.MsgStorageAppend {
		r.lg.Panic("unexpected Raft append work", zap.Stringer("message-type", m.GetType()))
	}

	hardState := hardStateFromAppendMessage(r.lg, m)

	var raftSnap *raftpb.Snapshot
	var applySnap *raftpb.Snapshot
	if !raft.IsEmptySnap(m.GetSnapshot()) {
		raftSnap = proto.Clone(m.GetSnapshot()).(*raftpb.Snapshot)
		applySnap = proto.Clone(m.GetSnapshot()).(*raftpb.Snapshot)
	} else {
		raftSnap = raftpb.EnsureSnapshot(nil)
	}

	var notifyc chan struct{}
	var snapshotAppliedC chan struct{}
	if !raft.IsEmptySnap(raftSnap) {
		notifyc = make(chan struct{}, 1)
		snapshotAppliedC = make(chan struct{})
		responses := m.GetResponses()
		ap := toApply{
			snapshot:      applySnap,
			notifyc:       notifyc,
			raftAdvancedC: make(chan struct{}),
			onApplied: func() {
				r.sendStorageResponses(responses)
				close(snapshotAppliedC)
			},
		}
		select {
		case r.applyc <- ap:
		case <-r.asyncStopc:
			return
		}
	}

	r.persistRaftData(hardState, m.GetEntries(), raftSnap, notifyc)
	if raft.IsEmptySnap(raftSnap) {
		r.sendStorageResponses(m.GetResponses())
		return
	}

	// applySnapshot consumes the first notification sent by persistRaftData.
	// The second notification allows applyAll to observe that the in-memory
	// Raft storage update and WAL release are also complete.
	select {
	case notifyc <- struct{}{}:
	case <-r.asyncStopc:
		return
	}
	select {
	case <-snapshotAppliedC:
	case <-r.asyncStopc:
	}
}

func hardStateFromAppendMessage(lg *zap.Logger, m *raftpb.Message) *raftpb.HardState {
	if m.Term == nil && m.Vote == nil && m.Commit == nil {
		return nil
	}
	if m.Term == nil || m.Vote == nil || m.Commit == nil {
		lg.Panic("incomplete hard state on Raft append work")
	}
	return &raftpb.HardState{
		Term:   m.Term,
		Vote:   m.Vote,
		Commit: m.Commit,
	}
}

// persistRaftData preserves etcd's crash-recovery ordering for both the
// Ready/Advance and asynchronous storage-write interfaces.
func (r *raftNode) persistRaftData(
	hardState *raftpb.HardState,
	entries []*raftpb.Entry,
	raftSnap *raftpb.Snapshot,
	snapshotPersistedC chan<- struct{},
) {
	// Must save the snapshot file and WAL snapshot entry before saving any other entries or hardstate to
	// ensure that recovery after a snapshot restore is possible.
	if !raft.IsEmptySnap(raftSnap) {
		// gofail: var raftBeforeSaveSnap struct{}
		if err := r.storage.SaveSnap(raftSnap); err != nil {
			r.lg.Fatal("failed to save Raft snapshot", zap.Error(err))
		}
		// gofail: var raftAfterSaveSnap struct{}
	}

	// gofail: var raftBeforeSave struct{}
	if err := r.storage.Save(hardState, entries); err != nil {
		r.lg.Fatal("failed to save Raft hard state and entries", zap.Error(err))
	}
	if !raft.IsEmptyHardState(hardState) {
		proposalsCommitted.Set(float64(hardState.GetCommit()))
	}
	// gofail: var raftAfterSave struct{}

	if !raft.IsEmptySnap(raftSnap) {
		// Force WAL to fsync its hard state before Release() releases
		// old data from the WAL. Otherwise could get an error like:
		// panic: tocommit(107) is out of range [lastIndex(84)]. Was the raft log corrupted, truncated, or lost?
		// See https://github.com/etcd-io/etcd/issues/10219 for more details.
		if err := r.storage.Sync(); err != nil {
			r.lg.Fatal("failed to sync Raft snapshot", zap.Error(err))
		}

		// etcdserver now claims the snapshot has been persisted onto the disk.
		snapshotPersistedC <- struct{}{}

		// gofail: var raftBeforeApplySnap struct{}
		r.raftStorage.ApplySnapshot(raftSnap)
		r.lg.Info("applied incoming Raft snapshot", zap.Uint64("snapshot-index", raftSnap.Metadata.GetIndex()))
		// gofail: var raftAfterApplySnap struct{}

		if err := r.storage.Release(raftSnap); err != nil {
			r.lg.Fatal("failed to release Raft wal", zap.Error(err))
		}
		// gofail: var raftAfterWALRelease struct{}
	}

	r.raftStorage.Append(entries)
}

func (r *raftNode) sendStorageResponses(responses []*raftpb.Message) {
	var messages []*raftpb.Message
	sendMessages := func() {
		if len(messages) == 0 {
			return
		}
		r.transport.Send(r.processMessages(messages))
		messages = nil
	}
	for _, m := range responses {
		if m.GetTo() == r.localID {
			sendMessages()
			if err := r.Step(context.Background(), m); err != nil {
				r.lg.Warn("failed to deliver local Raft storage response", zap.Stringer("message-type", m.GetType()), zap.Error(err))
			}
			continue
		}
		messages = append(messages, m)
	}
	sendMessages()
}

func updateCommittedIndex(ap *toApply, rh *raftReadyHandler) {
	var ci uint64
	if len(ap.entries) != 0 {
		ci = ap.entries[len(ap.entries)-1].GetIndex()
	}
	if ap.snapshot != nil && ap.snapshot.Metadata.GetIndex() > ci {
		ci = ap.snapshot.Metadata.GetIndex()
	}
	if ci != 0 {
		rh.updateCommittedIndex(ci)
	}
}

func (r *raftNode) processMessages(ms []*raftpb.Message) []*raftpb.Message {
	sentAppResp := false
	var messages []*raftpb.Message
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		if r.isIDRemoved(m.GetTo()) {
			continue
		}

		if m.GetType() == raftpb.MsgAppResp {
			if sentAppResp {
				continue
			}
			sentAppResp = true
		}

		if m.GetType() == raftpb.MsgSnap {
			// There are two separate data store: the store for v2, and the KV for v3.
			// The msgSnap only contains the most recent snapshot of store without KV.
			// So we need to redirect the msgSnap to etcd server main loop for merging in the
			// current store snapshot and KV snapshot.
			select {
			case r.msgSnapC <- m:
			default:
				// drop msgSnap if the inflight chan if full.
			}
			continue
		}
		if m.GetType() == raftpb.MsgHeartbeat {
			ok, exceed := r.td.Observe(m.GetTo())
			if !ok {
				// TODO: limit request rate.
				r.lg.Warn(
					"leader failed to send out heartbeat on time; took too long, leader is overloaded likely from slow disk",
					zap.String("to", fmt.Sprintf("%x", m.GetTo())),
					zap.Duration("heartbeat-interval", r.heartbeat),
					zap.Duration("expected-duration", 2*r.heartbeat),
					zap.Duration("exceeded-duration", exceed),
				)
				heartbeatSendFailures.Inc()
			}
		}
		messages = append(messages, m)
	}
	return messages
}

func (r *raftNode) apply() chan toApply {
	return r.applyc
}

func (r *raftNode) stop() {
	select {
	case r.stopped <- struct{}{}:
		// Not already stopped, so trigger it
	case <-r.done:
		// Has already been stopped - no need to do anything
		return
	}
	// Block until the stop has been acknowledged by start()
	<-r.done
}

func (r *raftNode) onStop() {
	r.Stop()
	r.ticker.Stop()
	r.transport.Stop()
	if err := r.storage.Close(); err != nil {
		r.lg.Panic("failed to close Raft storage", zap.Error(err))
	}
	close(r.done)
}

// for testing
func (r *raftNode) pauseSending() {
	p := r.transport.(rafthttp.Pausable)
	p.Pause()
}

func (r *raftNode) resumeSending() {
	p := r.transport.(rafthttp.Pausable)
	p.Resume()
}

// advanceTicks advances ticks of Raft node.
// This can be used for fast-forwarding election
// ticks in multi data-center deployments, thus
// speeding up election process.
func (r *raftNode) advanceTicks(ticks int) {
	for i := 0; i < ticks; i++ {
		r.tick()
	}
}

func (r *raftNode) ReadState() <-chan raft.ReadState {
	return r.readStateC
}
