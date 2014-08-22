package raft

import (
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
)

const none = -1

type messageType int64

const (
	msgHup messageType = iota
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

var EmptyState = State{}

type Message struct {
	Type      messageType
	ClusterId int64
	To        int64
	From      int64
	Term      int64
	LogTerm   int64
	Index     int64
	Entries   []Entry
	Commit    int64
	Snapshot  Snapshot
}

func (m Message) IsMsgApp() bool {
	return m.Type == msgApp
}

func (m Message) String() string {
	return fmt.Sprintf("type=%v from=%x to=%x term=%d logTerm=%d i=%d ci=%d len(ents)=%d",
		m.Type, m.From, m.To, m.Term, m.LogTerm, m.Index, m.Commit, len(m.Entries))
}

type index struct {
	match, next int64
}

func (in *index) update(n int64) {
	in.match = n
	in.next = n + 1
}

func (in *index) decr() {
	if in.next--; in.next < 1 {
		in.next = 1
	}
}

func (in *index) String() string {
	return fmt.Sprintf("n=%d m=%d", in.next, in.match)
}

// An AtomicInt is an int64 to be accessed atomically.
type atomicInt int64

func (i *atomicInt) Set(n int64) {
	atomic.StoreInt64((*int64)(i), n)
}

func (i *atomicInt) Get() int64 {
	return atomic.LoadInt64((*int64)(i))
}

// int64Slice implements sort interface
type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type raft struct {
	State

	// --- new stuff ---
	name      string
	election  int
	heartbeat int
	// -----------------

	clusterId int64
	id        int64

	// the term we are participating in at any time
	index atomicInt

	// the log
	raftLog *raftLog

	ins map[int64]*index

	state stateType

	votes map[int64]bool

	msgs []Message

	// the leader id
	lead atomicInt

	// pending reconfiguration
	pendingConf bool

	unstableState State

	// promotable indicates whether state machine could be promoted.
	// New machine has to wait until it has been added to the cluster, or it
	// may become the leader of the cluster without it.
	promotable bool
}

func newStateMachine(id int64, peers []int64) *raft {
	if id == none {
		panic("cannot use none id")
	}
	sm := &raft{id: id, clusterId: none, lead: none, raftLog: newLog(), ins: make(map[int64]*index)}
	for _, p := range peers {
		sm.ins[p] = &index{}
	}
	sm.reset(0)
	return sm
}

func (r *raft) hasLeader() bool { return r.state != stateCandidate }

func (r *raft) propose(data []byte) {
	r.Step(Message{From: r.id, Type: msgProp, Entries: []Entry{{Data: data}}})
}

func (sm *raft) String() string {
	s := fmt.Sprintf(`state=%v term=%d`, sm.state, sm.Term)
	switch sm.state {
	case stateFollower:
		s += fmt.Sprintf(" vote=%v lead=%v", sm.Vote, sm.lead)
	case stateCandidate:
		s += fmt.Sprintf(` votes="%v"`, sm.votes)
	case stateLeader:
		s += fmt.Sprintf(` ins="%v"`, sm.ins)
	}
	return s
}

func (sm *raft) poll(id int64, v bool) (granted int) {
	if _, ok := sm.votes[id]; !ok {
		sm.votes[id] = v
	}
	for _, vv := range sm.votes {
		if vv {
			granted++
		}
	}
	return granted
}

// send persists state to stable storage and then sends to its mailbox.
func (sm *raft) send(m Message) {
	m.ClusterId = sm.clusterId
	m.From = sm.id
	m.Term = sm.Term
	sm.msgs = append(sm.msgs, m)
}

// sendAppend sends RRPC, with entries to the given peer.
func (sm *raft) sendAppend(to int64) {
	in := sm.ins[to]
	m := Message{}
	m.To = to
	m.Index = in.next - 1
	if sm.needSnapshot(m.Index) {
		m.Type = msgSnap
		m.Snapshot = sm.raftLog.snapshot
	} else {
		m.Type = msgApp
		m.LogTerm = sm.raftLog.term(in.next - 1)
		m.Entries = sm.raftLog.entries(in.next)
		m.Commit = sm.raftLog.committed
	}
	sm.send(m)
}

// sendHeartbeat sends RRPC, without entries to the given peer.
func (sm *raft) sendHeartbeat(to int64) {
	in := sm.ins[to]
	index := max(in.next-1, sm.raftLog.lastIndex())
	m := Message{
		To:      to,
		Type:    msgApp,
		Index:   index,
		LogTerm: sm.raftLog.term(index),
		Commit:  sm.raftLog.committed,
	}
	sm.send(m)
}

// bcastAppend sends RRPC, with entries to all peers that are not up-to-date according to sm.mis.
func (sm *raft) bcastAppend() {
	for i := range sm.ins {
		if i == sm.id {
			continue
		}
		sm.sendAppend(i)
	}
}

// bcastHeartbeat sends RRPC, without entries to all the peers.
func (sm *raft) bcastHeartbeat() {
	for i := range sm.ins {
		if i == sm.id {
			continue
		}
		sm.sendHeartbeat(i)
	}
}

func (sm *raft) maybeCommit() bool {
	// TODO(bmizerany): optimize.. Currently naive
	mis := make(int64Slice, 0, len(sm.ins))
	for i := range sm.ins {
		mis = append(mis, sm.ins[i].match)
	}
	sort.Sort(sort.Reverse(mis))
	mci := mis[sm.q()-1]

	return sm.raftLog.maybeCommit(mci, sm.Term)
}

// nextEnts returns the appliable entries and updates the applied index
func (sm *raft) nextEnts() (ents []Entry) {
	ents = sm.raftLog.nextEnts()
	sm.raftLog.resetNextEnts()
	return ents
}

func (sm *raft) reset(term int64) {
	sm.setTerm(term)
	sm.lead.Set(none)
	sm.setVote(none)
	sm.votes = make(map[int64]bool)
	for i := range sm.ins {
		sm.ins[i] = &index{next: sm.raftLog.lastIndex() + 1}
		if i == sm.id {
			sm.ins[i].match = sm.raftLog.lastIndex()
		}
	}
}

func (sm *raft) q() int {
	return len(sm.ins)/2 + 1
}

func (sm *raft) appendEntry(e Entry) {
	e.Term = sm.Term
	e.Index = sm.raftLog.lastIndex() + 1
	sm.LastIndex = sm.raftLog.append(sm.raftLog.lastIndex(), e)
	sm.ins[sm.id].update(sm.raftLog.lastIndex())
	sm.maybeCommit()
}

func (sm *raft) becomeFollower(term int64, lead int64) {
	sm.reset(term)
	sm.lead.Set(lead)
	sm.state = stateFollower
	sm.pendingConf = false
}

func (sm *raft) becomeCandidate() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	sm.reset(sm.Term + 1)
	sm.setVote(sm.id)
	sm.state = stateCandidate
}

func (sm *raft) becomeLeader() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateFollower {
		panic("invalid transition [follower -> leader]")
	}
	sm.reset(sm.Term)
	sm.lead.Set(sm.id)
	sm.state = stateLeader

	for _, e := range sm.raftLog.entries(sm.raftLog.committed + 1) {
		if e.isConfig() {
			sm.pendingConf = true
		}
	}

	sm.appendEntry(Entry{Type: Normal, Data: nil})
}

func (sm *raft) ReadMessages() []Message {
	msgs := sm.msgs
	sm.msgs = make([]Message, 0)

	return msgs
}

func (sm *raft) Step(m Message) error {
	if m.Type == msgHup {
		sm.becomeCandidate()
		if sm.q() == sm.poll(sm.id, true) {
			sm.becomeLeader()
		}
		for i := range sm.ins {
			if i == sm.id {
				continue
			}
			lasti := sm.raftLog.lastIndex()
			sm.send(Message{To: i, Type: msgVote, Index: lasti, LogTerm: sm.raftLog.term(lasti)})
		}
	}

	switch {
	case m.Term == 0:
		// local message
	case m.Term > sm.Term:
		lead := m.From
		if m.Type == msgVote {
			lead = none
		}
		sm.becomeFollower(m.Term, lead)
	case m.Term < sm.Term:
		// ignore
	}

	stepmap[sm.state](sm, m)
	return nil
}

func (sm *raft) handleAppendEntries(m Message) {
	if sm.raftLog.maybeAppend(m.Index, m.LogTerm, m.Commit, m.Entries...) {
		sm.LastIndex = sm.raftLog.lastIndex()
		sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.raftLog.lastIndex()})
	} else {
		sm.send(Message{To: m.From, Type: msgAppResp, Index: -1})
	}
}

func (sm *raft) handleSnapshot(m Message) {
	if sm.restore(m.Snapshot) {
		sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.raftLog.lastIndex()})
	} else {
		sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.raftLog.committed})
	}
}

func (sm *raft) addNode(id int64) {
	sm.addIns(id, 0, sm.raftLog.lastIndex()+1)
	sm.pendingConf = false
	if id == sm.id {
		sm.promotable = true
	}
}

func (sm *raft) removeNode(id int64) {
	sm.deleteIns(id)
	sm.pendingConf = false
}

type stepFunc func(sm *raft, m Message) bool

func stepLeader(sm *raft, m Message) bool {
	switch m.Type {
	case msgBeat:
		sm.bcastHeartbeat()
	case msgProp:
		if len(m.Entries) != 1 {
			panic("unexpected length(entries) of a msgProp")
		}
		e := m.Entries[0]
		if e.isConfig() {
			if sm.pendingConf {
				return false
			}
			sm.pendingConf = true
		}
		sm.appendEntry(e)
		sm.bcastAppend()
	case msgAppResp:
		if m.Index < 0 {
			sm.ins[m.From].decr()
			sm.sendAppend(m.From)
		} else {
			sm.ins[m.From].update(m.Index)
			if sm.maybeCommit() {
				sm.bcastAppend()
			}
		}
	case msgVote:
		sm.send(Message{To: m.From, Type: msgVoteResp, Index: -1})
	}
	return true
}

func stepCandidate(sm *raft, m Message) bool {
	switch m.Type {
	case msgProp:
		return false
	case msgApp:
		sm.becomeFollower(sm.Term, m.From)
		sm.handleAppendEntries(m)
	case msgSnap:
		sm.becomeFollower(m.Term, m.From)
		sm.handleSnapshot(m)
	case msgVote:
		sm.send(Message{To: m.From, Type: msgVoteResp, Index: -1})
	case msgVoteResp:
		gr := sm.poll(m.From, m.Index >= 0)
		switch sm.q() {
		case gr:
			sm.becomeLeader()
			sm.bcastAppend()
		case len(sm.votes) - gr:
			sm.becomeFollower(sm.Term, none)
		}
	}
	return true
}

func stepFollower(sm *raft, m Message) bool {
	switch m.Type {
	case msgProp:
		if sm.lead.Get() == none {
			return false
		}
		m.To = sm.lead.Get()
		sm.send(m)
	case msgApp:
		sm.lead.Set(m.From)
		sm.handleAppendEntries(m)
	case msgSnap:
		sm.handleSnapshot(m)
	case msgVote:
		if (sm.Vote == none || sm.Vote == m.From) && sm.raftLog.isUpToDate(m.Index, m.LogTerm) {
			sm.setVote(m.From)
			sm.send(Message{To: m.From, Type: msgVoteResp, Index: sm.raftLog.lastIndex()})
		} else {
			sm.send(Message{To: m.From, Type: msgVoteResp, Index: -1})
		}
	}
	return true
}

func (sm *raft) compact(d []byte) {
	sm.raftLog.snap(d, sm.raftLog.applied, sm.raftLog.term(sm.raftLog.applied), sm.nodes())
	sm.raftLog.compact(sm.raftLog.applied)
}

// restore recovers the statemachine from a snapshot. It restores the log and the
// configuration of statemachine.
func (sm *raft) restore(s Snapshot) bool {
	if s.Index <= sm.raftLog.committed {
		return false
	}

	sm.raftLog.restore(s)
	sm.LastIndex = sm.raftLog.lastIndex()
	sm.ins = make(map[int64]*index)
	for _, n := range s.Nodes {
		if n == sm.id {
			sm.addIns(n, sm.raftLog.lastIndex(), sm.raftLog.lastIndex()+1)
		} else {
			sm.addIns(n, 0, sm.raftLog.lastIndex()+1)
		}
	}
	sm.pendingConf = false
	return true
}

func (sm *raft) needSnapshot(i int64) bool {
	if i < sm.raftLog.offset {
		if sm.raftLog.snapshot.IsEmpty() {
			panic("need non-empty snapshot")
		}
		return true
	}
	return false
}

func (sm *raft) nodes() []int64 {
	nodes := make([]int64, 0, len(sm.ins))
	for k := range sm.ins {
		nodes = append(nodes, k)
	}
	return nodes
}

func (sm *raft) setTerm(term int64) {
	sm.Term = term
	sm.saveState()
}

func (sm *raft) setVote(vote int64) {
	sm.Vote = vote
	sm.saveState()
}

func (sm *raft) addIns(id, match, next int64) {
	sm.ins[id] = &index{next: next, match: match}
	sm.saveState()
}

func (sm *raft) deleteIns(id int64) {
	delete(sm.ins, id)
	sm.saveState()
}

// saveState saves the state to sm.unstableState
// When there is a term change, vote change or configuration change, raft
// must call saveState.
func (sm *raft) saveState() {
	sm.setState(sm.Vote, sm.Term, sm.raftLog.committed)
}

func (sm *raft) clearState() {
	sm.setState(0, 0, 0)
}

func (sm *raft) setState(vote, term, commit int64) {
	sm.unstableState.Vote = vote
	sm.unstableState.Term = term
	sm.unstableState.Commit = commit
}

func (sm *raft) loadEnts(ents []Entry) {
	if !sm.raftLog.isEmpty() {
		panic("cannot load entries when log is not empty")
	}
	sm.raftLog.append(0, ents...)
	sm.raftLog.unstable = sm.raftLog.lastIndex() + 1
}

func (sm *raft) loadState(state State) {
	sm.raftLog.committed = state.Commit
	sm.setTerm(state.Term)
	sm.setVote(state.Vote)
}

func (s *State) IsEmpty() bool {
	return s.Term == 0
}
