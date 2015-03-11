// Copyright 2015 CoreOS, Inc.
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
	"fmt"
	"math/rand"
	"sort"
	"strings"

	pb "github.com/coreos/etcd/raft/raftpb"
)

// None is a placeholder node ID used when there is no leader.
const None uint64 = 0

var errNoLeader = errors.New("no leader")

// Possible values for StateType.
const (
	StateFollower StateType = iota
	StateCandidate
	StateLeader
)

// StateType represents the role of a node in a cluster.
type StateType uint64

var stmap = [...]string{
	"StateFollower",
	"StateCandidate",
	"StateLeader",
}

func (st StateType) String() string {
	return stmap[uint64(st)]
}

type Progress struct {
	Match, Next uint64
	Wait        int
	// If the last sent to the Progress failed and reported
	// by the link layer via MsgUnreachable, Unreachable will be set.
	// If the Progress is unreachable, snapshot and optimistically append
	// will be disabled.
	// Unreachable will be unset if raft starts to receive message (msgAppResp,
	// msgHeartbeatResp) from the remote peer of the Progress.
	Unreachable bool
	// If there is a pending snapshot, the pendingSnapshot will be set to the
	// index of the snapshot. If pendingSnapshot is set, the replication process of
	// this Progress will be paused. raft will not resend snapshot until the pending one
	// is reported to be failed.
	//
	// PendingSnapshot is set when raft sends out a snapshot to this Progress.
	// PendingSnapshot is unset when the snapshot is reported to be successfully,
	// or raft updates an equal or higher Match for this Progress.
	PendingSnapshot uint64
}

func (pr *Progress) update(n uint64) {
	pr.waitReset()

	if pr.Match < n {
		pr.Match = n
	}
	if pr.Next < n+1 {
		pr.Next = n + 1
	}
}

func (pr *Progress) optimisticUpdate(n uint64) { pr.Next = n + 1 }

// maybeDecrTo returns false if the given to index comes from an out of order message.
// Otherwise it decreases the progress next index to min(rejected, last) and returns true.
func (pr *Progress) maybeDecrTo(rejected, last uint64) bool {
	pr.waitReset()

	if pr.Match != 0 {
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
	return true
}

func (pr *Progress) waitDecr(i int) {
	pr.Wait -= i
	if pr.Wait < 0 {
		pr.Wait = 0
	}
}
func (pr *Progress) waitSet(w int)       { pr.Wait = w }
func (pr *Progress) waitReset()          { pr.Wait = 0 }
func (pr *Progress) isUnreachable() bool { return pr.Unreachable }
func (pr *Progress) reachable()          { pr.Unreachable = false }

func (pr *Progress) unreachable() {
	pr.Unreachable = true
	// When in optimistic appending path, if the remote becomes unreachable,
	// there is big probability that it loses MsgApp. Fall back to bad
	// path to recover it steadily.
	if pr.Match != 0 {
		pr.Next = pr.Match + 1
	}
}

func (pr *Progress) shouldWait() bool { return (pr.Unreachable || pr.Match == 0) && pr.Wait > 0 }

func (pr *Progress) hasPendingSnapshot() bool    { return pr.PendingSnapshot != 0 }
func (pr *Progress) setPendingSnapshot(i uint64) { pr.PendingSnapshot = i }

// finishSnapshot unsets the pending snapshot and optimistically increase Next to
// the index of pendingSnapshot + 1. The next replication message is expected
// to be msgApp.
func (pr *Progress) snapshotFinish() {
	pr.Next = pr.PendingSnapshot + 1
	pr.PendingSnapshot = 0
}

// snapshotFail unsets the pending snapshot. The next replication message is expected
// to be another msgSnap.
func (pr *Progress) snapshotFail() {
	pr.PendingSnapshot = 0
}

// maybeSnapshotAbort unsets pendingSnapshot if Match is equal or higher than
// the pendingSnapshot
func (pr *Progress) maybeSnapshotAbort() bool {
	if pr.hasPendingSnapshot() && pr.Match >= pr.PendingSnapshot {
		pr.PendingSnapshot = 0
		return true
	}
	return false
}

func (pr *Progress) String() string {
	return fmt.Sprintf("next = %d, match = %d, wait = %v", pr.Next, pr.Match, pr.Wait)
}

type raft struct {
	pb.HardState

	id uint64

	// the log
	raftLog *raftLog

	prs map[uint64]*Progress

	state StateType

	votes map[uint64]bool

	msgs []pb.Message

	// the leader id
	lead uint64

	// New configuration is ignored if there exists unapplied configuration.
	pendingConf bool

	elapsed          int // number of ticks since the last msg
	heartbeatTimeout int
	electionTimeout  int
	rand             *rand.Rand
	tick             func()
	step             stepFunc
}

func newRaft(id uint64, peers []uint64, election, heartbeat int, storage Storage,
	applied uint64) *raft {
	if id == None {
		panic("cannot use none id")
	}
	raftlog := newLog(storage)
	hs, cs, err := storage.InitialState()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	if len(cs.Nodes) > 0 {
		if len(peers) > 0 {
			// TODO(bdarnell): the peers argument is always nil except in
			// tests; the argument should be removed and these tests should be
			// updated to specify their nodes through a snapshot.
			panic("cannot specify both newRaft(peers) and ConfState.Nodes)")
		}
		peers = cs.Nodes
	}
	r := &raft{
		id:               id,
		lead:             None,
		raftLog:          raftlog,
		prs:              make(map[uint64]*Progress),
		electionTimeout:  election,
		heartbeatTimeout: heartbeat,
	}
	r.rand = rand.New(rand.NewSource(int64(id)))
	for _, p := range peers {
		r.prs[p] = &Progress{Next: 1}
	}
	if !isHardStateEqual(hs, emptyState) {
		r.loadState(hs)
	}
	if applied > 0 {
		raftlog.appliedTo(applied)
	}
	r.becomeFollower(r.Term, None)

	nodesStrs := make([]string, 0)
	for _, n := range r.nodes() {
		nodesStrs = append(nodesStrs, fmt.Sprintf("%x", n))
	}

	raftLogger.Infof("raft: newRaft %x [peers: [%s], term: %d, commit: %d, applied: %d, lastindex: %d, lastterm: %d]",
		r.id, strings.Join(nodesStrs, ","), r.Term, r.raftLog.committed, r.raftLog.applied, r.raftLog.lastIndex(), r.raftLog.lastTerm())
	return r
}

func (r *raft) hasLeader() bool { return r.lead != None }

func (r *raft) softState() *SoftState { return &SoftState{Lead: r.lead, RaftState: r.state} }

func (r *raft) q() int { return len(r.prs)/2 + 1 }

func (r *raft) nodes() []uint64 {
	nodes := make([]uint64, 0, len(r.prs))
	for k := range r.prs {
		nodes = append(nodes, k)
	}
	sort.Sort(uint64Slice(nodes))
	return nodes
}

// send persists state to stable storage and then sends to its mailbox.
func (r *raft) send(m pb.Message) {
	m.From = r.id
	// do not attach term to MsgProp
	// proposals are a way to forward to the leader and
	// should be treated as local message.
	if m.Type != pb.MsgProp {
		m.Term = r.Term
	}
	r.msgs = append(r.msgs, m)
}

// sendAppend sends RRPC, with entries to the given peer.
func (r *raft) sendAppend(to uint64) {
	pr := r.prs[to]
	if pr.shouldWait() || pr.hasPendingSnapshot() {
		return
	}
	m := pb.Message{}
	m.To = to
	if r.needSnapshot(pr.Next) {
		if pr.isUnreachable() {
			// do not try to send snapshot until the Progress is
			// reachable
			return
		}

		m.Type = pb.MsgSnap
		snapshot, err := r.raftLog.snapshot()
		if err != nil {
			panic(err) // TODO(bdarnell)
		}
		if IsEmptySnap(snapshot) {
			panic("need non-empty snapshot")
		}
		m.Snapshot = snapshot
		sindex, sterm := snapshot.Metadata.Index, snapshot.Metadata.Term
		raftLogger.Infof("raft: %x [firstindex: %d, commit: %d] sent snapshot[index: %d, term: %d] to %x [%s]",
			r.id, r.raftLog.firstIndex(), r.Commit, sindex, sterm, to, pr)
		pr.setPendingSnapshot(sindex)
		raftLogger.Infof("raft: %x paused sending replication messages to %x [%s]", r.id, to, pr)
	} else {
		m.Type = pb.MsgApp
		m.Index = pr.Next - 1
		m.LogTerm = r.raftLog.term(pr.Next - 1)
		m.Entries = r.raftLog.entries(pr.Next)
		m.Commit = r.raftLog.committed
		// optimistically increase the next if the follower
		// has been matched.
		if n := len(m.Entries); pr.Match != 0 && !pr.isUnreachable() && n != 0 {
			pr.optimisticUpdate(m.Entries[n-1].Index)
		} else if pr.Match == 0 || pr.isUnreachable() {
			pr.waitSet(r.heartbeatTimeout)
		}
	}
	r.send(m)
}

// sendHeartbeat sends an empty MsgApp
func (r *raft) sendHeartbeat(to uint64) {
	// Attach the commit as min(to.matched, r.committed).
	// When the leader sends out heartbeat message,
	// the receiver(follower) might not be matched with the leader
	// or it might not have all the committed entries.
	// The leader MUST NOT forward the follower's commit to
	// an unmatched index.
	commit := min(r.prs[to].Match, r.raftLog.committed)
	m := pb.Message{
		To:     to,
		Type:   pb.MsgHeartbeat,
		Commit: commit,
	}
	r.send(m)
}

// bcastAppend sends RRPC, with entries to all peers that are not up-to-date
// according to the progress recorded in r.prs.
func (r *raft) bcastAppend() {
	for i := range r.prs {
		if i == r.id {
			continue
		}
		r.sendAppend(i)
	}
}

// bcastHeartbeat sends RRPC, without entries to all the peers.
func (r *raft) bcastHeartbeat() {
	for i := range r.prs {
		if i == r.id {
			continue
		}
		r.sendHeartbeat(i)
		r.prs[i].waitDecr(r.heartbeatTimeout)
	}
}

func (r *raft) maybeCommit() bool {
	// TODO(bmizerany): optimize.. Currently naive
	mis := make(uint64Slice, 0, len(r.prs))
	for i := range r.prs {
		mis = append(mis, r.prs[i].Match)
	}
	sort.Sort(sort.Reverse(mis))
	mci := mis[r.q()-1]
	return r.raftLog.maybeCommit(mci, r.Term)
}

func (r *raft) reset(term uint64) {
	if r.Term != term {
		r.Term = term
		r.Vote = None
	}
	r.lead = None
	r.elapsed = 0
	r.votes = make(map[uint64]bool)
	for i := range r.prs {
		r.prs[i] = &Progress{Next: r.raftLog.lastIndex() + 1}
		if i == r.id {
			r.prs[i].Match = r.raftLog.lastIndex()
		}
	}
	r.pendingConf = false
}

func (r *raft) appendEntry(es ...pb.Entry) {
	li := r.raftLog.lastIndex()
	for i := range es {
		es[i].Term = r.Term
		es[i].Index = li + 1 + uint64(i)
	}
	r.raftLog.append(es...)
	r.prs[r.id].update(r.raftLog.lastIndex())
	r.maybeCommit()
}

// tickElection is run by followers and candidates after r.electionTimeout.
func (r *raft) tickElection() {
	if !r.promotable() {
		r.elapsed = 0
		return
	}
	r.elapsed++
	if r.isElectionTimeout() {
		r.elapsed = 0
		r.Step(pb.Message{From: r.id, Type: pb.MsgHup})
	}
}

// tickHeartbeat is run by leaders to send a MsgBeat after r.heartbeatTimeout.
func (r *raft) tickHeartbeat() {
	r.elapsed++
	if r.elapsed >= r.heartbeatTimeout {
		r.elapsed = 0
		r.Step(pb.Message{From: r.id, Type: pb.MsgBeat})
	}
}

func (r *raft) becomeFollower(term uint64, lead uint64) {
	r.step = stepFollower
	r.reset(term)
	r.tick = r.tickElection
	r.lead = lead
	r.state = StateFollower
	raftLogger.Infof("raft: %x became follower at term %d", r.id, r.Term)
}

func (r *raft) becomeCandidate() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if r.state == StateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	r.step = stepCandidate
	r.reset(r.Term + 1)
	r.tick = r.tickElection
	r.Vote = r.id
	r.state = StateCandidate
	raftLogger.Infof("raft: %x became candidate at term %d", r.id, r.Term)
}

func (r *raft) becomeLeader() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if r.state == StateFollower {
		panic("invalid transition [follower -> leader]")
	}
	r.step = stepLeader
	r.reset(r.Term)
	r.tick = r.tickHeartbeat
	r.lead = r.id
	r.state = StateLeader
	for _, e := range r.raftLog.entries(r.raftLog.committed + 1) {
		if e.Type != pb.EntryConfChange {
			continue
		}
		if r.pendingConf {
			panic("unexpected double uncommitted config entry")
		}
		r.pendingConf = true
	}
	r.appendEntry(pb.Entry{Data: nil})
	raftLogger.Infof("raft: %x became leader at term %d", r.id, r.Term)
}

func (r *raft) campaign() {
	r.becomeCandidate()
	if r.q() == r.poll(r.id, true) {
		r.becomeLeader()
		return
	}
	for i := range r.prs {
		if i == r.id {
			continue
		}
		raftLogger.Infof("raft: %x [logterm: %d, index: %d] sent vote request to %x at term %d",
			r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), i, r.Term)
		r.send(pb.Message{To: i, Type: pb.MsgVote, Index: r.raftLog.lastIndex(), LogTerm: r.raftLog.lastTerm()})
	}
}

func (r *raft) poll(id uint64, v bool) (granted int) {
	if v {
		raftLogger.Infof("raft: %x received vote from %x at term %d", r.id, id, r.Term)
	} else {
		raftLogger.Infof("raft: %x received vote rejection from %x at term %d", r.id, id, r.Term)
	}
	if _, ok := r.votes[id]; !ok {
		r.votes[id] = v
	}
	for _, vv := range r.votes {
		if vv {
			granted++
		}
	}
	return granted
}

func (r *raft) Step(m pb.Message) error {
	if m.Type == pb.MsgHup {
		raftLogger.Infof("raft: %x is starting a new election at term %d", r.id, r.Term)
		r.campaign()
		r.Commit = r.raftLog.committed
		return nil
	}

	switch {
	case m.Term == 0:
		// local message
	case m.Term > r.Term:
		lead := m.From
		if m.Type == pb.MsgVote {
			lead = None
		}
		raftLogger.Infof("raft: %x [term: %d] received a %s message with higher term from %x [term: %d]",
			r.id, r.Term, m.Type, m.From, m.Term)
		r.becomeFollower(m.Term, lead)
	case m.Term < r.Term:
		// ignore
		raftLogger.Infof("raft: %x [term: %d] ignored a %s message with lower term from %x [term: %d]",
			r.id, r.Term, m.Type, m.From, m.Term)
		return nil
	}
	r.step(r, m)
	r.Commit = r.raftLog.committed
	return nil
}

type stepFunc func(r *raft, m pb.Message)

func stepLeader(r *raft, m pb.Message) {
	pr := r.prs[m.From]

	switch m.Type {
	case pb.MsgBeat:
		r.bcastHeartbeat()
	case pb.MsgProp:
		if len(m.Entries) == 0 {
			raftLogger.Panicf("raft: %x stepped empty MsgProp", r.id)
		}
		for i, e := range m.Entries {
			if e.Type == pb.EntryConfChange {
				if r.pendingConf {
					m.Entries[i] = pb.Entry{Type: pb.EntryNormal}
				}
				r.pendingConf = true
			}
		}
		r.appendEntry(m.Entries...)
		r.bcastAppend()
	case pb.MsgAppResp:
		if pr.isUnreachable() {
			pr.reachable()
			raftLogger.Infof("raft: %x received msgAppResp from %x and changed it to be reachable [%s]", r.id, m.From, pr)
		}
		if m.Reject {
			raftLogger.Infof("raft: %x received msgApp rejection(lastindex: %d) from %x for index %d",
				r.id, m.RejectHint, m.From, m.Index)
			if pr.maybeDecrTo(m.Index, m.RejectHint) {
				raftLogger.Infof("raft: %x decreased progress of %x to [%s]", r.id, m.From, pr)
				r.sendAppend(m.From)
			}
		} else {
			oldWait := pr.shouldWait()
			pr.update(m.Index)
			if r.prs[m.From].maybeSnapshotAbort() {
				raftLogger.Infof("raft: %x snapshot aborted, resumed sending replication messages to %x [%s]", r.id, m.From, pr)
			}
			if r.maybeCommit() {
				r.bcastAppend()
			} else if oldWait {
				// update() reset the wait state on this node. If we had delayed sending
				// an update before, send it now.
				r.sendAppend(m.From)
			}
		}
	case pb.MsgHeartbeatResp:
		if pr.isUnreachable() {
			pr.reachable()
			raftLogger.Infof("raft: %x received msgHeartbeatResp from %x and changed it to be reachable [%s]", r.id, m.From, pr)
		}
		if pr.Match < r.raftLog.lastIndex() {
			r.sendAppend(m.From)
		}
	case pb.MsgVote:
		raftLogger.Infof("raft: %x [logterm: %d, index: %d, vote: %x] rejected vote from %x [logterm: %d, index: %d] at term %d",
			r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
		r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp, Reject: true})
	case pb.MsgSnapStatus:
		if !pr.hasPendingSnapshot() {
			return
		}
		if m.Reject {
			pr.snapshotFail()
			raftLogger.Infof("raft: %x snapshot failed, resumed sending replication messages to %x [%s]", r.id, m.From, pr)
		} else {
			pr.snapshotFinish()
			raftLogger.Infof("raft: %x snapshot succeeded, resumed sending replication messages to %x [%s]", r.id, m.From, pr)
			// wait for the msgAppResp from the remote node before sending
			// out the next msgApp
			pr.waitSet(r.electionTimeout)
		}
	case pb.MsgUnreachable:
		if !pr.isUnreachable() {
			pr.unreachable()
			raftLogger.Infof("raft: %x failed to send message to %x and changed it to be unreachable [%s]", r.id, m.From, pr)
		}
	}
}

func stepCandidate(r *raft, m pb.Message) {
	switch m.Type {
	case pb.MsgProp:
		raftLogger.Infof("raft: %x no leader at term %d; dropping proposal", r.id, r.Term)
		return
	case pb.MsgApp:
		r.becomeFollower(r.Term, m.From)
		r.handleAppendEntries(m)
	case pb.MsgHeartbeat:
		r.becomeFollower(r.Term, m.From)
		r.handleHeartbeat(m)
	case pb.MsgSnap:
		r.becomeFollower(m.Term, m.From)
		r.handleSnapshot(m)
	case pb.MsgVote:
		raftLogger.Infof("raft: %x [logterm: %d, index: %d, vote: %x] rejected vote from %x [logterm: %d, index: %d] at term %x",
			r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
		r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp, Reject: true})
	case pb.MsgVoteResp:
		gr := r.poll(m.From, !m.Reject)
		raftLogger.Infof("raft: %x [q:%d] has received %d votes and %d vote rejections", r.id, r.q(), gr, len(r.votes)-gr)
		switch r.q() {
		case gr:
			r.becomeLeader()
			r.bcastAppend()
		case len(r.votes) - gr:
			r.becomeFollower(r.Term, None)
		}
	}
}

func stepFollower(r *raft, m pb.Message) {
	switch m.Type {
	case pb.MsgProp:
		if r.lead == None {
			raftLogger.Infof("raft: %x no leader at term %d; dropping proposal", r.id, r.Term)
			return
		}
		m.To = r.lead
		r.send(m)
	case pb.MsgApp:
		r.elapsed = 0
		r.lead = m.From
		r.handleAppendEntries(m)
	case pb.MsgHeartbeat:
		r.elapsed = 0
		r.lead = m.From
		r.handleHeartbeat(m)
	case pb.MsgSnap:
		r.elapsed = 0
		r.handleSnapshot(m)
	case pb.MsgVote:
		if (r.Vote == None || r.Vote == m.From) && r.raftLog.isUpToDate(m.Index, m.LogTerm) {
			r.elapsed = 0
			raftLogger.Infof("raft: %x [logterm: %d, index: %d, vote: %x] voted for %x [logterm: %d, index: %d] at term %d",
				r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
			r.Vote = m.From
			r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp})
		} else {
			raftLogger.Infof("raft: %x [logterm: %d, index: %d, vote: %x] rejected vote from %x [logterm: %d, index: %d] at term %d",
				r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
			r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp, Reject: true})
		}
	}
}

func (r *raft) handleAppendEntries(m pb.Message) {
	if m.Index < r.Commit {
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: r.Commit})
		return
	}

	if mlastIndex, ok := r.raftLog.maybeAppend(m.Index, m.LogTerm, m.Commit, m.Entries...); ok {
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: mlastIndex})
	} else {
		raftLogger.Infof("raft: %x [logterm: %d, index: %d] rejected msgApp [logterm: %d, index: %d] from %x",
			r.id, r.raftLog.term(m.Index), m.Index, m.LogTerm, m.Index, m.From)
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: m.Index, Reject: true, RejectHint: r.raftLog.lastIndex()})
	}
}

func (r *raft) handleHeartbeat(m pb.Message) {
	r.raftLog.commitTo(m.Commit)
	r.send(pb.Message{To: m.From, Type: pb.MsgHeartbeatResp})
}

func (r *raft) handleSnapshot(m pb.Message) {
	sindex, sterm := m.Snapshot.Metadata.Index, m.Snapshot.Metadata.Term
	if r.restore(m.Snapshot) {
		raftLogger.Infof("raft: %x [commit: %d] restored snapshot [index: %d, term: %d]",
			r.id, r.Commit, sindex, sterm)
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: r.raftLog.lastIndex()})
	} else {
		raftLogger.Infof("raft: %x [commit: %d] ignored snapshot [index: %d, term: %d]",
			r.id, r.Commit, sindex, sterm)
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: r.raftLog.committed})
	}
}

// restore recovers the state machine from a snapshot. It restores the log and the
// configuration of state machine.
func (r *raft) restore(s pb.Snapshot) bool {
	if s.Metadata.Index <= r.raftLog.committed {
		return false
	}
	if r.raftLog.matchTerm(s.Metadata.Index, s.Metadata.Term) {
		raftLogger.Infof("raft: %x [commit: %d, lastindex: %d, lastterm: %d] fast-forwarded commit to snapshot [index: %d, term: %d]",
			r.id, r.Commit, r.raftLog.lastIndex(), r.raftLog.lastTerm(), s.Metadata.Index, s.Metadata.Term)
		r.raftLog.commitTo(s.Metadata.Index)
		return false
	}

	raftLogger.Infof("raft: %x [commit: %d, lastindex: %d, lastterm: %d] starts to restore snapshot [index: %d, term: %d]",
		r.id, r.Commit, r.raftLog.lastIndex(), r.raftLog.lastTerm(), s.Metadata.Index, s.Metadata.Term)

	r.raftLog.restore(s)
	r.prs = make(map[uint64]*Progress)
	for _, n := range s.Metadata.ConfState.Nodes {
		match, next := uint64(0), uint64(r.raftLog.lastIndex())+1
		if n == r.id {
			match = next - 1
		} else {
			match = 0
		}
		r.setProgress(n, match, next)
		raftLogger.Infof("raft: %x restored progress of %x [%s]", r.id, n, r.prs[n])
	}
	return true
}

func (r *raft) needSnapshot(i uint64) bool {
	return i < r.raftLog.firstIndex()
}

// promotable indicates whether state machine can be promoted to leader,
// which is true when its own id is in progress list.
func (r *raft) promotable() bool {
	_, ok := r.prs[r.id]
	return ok
}

func (r *raft) addNode(id uint64) {
	if _, ok := r.prs[id]; ok {
		// Ignore any redundant addNode calls (which can happen because the
		// initial bootstrapping entries are applied twice).
		return
	}

	r.setProgress(id, 0, r.raftLog.lastIndex()+1)
	r.pendingConf = false
}

func (r *raft) removeNode(id uint64) {
	r.delProgress(id)
	r.pendingConf = false
}

func (r *raft) resetPendingConf() { r.pendingConf = false }

func (r *raft) setProgress(id, match, next uint64) {
	r.prs[id] = &Progress{Next: next, Match: match}
}

func (r *raft) delProgress(id uint64) {
	delete(r.prs, id)
}

func (r *raft) loadState(state pb.HardState) {
	if state.Commit < r.raftLog.committed || state.Commit > r.raftLog.lastIndex() {
		raftLogger.Panicf("raft: %x state.commit %d is out of range [%d, %d]", r.id, state.Commit, r.raftLog.committed, r.raftLog.lastIndex())
	}
	r.raftLog.committed = state.Commit
	r.Term = state.Term
	r.Vote = state.Vote
	r.Commit = state.Commit
}

// isElectionTimeout returns true if r.elapsed is greater than the
// randomized election timeout in (electiontimeout, 2 * electiontimeout - 1).
// Otherwise, it returns false.
func (r *raft) isElectionTimeout() bool {
	d := r.elapsed - r.electionTimeout
	if d < 0 {
		return false
	}
	return d > r.rand.Int()%r.electionTimeout
}
