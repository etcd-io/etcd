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
	"fmt"
	"sort"
)

const (
	ProgressStateProbe ProgressStateType = iota
	ProgressStateReplicate
	ProgressStateSnapshot
)

type ProgressStateType uint64

var prstmap = [...]string{
	"ProgressStateProbe",
	"ProgressStateReplicate",
	"ProgressStateSnapshot",
}

func (st ProgressStateType) String() string { return prstmap[uint64(st)] }

// Progress represents a follower’s progress in the view of the leader. Leader maintains
// progresses of all followers, and sends entries to the follower based on its progress.
type Progress struct {
	Match, Next uint64
	// State defines how the leader should interact with the follower.
	//
	// When in ProgressStateProbe, leader sends at most one replication message
	// per heartbeat interval. It also probes actual progress of the follower.
	//
	// When in ProgressStateReplicate, leader optimistically increases next
	// to the latest entry sent after sending replication message. This is
	// an optimized state for fast replicating log entries to the follower.
	//
	// When in ProgressStateSnapshot, leader should have sent out snapshot
	// before and stops sending any replication message.
	State ProgressStateType

	// Paused is used in ProgressStateProbe.
	// When Paused is true, raft should pause sending replication message to this peer.
	Paused bool
	// PendingSnapshot is used in ProgressStateSnapshot.
	// If there is a pending snapshot, the pendingSnapshot will be set to the
	// index of the snapshot. If pendingSnapshot is set, the replication process of
	// this Progress will be paused. raft will not resend snapshot until the pending one
	// is reported to be failed.
	PendingSnapshot uint64

	// RecentActive is true if the progress is recently active. Receiving any messages
	// from the corresponding follower indicates the progress is active.
	// RecentActive can be reset to false after an election timeout.
	RecentActive bool

	// inflights is a sliding window for the inflight messages.
	// Each inflight message contains one or more log entries.
	// The max number of entries per message is defined in raft config as MaxSizePerMsg.
	// Thus inflight effectively limits both the number of inflight messages
	// and the bandwidth each Progress can use.
	// When inflights is full, no more message should be sent.
	// When a leader sends out a message, the index of the last
	// entry should be added to inflights. The index MUST be added
	// into inflights in order.
	// When a leader receives a reply, the previous inflights should
	// be freed by calling inflights.freeTo with the index of the last
	// received entry.
	ins *inflights

	// IsLearner is true if this progress is tracked for a learner.
	IsLearner bool
}

func (pr *Progress) resetState(state ProgressStateType) {
	pr.Paused = false
	pr.PendingSnapshot = 0
	pr.State = state
	pr.ins.reset()
}

func (pr *Progress) becomeProbe() {
	// If the original state is ProgressStateSnapshot, progress knows that
	// the pending snapshot has been sent to this peer successfully, then
	// probes from pendingSnapshot + 1.
	if pr.State == ProgressStateSnapshot {
		pendingSnapshot := pr.PendingSnapshot
		pr.resetState(ProgressStateProbe)
		pr.Next = max(pr.Match+1, pendingSnapshot+1)
	} else {
		pr.resetState(ProgressStateProbe)
		pr.Next = pr.Match + 1
	}
}

func (pr *Progress) becomeReplicate() {
	pr.resetState(ProgressStateReplicate)
	pr.Next = pr.Match + 1
}

func (pr *Progress) becomeSnapshot(snapshoti uint64) {
	pr.resetState(ProgressStateSnapshot)
	pr.PendingSnapshot = snapshoti
}

// maybeUpdate returns false if the given n index comes from an outdated message.
// Otherwise it updates the progress and returns true.
func (pr *Progress) maybeUpdate(n uint64) bool {
	var updated bool
	if pr.Match < n {
		pr.Match = n
		updated = true
		pr.resume()
	}
	if pr.Next < n+1 {
		pr.Next = n + 1
	}
	return updated
}

func (pr *Progress) optimisticUpdate(n uint64) { pr.Next = n + 1 }

// maybeDecrTo returns false if the given to index comes from an out of order message.
// Otherwise it decreases the progress next index to min(rejected, last) and returns true.
func (pr *Progress) maybeDecrTo(rejected, last uint64) bool {
	if pr.State == ProgressStateReplicate {
		// the rejection must be stale if the progress has matched and "rejected"
		// is smaller than "match".
		if rejected <= pr.Match {
			return false
		}
		// directly decrease next to match + 1
		pr.Next = pr.Match + 1
		return true
	}

	// the rejection must be stale if "rejected" does not match next - 1
	if pr.Next-1 != rejected {
		return false
	}

	if pr.Next = min(rejected, last+1); pr.Next < 1 {
		pr.Next = 1
	}
	pr.resume()
	return true
}

func (pr *Progress) pause()  { pr.Paused = true }
func (pr *Progress) resume() { pr.Paused = false }

// IsPaused returns whether sending log entries to this node has been
// paused. A node may be paused because it has rejected recent
// MsgApps, is currently waiting for a snapshot, or has reached the
// MaxInflightMsgs limit.
func (pr *Progress) IsPaused() bool {
	switch pr.State {
	case ProgressStateProbe:
		return pr.Paused
	case ProgressStateReplicate:
		return pr.ins.full()
	case ProgressStateSnapshot:
		return true
	default:
		panic("unexpected state")
	}
}

func (pr *Progress) snapshotFailure() { pr.PendingSnapshot = 0 }

// needSnapshotAbort returns true if snapshot progress's Match
// is equal or higher than the pendingSnapshot.
func (pr *Progress) needSnapshotAbort() bool {
	return pr.State == ProgressStateSnapshot && pr.Match >= pr.PendingSnapshot
}

func (pr *Progress) String() string {
	return fmt.Sprintf("next = %d, match = %d, state = %s, waiting = %v, pendingSnapshot = %d, recentActive = %v, isLearner = %v",
		pr.Next, pr.Match, pr.State, pr.IsPaused(), pr.PendingSnapshot, pr.RecentActive, pr.IsLearner)
}

type inflights struct {
	// the starting index in the buffer
	start int
	// number of inflights in the buffer
	count int

	// the size of the buffer
	size int

	// buffer contains the index of the last entry
	// inside one message.
	buffer []uint64
}

func newInflights(size int) *inflights {
	return &inflights{
		size: size,
	}
}

// add adds an inflight into inflights
func (in *inflights) add(inflight uint64) {
	if in.full() {
		panic("cannot add into a full inflights")
	}
	next := in.start + in.count
	size := in.size
	if next >= size {
		next -= size
	}
	if next >= len(in.buffer) {
		in.growBuf()
	}
	in.buffer[next] = inflight
	in.count++
}

// grow the inflight buffer by doubling up to inflights.size. We grow on demand
// instead of preallocating to inflights.size to handle systems which have
// thousands of Raft groups per process.
func (in *inflights) growBuf() {
	newSize := len(in.buffer) * 2
	if newSize == 0 {
		newSize = 1
	} else if newSize > in.size {
		newSize = in.size
	}
	newBuffer := make([]uint64, newSize)
	copy(newBuffer, in.buffer)
	in.buffer = newBuffer
}

// freeTo frees the inflights smaller or equal to the given `to` flight.
func (in *inflights) freeTo(to uint64) {
	if in.count == 0 || to < in.buffer[in.start] {
		// out of the left side of the window
		return
	}

	idx := in.start
	var i int
	for i = 0; i < in.count; i++ {
		if to < in.buffer[idx] { // found the first large inflight
			break
		}

		// increase index and maybe rotate
		size := in.size
		if idx++; idx >= size {
			idx -= size
		}
	}
	// free i inflights and set new start index
	in.count -= i
	in.start = idx
	if in.count == 0 {
		// inflights is empty, reset the start index so that we don't grow the
		// buffer unnecessarily.
		in.start = 0
	}
}

func (in *inflights) freeFirstOne() { in.freeTo(in.buffer[in.start]) }

// full returns true if the inflights is full.
func (in *inflights) full() bool {
	return in.count == in.size
}

// resets frees all inflights.
func (in *inflights) reset() {
	in.count = 0
	in.start = 0
}

// progressTracker tracks the currently active configuration and the information
// known about the nodes and learners in it. In particular, it tracks the match
// index for each peer which in turn allows reasoning about the committed index.
type progressTracker struct {
	nodes    map[uint64]*Progress
	learners map[uint64]*Progress

	votes map[uint64]bool

	maxInflight int
	matchBuf    uint64Slice
}

func makePRS(maxInflight int) progressTracker {
	p := progressTracker{
		maxInflight: maxInflight,
		nodes:       map[uint64]*Progress{},
		learners:    map[uint64]*Progress{},
		votes:       map[uint64]bool{},
	}
	return p
}

// isSingleton returns true if (and only if) there is only one voting member
// (i.e. the leader) in the current configuration.
func (p *progressTracker) isSingleton() bool {
	return len(p.nodes) == 1
}

func (p *progressTracker) quorum() int {
	return len(p.nodes)/2 + 1
}

func (p *progressTracker) hasQuorum(m map[uint64]struct{}) bool {
	return len(m) >= p.quorum()
}

// committed returns the largest log index known to be committed based on what
// the voting members of the group have acknowledged.
func (p *progressTracker) committed() uint64 {
	// Preserving matchBuf across calls is an optimization
	// used to avoid allocating a new slice on each call.
	if cap(p.matchBuf) < len(p.nodes) {
		p.matchBuf = make(uint64Slice, len(p.nodes))
	}
	p.matchBuf = p.matchBuf[:len(p.nodes)]
	idx := 0
	for _, pr := range p.nodes {
		p.matchBuf[idx] = pr.Match
		idx++
	}
	sort.Sort(&p.matchBuf)
	return p.matchBuf[len(p.matchBuf)-p.quorum()]
}

func (p *progressTracker) removeAny(id uint64) {
	pN := p.nodes[id]
	pL := p.learners[id]

	if pN == nil && pL == nil {
		panic("attempting to remove unknown peer %x")
	} else if pN != nil && pL != nil {
		panic(fmt.Sprintf("peer %x is both voter and learner", id))
	}

	delete(p.nodes, id)
	delete(p.learners, id)
}

// initProgress initializes a new progress for the given node or learner. The
// node may not exist yet in either form or a panic will ensue.
func (p *progressTracker) initProgress(id, match, next uint64, isLearner bool) {
	if pr := p.nodes[id]; pr != nil {
		panic(fmt.Sprintf("peer %x already tracked as node %v", id, pr))
	}
	if pr := p.learners[id]; pr != nil {
		panic(fmt.Sprintf("peer %x already tracked as learner %v", id, pr))
	}
	if !isLearner {
		p.nodes[id] = &Progress{Next: next, Match: match, ins: newInflights(p.maxInflight)}
		return
	}
	p.learners[id] = &Progress{Next: next, Match: match, ins: newInflights(p.maxInflight), IsLearner: true}
}

func (p *progressTracker) getProgress(id uint64) *Progress {
	if pr, ok := p.nodes[id]; ok {
		return pr
	}

	return p.learners[id]
}

// visit invokes the supplied closure for all tracked progresses.
func (p *progressTracker) visit(f func(id uint64, pr *Progress)) {
	for id, pr := range p.nodes {
		f(id, pr)
	}

	for id, pr := range p.learners {
		f(id, pr)
	}
}

// checkQuorumActive returns true if the quorum is active from
// the view of the local raft state machine. Otherwise, it returns
// false.
func (p *progressTracker) quorumActive() bool {
	var act int
	p.visit(func(id uint64, pr *Progress) {
		if pr.RecentActive && !pr.IsLearner {
			act++
		}
	})

	return act >= p.quorum()
}

func (p *progressTracker) voterNodes() []uint64 {
	nodes := make([]uint64, 0, len(p.nodes))
	for id := range p.nodes {
		nodes = append(nodes, id)
	}
	sort.Sort(uint64Slice(nodes))
	return nodes
}

func (p *progressTracker) learnerNodes() []uint64 {
	nodes := make([]uint64, 0, len(p.learners))
	for id := range p.learners {
		nodes = append(nodes, id)
	}
	sort.Sort(uint64Slice(nodes))
	return nodes
}

// resetVotes prepares for a new round of vote counting via recordVote.
func (p *progressTracker) resetVotes() {
	p.votes = map[uint64]bool{}
}

// recordVote records that the node with the given id voted for this Raft
// instance if v == true (and declined it otherwise).
func (p *progressTracker) recordVote(id uint64, v bool) {
	_, ok := p.votes[id]
	if !ok {
		p.votes[id] = v
	}
}

// tallyVotes returns the number of granted and rejected votes, and whether the
// election outcome is known.
func (p *progressTracker) tallyVotes() (granted int, rejected int, result electionResult) {
	for _, v := range p.votes {
		if v {
			granted++
		} else {
			rejected++
		}
	}

	q := p.quorum()

	result = electionIndeterminate
	if granted >= q {
		result = electionWon
	} else if rejected >= q {
		result = electionLost
	}
	return granted, rejected, result
}
