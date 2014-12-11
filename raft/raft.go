/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package raft

import (
	"errors"
	"fmt"
	"log"
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

func (st StateType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", st.String())), nil
}

type progress struct{ match, next uint64 }

func (pr *progress) update(n uint64) {
	if pr.match < n {
		pr.match = n
	}
	if pr.next < n+1 {
		pr.next = n + 1
	}
}

func (pr *progress) optimisticUpdate(n uint64) { pr.next = n + 1 }

// maybeDecrTo returns false if the given to index comes from an out of order message.
// Otherwise it decreases the progress next index and returns true.
func (pr *progress) maybeDecrTo(to uint64) bool {
	if pr.match != 0 {
		// the rejection must be stale if the progress has matched and "to"
		// is smaller than "match".
		if to <= pr.match {
			return false
		}
		// directly decrease next to match + 1
		pr.next = pr.match + 1
		return true
	}

	// the rejection must be stale if "to" does not match next - 1
	if pr.next-1 != to {
		return false
	}

	if pr.next--; pr.next < 1 {
		pr.next = 1
	}
	return true
}

func (pr *progress) String() string { return fmt.Sprintf("next = %d, match = %d", pr.next, pr.match) }

type raft struct {
	pb.HardState

	id uint64

	// the log
	raftLog *raftLog

	prs map[uint64]*progress

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

func newRaft(id uint64, peers []uint64, election, heartbeat int, storage Storage) *raft {
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
		prs:              make(map[uint64]*progress),
		electionTimeout:  election,
		heartbeatTimeout: heartbeat,
	}
	r.rand = rand.New(rand.NewSource(int64(id)))
	for _, p := range peers {
		r.prs[p] = &progress{next: 1}
	}
	if !isHardStateEqual(hs, emptyState) {
		r.loadState(hs)
	}
	r.becomeFollower(r.Term, None)

	nodesStrs := make([]string, 0)
	for _, n := range r.nodes() {
		nodesStrs = append(nodesStrs, fmt.Sprintf("%x", n))
	}

	log.Printf("raft: newRaft %x [peers: [%s], term: %d, commit: %d, lastindex: %d, lastterm: %d]",
		r.id, strings.Join(nodesStrs, ","), r.Term, r.raftLog.committed, r.raftLog.lastIndex(), r.raftLog.lastTerm())
	return r
}

func (r *raft) hasLeader() bool { return r.lead != None }

func (r *raft) leader() uint64 { return r.lead }

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
	m := pb.Message{}
	m.To = to
	if r.needSnapshot(pr.next) {
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
		log.Printf("raft: %x [firstindex: %d, commit: %d] sent snapshot[index: %d, term: %d] to %x [%s]",
			r.id, r.raftLog.firstIndex(), r.Commit, sindex, sterm, to, pr)
	} else {
		m.Type = pb.MsgApp
		m.Index = pr.next - 1
		m.LogTerm = r.raftLog.term(pr.next - 1)
		m.Entries = r.raftLog.entries(pr.next)
		m.Commit = r.raftLog.committed
		// optimistically increase the next if the follower
		// has been matched.
		if n := len(m.Entries); pr.match != 0 && n != 0 {
			pr.optimisticUpdate(m.Entries[n-1].Index)
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
	commit := min(r.prs[to].match, r.raftLog.committed)
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
	}
}

func (r *raft) maybeCommit() bool {
	// TODO(bmizerany): optimize.. Currently naive
	mis := make(uint64Slice, 0, len(r.prs))
	for i := range r.prs {
		mis = append(mis, r.prs[i].match)
	}
	sort.Sort(sort.Reverse(mis))
	mci := mis[r.q()-1]

	return r.raftLog.maybeCommit(mci, r.Term)
}

func (r *raft) reset(term uint64) {
	r.Term = term
	r.lead = None
	r.Vote = None
	r.elapsed = 0
	r.votes = make(map[uint64]bool)
	for i := range r.prs {
		r.prs[i] = &progress{next: r.raftLog.lastIndex() + 1}
		if i == r.id {
			r.prs[i].match = r.raftLog.lastIndex()
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
	if r.elapsed > r.heartbeatTimeout {
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
	log.Printf("raft: %x became follower at term %d", r.id, r.Term)
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
	log.Printf("raft: %x became candidate at term %d", r.id, r.Term)
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
	log.Printf("raft: %x became leader at term %d", r.id, r.Term)
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
		log.Printf("raft: %x [logterm: %d, index: %d] sent vote request to %x at term %d",
			r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), i, r.Term)
		r.send(pb.Message{To: i, Type: pb.MsgVote, Index: r.raftLog.lastIndex(), LogTerm: r.raftLog.lastTerm()})
	}
}

func (r *raft) poll(id uint64, v bool) (granted int) {
	if v {
		log.Printf("raft: %x received vote from %x at term %d", r.id, id, r.Term)
	} else {
		log.Printf("raft: %x received vote rejection from %x at term %d", r.id, id, r.Term)
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
		log.Printf("raft: %x is starting a new election at term %d", r.id, r.Term)
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
		log.Printf("raft: %x [term: %d] received a %s message with higher term from %x [term: %d]",
			r.id, r.Term, m.Type, m.From, m.Term)
		r.becomeFollower(m.Term, lead)
	case m.Term < r.Term:
		// ignore
		log.Printf("raft: %x [term: %d] ignored a %s message with lower term from %x [term: %d]",
			r.id, r.Term, m.Type, m.From, m.Term)
		return nil
	}
	r.step(r, m)
	r.Commit = r.raftLog.committed
	return nil
}

type stepFunc func(r *raft, m pb.Message)

func stepLeader(r *raft, m pb.Message) {
	switch m.Type {
	case pb.MsgBeat:
		r.bcastHeartbeat()
	case pb.MsgProp:
		if len(m.Entries) == 0 {
			log.Panicf("raft: %x stepped empty MsgProp", r.id)
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
		if m.Reject {
			log.Printf("raft: %x received msgApp rejection from %x for index %d",
				r.id, m.From, m.Index)
			if r.prs[m.From].maybeDecrTo(m.Index) {
				r.sendAppend(m.From)
			}
		} else {
			r.prs[m.From].update(m.Index)
			if r.maybeCommit() {
				r.bcastAppend()
			}
		}
	case pb.MsgVote:
		log.Printf("raft: %x [logterm: %d, index: %d, vote: %x] rejected vote from %x [logterm: %d, index: %d] at term %x",
			r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
		r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp, Reject: true})
	}
}

func stepCandidate(r *raft, m pb.Message) {
	switch m.Type {
	case pb.MsgProp:
		log.Printf("raft: %x no leader at term %d; dropping proposal", r.id, r.Term)
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
		log.Printf("raft: %x [logterm: %d, index: %d, vote: %x] rejected vote from %x [logterm: %d, index: %d] at term %x",
			r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
		r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp, Reject: true})
	case pb.MsgVoteResp:
		gr := r.poll(m.From, !m.Reject)
		log.Printf("raft: %x [q:%d] has received %d votes and %d vote rejections", r.id, r.q(), gr, len(r.votes)-gr)
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
			log.Printf("raft: %x no leader at term %d; dropping proposal", r.id, r.Term)
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
			log.Printf("raft: %x [logterm: %d, index: %d, vote: %x] voted for %x [logterm: %d, index: %d] at term %x",
				r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
			r.Vote = m.From
			r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp})
		} else {
			log.Printf("raft: %x [logterm: %d, index: %d, vote: %x] rejected vote from %x [logterm: %d, index: %d] at term %x",
				r.id, r.raftLog.lastTerm(), r.raftLog.lastIndex(), r.Vote, m.From, m.LogTerm, m.Index, r.Term)
			r.send(pb.Message{To: m.From, Type: pb.MsgVoteResp, Reject: true})
		}
	}
}

func (r *raft) handleAppendEntries(m pb.Message) {
	if mlastIndex, ok := r.raftLog.maybeAppend(m.Index, m.LogTerm, m.Commit, m.Entries...); ok {
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: mlastIndex})
	} else {
		log.Printf("raft: %x [logterm: %d, index: %d] rejected msgApp [logterm: %d, index: %d] from %x",
			r.id, r.raftLog.term(m.Index), m.Index, m.LogTerm, m.Index, m.From)
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: m.Index, Reject: true})
	}
}

func (r *raft) handleHeartbeat(m pb.Message) {
	r.raftLog.commitTo(m.Commit)
}

func (r *raft) handleSnapshot(m pb.Message) {
	sindex, sterm := m.Snapshot.Metadata.Index, m.Snapshot.Metadata.Term
	if r.restore(m.Snapshot) {
		log.Printf("raft: %x [commit: %d] restored snapshot [index: %d, term: %d]",
			r.id, r.Commit, sindex, sterm)
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: r.raftLog.lastIndex()})
	} else {
		log.Printf("raft: %x [commit: %d] ignored snapshot [index: %d, term: %d]",
			r.id, r.Commit, sindex, sterm)
		r.send(pb.Message{To: m.From, Type: pb.MsgAppResp, Index: r.raftLog.committed})
	}
}

// restore recovers the statemachine from a snapshot. It restores the log and the
// configuration of statemachine.
func (r *raft) restore(s pb.Snapshot) bool {
	if s.Metadata.Index <= r.raftLog.committed {
		return false
	}
	if r.raftLog.matchTerm(s.Metadata.Index, s.Metadata.Term) {
		log.Printf("raft: %x [commit: %d, lastindex: %d, lastterm: %d] fast-forwarded commit to snapshot [index: %d, term: %d]",
			r.id, r.Commit, r.raftLog.lastIndex(), r.raftLog.lastTerm(), s.Metadata.Index, s.Metadata.Term)
		r.raftLog.commitTo(s.Metadata.Index)
		return false
	}

	log.Printf("raft: %x [commit: %d, lastindex: %d, lastterm: %d] starts to restore snapshot [index: %d, term: %d]",
		r.id, r.Commit, r.raftLog.lastIndex(), r.raftLog.lastTerm(), s.Metadata.Index, s.Metadata.Term)

	r.raftLog.restore(s)
	r.prs = make(map[uint64]*progress)
	for _, n := range s.Metadata.ConfState.Nodes {
		match, next := uint64(0), uint64(r.raftLog.lastIndex())+1
		if n == r.id {
			match = next - 1
		} else {
			match = 0
		}
		r.setProgress(n, match, next)
		log.Printf("raft: %x restored progress of %x [%s]", r.id, n, r.prs[n])
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
	r.prs[id] = &progress{next: next, match: match}
}

func (r *raft) delProgress(id uint64) {
	delete(r.prs, id)
}

func (r *raft) loadState(state pb.HardState) {
	if state.Commit < r.raftLog.committed || state.Commit > r.raftLog.lastIndex() {
		log.Panicf("raft: %x state.commit %d is out of range [%d, %d]", r.id, state.Commit, r.raftLog.committed, r.raftLog.lastIndex())
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
