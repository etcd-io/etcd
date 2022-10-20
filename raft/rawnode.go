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
	"errors"

	pb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/raft/v3/tracker"
)

// ErrStepLocalMsg is returned when try to step a local raft message
var ErrStepLocalMsg = errors.New("raft: cannot step raft local message")

// ErrStepPeerNotFound is returned when try to step a response message
// but there is no peer found in raft.prs for that node.
var ErrStepPeerNotFound = errors.New("raft: cannot step as peer not found")

// RawNode is a thread-unsafe Node.
// The methods of this struct correspond to the methods of Node and are described
// more fully there.
type RawNode struct {
	raft           *raft
	prevSoftSt     *SoftState
	prevHardSt     pb.HardState
	stepsOnAdvance []pb.Message
}

// NewRawNode instantiates a RawNode from the given configuration.
//
// See Bootstrap() for bootstrapping an initial state; this replaces the former
// 'peers' argument to this method (with identical behavior). However, It is
// recommended that instead of calling Bootstrap, applications bootstrap their
// state manually by setting up a Storage that has a first index > 1 and which
// stores the desired ConfState as its InitialState.
func NewRawNode(config *Config) (*RawNode, error) {
	r := newRaft(config)
	rn := &RawNode{
		raft: r,
	}
	rn.prevSoftSt = r.softState()
	rn.prevHardSt = r.hardState()
	return rn, nil
}

// Tick advances the internal logical clock by a single tick.
func (rn *RawNode) Tick() {
	rn.raft.tick()
}

// TickQuiesced advances the internal logical clock by a single tick without
// performing any other state machine processing. It allows the caller to avoid
// periodic heartbeats and elections when all of the peers in a Raft group are
// known to be at the same state. Expected usage is to periodically invoke Tick
// or TickQuiesced depending on whether the group is "active" or "quiesced".
//
// WARNING: Be very careful about using this method as it subverts the Raft
// state machine. You should probably be using Tick instead.
func (rn *RawNode) TickQuiesced() {
	rn.raft.electionElapsed++
}

// Campaign causes this RawNode to transition to candidate state.
func (rn *RawNode) Campaign() error {
	return rn.raft.Step(pb.Message{
		Type: pb.MsgHup,
	})
}

// Propose proposes data be appended to the raft log.
func (rn *RawNode) Propose(data []byte) error {
	return rn.raft.Step(pb.Message{
		Type: pb.MsgProp,
		From: rn.raft.id,
		Entries: []pb.Entry{
			{Data: data},
		}})
}

// ProposeConfChange proposes a config change. See (Node).ProposeConfChange for
// details.
func (rn *RawNode) ProposeConfChange(cc pb.ConfChangeI) error {
	m, err := confChangeToMsg(cc)
	if err != nil {
		return err
	}
	return rn.raft.Step(m)
}

// ApplyConfChange applies a config change to the local node. The app must call
// this when it applies a configuration change, except when it decides to reject
// the configuration change, in which case no call must take place.
func (rn *RawNode) ApplyConfChange(cc pb.ConfChangeI) *pb.ConfState {
	cs := rn.raft.applyConfChange(cc.AsV2())
	return &cs
}

// Step advances the state machine using the given message.
func (rn *RawNode) Step(m pb.Message) error {
	// Ignore unexpected local messages receiving over network.
	if IsLocalMsg(m.Type) && !IsLocalMsgTarget(m.From) {
		return ErrStepLocalMsg
	}
	if IsResponseMsg(m.Type) && !IsLocalMsgTarget(m.From) && rn.raft.prs.Progress[m.From] == nil {
		return ErrStepPeerNotFound
	}
	return rn.raft.Step(m)
}

// Ready returns the outstanding work that the application needs to handle. This
// includes appending and applying entries or a snapshot, updating the HardState,
// and sending messages. The returned Ready() *must* be handled and subsequently
// passed back via Advance().
func (rn *RawNode) Ready() Ready {
	rd := rn.readyWithoutAccept()
	rn.acceptReady(rd)
	return rd
}

// readyWithoutAccept returns a Ready. This is a read-only operation, i.e. there
// is no obligation that the Ready must be handled.
func (rn *RawNode) readyWithoutAccept() Ready {
	return newReady(rn.raft, rn.prevSoftSt, rn.prevHardSt)
}

// acceptReady is called when the consumer of the RawNode has decided to go
// ahead and handle a Ready. Nothing must alter the state of the RawNode between
// this call and the prior call to Ready().
func (rn *RawNode) acceptReady(rd Ready) {
	if rd.SoftState != nil {
		rn.prevSoftSt = rd.SoftState
	}
	if !IsEmptyHardState(rd.HardState) {
		rn.prevHardSt = rd.HardState
	}
	if len(rd.ReadStates) != 0 {
		rn.raft.readStates = nil
	}
	if !rn.raft.asyncStorageWrites {
		if len(rn.stepsOnAdvance) != 0 {
			rn.raft.logger.Panicf("two accepted Ready structs without call to Advance")
		}
		for _, m := range rn.raft.msgsAfterAppend {
			if m.To == rn.raft.id {
				rn.stepsOnAdvance = append(rn.stepsOnAdvance, m)
			}
		}
		if needStorageAppend(rd, false /* haveMsgsAfterAppend */) {
			m := newStorageAppendRespMsg(rn.raft, rd)
			rn.stepsOnAdvance = append(rn.stepsOnAdvance, m)
		}
		if needStorageApply(rd) {
			m := newStorageApplyRespMsg(rn.raft, rd.CommittedEntries)
			rn.stepsOnAdvance = append(rn.stepsOnAdvance, m)
		}
	}
	rn.raft.msgs = nil
	rn.raft.msgsAfterAppend = nil
	rn.raft.raftLog.acceptUnstable()
	if len(rd.CommittedEntries) > 0 {
		ents := rd.CommittedEntries
		rn.raft.raftLog.acceptApplying(ents[len(ents)-1].Index)
	}
}

// HasReady called when RawNode user need to check if any Ready pending.
func (rn *RawNode) HasReady() bool {
	r := rn.raft
	if !r.softState().equal(rn.prevSoftSt) {
		return true
	}
	if hardSt := r.hardState(); !IsEmptyHardState(hardSt) && !isHardStateEqual(hardSt, rn.prevHardSt) {
		return true
	}
	if r.raftLog.hasNextUnstableSnapshot() {
		return true
	}
	if len(r.msgs) > 0 || len(r.msgsAfterAppend) > 0 {
		return true
	}
	if r.raftLog.hasNextUnstableEnts() || r.raftLog.hasNextCommittedEnts(!rn.raft.asyncStorageWrites) {
		return true
	}
	if len(r.readStates) != 0 {
		return true
	}
	return false
}

// Advance notifies the RawNode that the application has applied and saved progress in the
// last Ready results.
//
// NOTE: Advance must not be called when using AsyncStorageWrites. Response messages from
// the local append and apply threads take its place.
func (rn *RawNode) Advance(_ Ready) {
	// The actions performed by this function are encoded into stepsOnAdvance in
	// acceptReady. In earlier versions of this library, they were computed from
	// the provided Ready struct. Retain the unused parameter for compatability.
	if rn.raft.asyncStorageWrites {
		rn.raft.logger.Panicf("Advance must not be called when using AsyncStorageWrites")
	}
	for i, m := range rn.stepsOnAdvance {
		_ = rn.raft.Step(m)
		rn.stepsOnAdvance[i] = pb.Message{}
	}
	rn.stepsOnAdvance = rn.stepsOnAdvance[:0]
}

// Status returns the current status of the given group. This allocates, see
// BasicStatus and WithProgress for allocation-friendlier choices.
func (rn *RawNode) Status() Status {
	status := getStatus(rn.raft)
	return status
}

// BasicStatus returns a BasicStatus. Notably this does not contain the
// Progress map; see WithProgress for an allocation-free way to inspect it.
func (rn *RawNode) BasicStatus() BasicStatus {
	return getBasicStatus(rn.raft)
}

// ProgressType indicates the type of replica a Progress corresponds to.
type ProgressType byte

const (
	// ProgressTypePeer accompanies a Progress for a regular peer replica.
	ProgressTypePeer ProgressType = iota
	// ProgressTypeLearner accompanies a Progress for a learner replica.
	ProgressTypeLearner
)

// WithProgress is a helper to introspect the Progress for this node and its
// peers.
func (rn *RawNode) WithProgress(visitor func(id uint64, typ ProgressType, pr tracker.Progress)) {
	rn.raft.prs.Visit(func(id uint64, pr *tracker.Progress) {
		typ := ProgressTypePeer
		if pr.IsLearner {
			typ = ProgressTypeLearner
		}
		p := *pr
		p.Inflights = nil
		visitor(id, typ, p)
	})
}

// ReportUnreachable reports the given node is not reachable for the last send.
func (rn *RawNode) ReportUnreachable(id uint64) {
	_ = rn.raft.Step(pb.Message{Type: pb.MsgUnreachable, From: id})
}

// ReportSnapshot reports the status of the sent snapshot.
func (rn *RawNode) ReportSnapshot(id uint64, status SnapshotStatus) {
	rej := status == SnapshotFailure

	_ = rn.raft.Step(pb.Message{Type: pb.MsgSnapStatus, From: id, Reject: rej})
}

// TransferLeader tries to transfer leadership to the given transferee.
func (rn *RawNode) TransferLeader(transferee uint64) {
	_ = rn.raft.Step(pb.Message{Type: pb.MsgTransferLeader, From: transferee})
}

// ReadIndex requests a read state. The read state will be set in ready.
// Read State has a read index. Once the application advances further than the read
// index, any linearizable read requests issued before the read request can be
// processed safely. The read state will have the same rctx attached.
func (rn *RawNode) ReadIndex(rctx []byte) {
	_ = rn.raft.Step(pb.Message{Type: pb.MsgReadIndex, Entries: []pb.Entry{{Data: rctx}}})
}
