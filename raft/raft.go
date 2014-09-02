package raft

import (
	"errors"
	"fmt"
	"sort"

	pb "github.com/coreos/etcd/raft/raftpb"
)

const none = -1

type messageType int64

const (
	msgHup int64 = iota
	msgBeat
	msgProp
	msgApp
	msgAppResp
	msgVote
	msgVoteResp
	msgSnap
	msgDenied
)

var mtmap = [...]string{
	msgHup:      "msgHup",
	msgBeat:     "msgBeat",
	msgProp:     "msgProp",
	msgApp:      "msgApp",
	msgAppResp:  "msgAppResp",
	msgVote:     "msgVote",
	msgVoteResp: "msgVoteResp",
	msgSnap:     "msgSnap",
	msgDenied:   "msgDenied",
}

func (mt messageType) String() string {
	return mtmap[int64(mt)]
}

var errNoLeader = errors.New("no leader")

const (
	stateFollower stateType = iota
	stateCandidate
	stateLeader
)

type stateType int64

var stmap = [...]string{
	stateFollower:  "stateFollower",
	stateCandidate: "stateCandidate",
	stateLeader:    "stateLeader",
}

var stepmap = [...]stepFunc{
	stateFollower:  stepFollower,
	stateCandidate: stepCandidate,
	stateLeader:    stepLeader,
}

func (st stateType) String() string {
	return stmap[int64(st)]
}

type progress struct {
	match, next int64
}

func (pr *progress) update(n int64) {
	pr.match = n
	pr.next = n + 1
}

func (pr *progress) decr() {
	if pr.next--; pr.next < 1 {
		pr.next = 1
	}
}

func (pr *progress) String() string {
	return fmt.Sprintf("n=%d m=%d", pr.next, pr.match)
}

// int64Slice implements sort interface
type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type raft struct {
	pb.State

	id int64

	// the term we are participating in at any time
	index int64

	// the log
	raftLog *raftLog

	prs map[int64]*progress

	state stateType

	votes map[int64]bool

	msgs []pb.Message

	// the leader id
	lead int64

	// pending reconfiguration
	configuring bool

	// promotable indicates whether state machine could be promoted.
	// New machine has to wait until it has been added to the cluster, or it
	// may become the leader of the cluster without it.
	promotable bool

	elapsed          int
	heartbeatTimeout int
	electionTimeout  int
	tick             func()
}

func newRaft(id int64, peers []int64, election, heartbeat int) *raft {
	if id == none {
		panic("cannot use none id")
	}
	r := &raft{
		id:               id,
		lead:             none,
		raftLog:          newLog(),
		prs:              make(map[int64]*progress),
		electionTimeout:  election,
		heartbeatTimeout: heartbeat,
	}
	for _, p := range peers {
		r.prs[p] = &progress{}
	}
	r.becomeFollower(0, none)
	return r
}

func (r *raft) hasLeader() bool { return r.lead != none }

func (r *raft) String() string {
	s := fmt.Sprintf(`state=%v term=%d`, r.state, r.Term)
	switch r.state {
	case stateFollower:
		s += fmt.Sprintf(" vote=%v lead=%v", r.Vote, r.lead)
	case stateCandidate:
		s += fmt.Sprintf(` votes="%v"`, r.votes)
	case stateLeader:
		s += fmt.Sprintf(` prs="%v"`, r.prs)
	}
	return s
}

func (r *raft) poll(id int64, v bool) (granted int) {
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

// send persists state to stable storage and then sends to its mailbox.
func (r *raft) send(m pb.Message) {
	m.From = r.id
	m.Term = r.Term
	r.msgs = append(r.msgs, m)
}

// sendAppend sends RRPC, with entries to the given peer.
func (r *raft) sendAppend(to int64) {
	pr := r.prs[to]
	m := pb.Message{}
	m.To = to
	m.Index = pr.next - 1
	if r.needSnapshot(m.Index) {
		m.Type = msgSnap
		m.Snapshot = r.raftLog.snapshot
	} else {
		m.Type = msgApp
		m.LogTerm = r.raftLog.term(pr.next - 1)
		m.Entries = r.raftLog.entries(pr.next)
		m.Commit = r.raftLog.committed
	}
	r.send(m)
}

// sendHeartbeat sends RRPC, without entries to the given peer.
func (r *raft) sendHeartbeat(to int64) {
	pr := r.prs[to]
	index := max(pr.next-1, r.raftLog.lastIndex())
	m := pb.Message{
		To:      to,
		Type:    msgApp,
		Index:   index,
		LogTerm: r.raftLog.term(index),
		Commit:  r.raftLog.committed,
	}
	r.send(m)
}

// bcastAppend sends RRPC, with entries to all peers that are not up-to-date according to r.mis.
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
	mis := make(int64Slice, 0, len(r.prs))
	for i := range r.prs {
		mis = append(mis, r.prs[i].match)
	}
	sort.Sort(sort.Reverse(mis))
	mci := mis[r.q()-1]

	return r.raftLog.maybeCommit(mci, r.Term)
}

func (r *raft) reset(term int64) {
	r.Term = term
	r.lead = none
	r.Vote = none
	r.votes = make(map[int64]bool)
	for i := range r.prs {
		r.prs[i] = &progress{next: r.raftLog.lastIndex() + 1}
		if i == r.id {
			r.prs[i].match = r.raftLog.lastIndex()
		}
	}
}

func (r *raft) q() int {
	return len(r.prs)/2 + 1
}

func (r *raft) appendEntry(e pb.Entry) {
	e.Term = r.Term
	e.Index = r.raftLog.lastIndex() + 1
	r.LastIndex = r.raftLog.append(r.raftLog.lastIndex(), e)
	r.prs[r.id].update(r.raftLog.lastIndex())
	r.maybeCommit()
}

func (r *raft) tickElection() {
	r.elapsed++
	if r.elapsed > r.electionTimeout {
		r.elapsed = 0
		r.campaign()
	}
}

func (r *raft) tickHeartbeat() {
	r.elapsed++
	if r.elapsed > r.heartbeatTimeout {
		r.elapsed = 0
		r.bcastHeartbeat()
	}
}

func (r *raft) setTick(f func()) {
	r.elapsed = 0
	r.tick = f
}

func (r *raft) becomeFollower(term int64, lead int64) {
	r.setTick(r.tickElection)
	r.reset(term)
	r.lead = lead
	r.state = stateFollower
	r.configuring = false
}

func (r *raft) becomeCandidate() {
	r.setTick(r.tickElection)
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if r.state == stateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	r.reset(r.Term + 1)
	r.Vote = r.id
	r.state = stateCandidate
}

func (r *raft) becomeLeader() {
	r.setTick(r.tickHeartbeat)
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if r.state == stateFollower {
		panic("invalid transition [follower -> leader]")
	}
	r.reset(r.Term)
	r.lead = r.id
	r.state = stateLeader

	for _, e := range r.raftLog.entries(r.raftLog.committed + 1) {
		if isConfig(e) {
			r.configuring = true
		}
	}

	r.appendEntry(pb.Entry{Type: Normal, Data: nil})
}

func (r *raft) ReadMessages() []pb.Message {
	msgs := r.msgs
	r.msgs = make([]pb.Message, 0)

	return msgs
}

func (r *raft) campaign() {
	r.becomeCandidate()
	if r.q() == r.poll(r.id, true) {
		r.becomeLeader()
	}
	for i := range r.prs {
		if i == r.id {
			continue
		}
		lasti := r.raftLog.lastIndex()
		r.send(pb.Message{To: i, Type: msgVote, Index: lasti, LogTerm: r.raftLog.term(lasti)})
	}
}

func (r *raft) Step(m pb.Message) error {
	// TODO(bmizerany): this likely allocs - prevent that.
	defer func() { r.Commit = r.raftLog.committed }()

	if m.Type == msgHup {
		r.campaign()
	}

	switch {
	case m.Term == 0:
		// local message
	case m.Term > r.Term:
		lead := m.From
		if m.Type == msgVote {
			lead = none
		}
		r.becomeFollower(m.Term, lead)
	case m.Term < r.Term:
		// ignore
	}

	stepmap[r.state](r, m)
	return nil
}

func (r *raft) handleAppendEntries(m pb.Message) {
	if r.raftLog.maybeAppend(m.Index, m.LogTerm, m.Commit, m.Entries...) {
		r.LastIndex = r.raftLog.lastIndex()
		r.send(pb.Message{To: m.From, Type: msgAppResp, Index: r.raftLog.lastIndex()})
	} else {
		r.send(pb.Message{To: m.From, Type: msgAppResp, Index: -1})
	}
}

func (r *raft) handleSnapshot(m pb.Message) {
	if r.restore(m.Snapshot) {
		r.send(pb.Message{To: m.From, Type: msgAppResp, Index: r.raftLog.lastIndex()})
	} else {
		r.send(pb.Message{To: m.From, Type: msgAppResp, Index: r.raftLog.committed})
	}
}

func (r *raft) addNode(id int64) {
	r.setProgress(id, 0, r.raftLog.lastIndex()+1)
	r.configuring = false
	if id == r.id {
		r.promotable = true
	}
}

func (r *raft) removeNode(id int64) {
	r.delProgress(id)
	r.configuring = false
}

type stepFunc func(r *raft, m pb.Message)

func stepLeader(r *raft, m pb.Message) {
	switch m.Type {
	case msgBeat:
		r.bcastHeartbeat()
	case msgProp:
		if len(m.Entries) != 1 {
			panic("unexpected length(entries) of a msgProp")
		}
		e := m.Entries[0]
		if isConfig(e) {
			if r.configuring {
				panic("pending conf")
			}
			r.configuring = true
		}
		r.appendEntry(e)
		r.bcastAppend()
	case msgAppResp:
		if m.Index < 0 {
			r.prs[m.From].decr()
			r.sendAppend(m.From)
		} else {
			r.prs[m.From].update(m.Index)
			if r.maybeCommit() {
				r.bcastAppend()
			}
		}
	case msgVote:
		r.send(pb.Message{To: m.From, Type: msgVoteResp, Index: -1})
	}
}

func stepCandidate(r *raft, m pb.Message) {
	switch m.Type {
	case msgProp:
		panic("no leader")
	case msgApp:
		r.elapsed = 0
		r.becomeFollower(r.Term, m.From)
		r.handleAppendEntries(m)
	case msgSnap:
		r.becomeFollower(m.Term, m.From)
		r.handleSnapshot(m)
	case msgVote:
		r.send(pb.Message{To: m.From, Type: msgVoteResp, Index: -1})
	case msgVoteResp:
		gr := r.poll(m.From, m.Index >= 0)
		switch r.q() {
		case gr:
			r.becomeLeader()
			r.bcastAppend()
		case len(r.votes) - gr:
			r.becomeFollower(r.Term, none)
		}
	}
}

func stepFollower(r *raft, m pb.Message) {
	switch m.Type {
	case msgProp:
		if r.lead == none {
			panic("no leader")
		}
		m.To = r.lead
		r.send(m)
	case msgApp:
		r.elapsed = 0
		r.lead = m.From
		r.handleAppendEntries(m)
	case msgSnap:
		r.elapsed = 0
		r.handleSnapshot(m)
	case msgVote:
		// TODO(xiang): maybe reset elapsed?
		if (r.Vote == none || r.Vote == m.From) && r.raftLog.isUpToDate(m.Index, m.LogTerm) {
			r.Vote = m.From
			r.send(pb.Message{To: m.From, Type: msgVoteResp, Index: r.raftLog.lastIndex()})
		} else {
			r.send(pb.Message{To: m.From, Type: msgVoteResp, Index: -1})
		}
	}
}

func (r *raft) compact(d []byte) {
	r.raftLog.snap(d, r.raftLog.applied, r.raftLog.term(r.raftLog.applied), r.nodes())
	r.raftLog.compact(r.raftLog.applied)
}

// restore recovers the statemachine from a snapshot. It restores the log and the
// configuration of statemachine.
func (r *raft) restore(s pb.Snapshot) bool {
	if s.Index <= r.raftLog.committed {
		return false
	}

	r.raftLog.restore(s)
	r.LastIndex = r.raftLog.lastIndex()
	r.prs = make(map[int64]*progress)
	for _, n := range s.Nodes {
		if n == r.id {
			r.setProgress(n, r.raftLog.lastIndex(), r.raftLog.lastIndex()+1)
		} else {
			r.setProgress(n, 0, r.raftLog.lastIndex()+1)
		}
	}
	r.configuring = false
	return true
}

func (r *raft) needSnapshot(i int64) bool {
	if i < r.raftLog.offset {
		if r.raftLog.snapshot.Term == 0 {
			panic("need non-empty snapshot")
		}
		return true
	}
	return false
}

func (r *raft) nodes() []int64 {
	nodes := make([]int64, 0, len(r.prs))
	for k := range r.prs {
		nodes = append(nodes, k)
	}
	return nodes
}

func (r *raft) setProgress(id, match, next int64) {
	r.prs[id] = &progress{next: next, match: match}
}

func (r *raft) delProgress(id int64) {
	delete(r.prs, id)
}

func (r *raft) loadEnts(ents []pb.Entry) {
	if !r.raftLog.isEmpty() {
		panic("cannot load entries when log is not empty")
	}
	r.raftLog.append(0, ents...)
	r.raftLog.unstable = r.raftLog.lastIndex() + 1
}

func (r *raft) loadState(state pb.State) {
	r.raftLog.committed = state.Commit
	r.Term = state.Term
	r.Vote = state.Vote
}
