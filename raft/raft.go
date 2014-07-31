package raft

import (
	"errors"
	"fmt"
	"log"
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

type State struct {
	Term   int64
	Vote   int64
	Commit int64
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

type stateMachine struct {
	clusterId int64
	id        int64

	// the term we are participating in at any time
	term  atomicInt
	index atomicInt

	// who we voted for in term
	vote int64

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
}

func newStateMachine(id int64, peers []int64) *stateMachine {
	if id == none {
		panic("cannot use none id")
	}
	sm := &stateMachine{id: id, clusterId: none, lead: none, raftLog: newLog(), ins: make(map[int64]*index)}
	for _, p := range peers {
		sm.ins[p] = &index{}
	}
	sm.reset(0)
	return sm
}

func (sm *stateMachine) String() string {
	s := fmt.Sprintf(`state=%v term=%d`, sm.state, sm.term)
	switch sm.state {
	case stateFollower:
		s += fmt.Sprintf(" vote=%v lead=%v", sm.vote, sm.lead)
	case stateCandidate:
		s += fmt.Sprintf(` votes="%v"`, sm.votes)
	case stateLeader:
		s += fmt.Sprintf(` ins="%v"`, sm.ins)
	}
	return s
}

func (sm *stateMachine) poll(id int64, v bool) (granted int) {
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
func (sm *stateMachine) send(m Message) {
	m.ClusterId = sm.clusterId
	m.From = sm.id
	m.Term = sm.term.Get()
	log.Printf("raft.send msg %v\n", m)
	sm.msgs = append(sm.msgs, m)
}

// sendAppend sends RRPC, with entries to the given peer.
func (sm *stateMachine) sendAppend(to int64) {
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
func (sm *stateMachine) sendHeartbeat(to int64) {
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
func (sm *stateMachine) bcastAppend() {
	for i := range sm.ins {
		if i == sm.id {
			continue
		}
		sm.sendAppend(i)
	}
}

// bcastHeartbeat sends RRPC, without entries to all the peers.
func (sm *stateMachine) bcastHeartbeat() {
	for i := range sm.ins {
		if i == sm.id {
			continue
		}
		sm.sendHeartbeat(i)
	}
}

func (sm *stateMachine) maybeCommit() bool {
	// TODO(bmizerany): optimize.. Currently naive
	mis := make(int64Slice, 0, len(sm.ins))
	for i := range sm.ins {
		mis = append(mis, sm.ins[i].match)
	}
	sort.Sort(sort.Reverse(mis))
	mci := mis[sm.q()-1]

	return sm.raftLog.maybeCommit(mci, sm.term.Get())
}

// nextEnts returns the appliable entries and updates the applied index
func (sm *stateMachine) nextEnts() (ents []Entry) {
	return sm.raftLog.nextEnts()
}

func (sm *stateMachine) reset(term int64) {
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

func (sm *stateMachine) q() int {
	return len(sm.ins)/2 + 1
}

func (sm *stateMachine) appendEntry(e Entry) {
	e.Term = sm.term.Get()
	e.Index = sm.raftLog.lastIndex() + 1
	sm.index.Set(sm.raftLog.append(sm.raftLog.lastIndex(), e))
	sm.ins[sm.id].update(sm.raftLog.lastIndex())
	sm.maybeCommit()
}

// promotable indicates whether state machine could be promoted.
// New machine has to wait for the first log entry to be committed, or it will
// always start as a one-node cluster.
func (sm *stateMachine) promotable() bool {
	return sm.raftLog.committed != 0
}

func (sm *stateMachine) becomeFollower(term int64, lead int64) {
	sm.reset(term)
	sm.lead.Set(lead)
	sm.state = stateFollower
	sm.pendingConf = false
}

func (sm *stateMachine) becomeCandidate() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	sm.reset(sm.term.Get() + 1)
	sm.setVote(sm.id)
	sm.state = stateCandidate
}

func (sm *stateMachine) becomeLeader() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateFollower {
		panic("invalid transition [follower -> leader]")
	}
	sm.reset(sm.term.Get())
	sm.lead.Set(sm.id)
	sm.state = stateLeader

	for _, e := range sm.raftLog.entries(sm.raftLog.committed + 1) {
		if e.isConfig() {
			sm.pendingConf = true
		}
	}

	sm.appendEntry(Entry{Type: Normal, Data: nil})
}

func (sm *stateMachine) Msgs() []Message {
	msgs := sm.msgs
	sm.msgs = make([]Message, 0)

	return msgs
}

func (sm *stateMachine) Step(m Message) (ok bool) {
	log.Printf("raft.step beforeState %v\n", sm)
	log.Printf("raft.step beforeLog %v\n", sm.raftLog)
	defer log.Printf("raft.step afterLog %v\n", sm.raftLog)
	defer log.Printf("raft.step afterState %v\n", sm)
	log.Printf("raft.step msg %v\n", m)
	if m.Type == msgHup {
		sm.becomeCandidate()
		if sm.q() == sm.poll(sm.id, true) {
			sm.becomeLeader()
			return true
		}
		for i := range sm.ins {
			if i == sm.id {
				continue
			}
			lasti := sm.raftLog.lastIndex()
			sm.send(Message{To: i, Type: msgVote, Index: lasti, LogTerm: sm.raftLog.term(lasti)})
		}
		return true
	}

	switch {
	case m.Term == 0:
		// local message
	case m.Term > sm.term.Get():
		lead := m.From
		if m.Type == msgVote {
			lead = none
		}
		sm.becomeFollower(m.Term, lead)
	case m.Term < sm.term.Get():
		// ignore
		return true
	}

	return stepmap[sm.state](sm, m)
}

func (sm *stateMachine) handleAppendEntries(m Message) {
	if sm.raftLog.maybeAppend(m.Index, m.LogTerm, m.Commit, m.Entries...) {
		sm.index.Set(sm.raftLog.lastIndex())
		sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.raftLog.lastIndex()})
	} else {
		sm.send(Message{To: m.From, Type: msgAppResp, Index: -1})
	}
}

func (sm *stateMachine) handleSnapshot(m Message) {
	if sm.restore(m.Snapshot) {
		sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.raftLog.lastIndex()})
	} else {
		sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.raftLog.committed})
	}
}

func (sm *stateMachine) addNode(id int64) {
	sm.addIns(id, 0, sm.raftLog.lastIndex()+1)
	sm.pendingConf = false
}

func (sm *stateMachine) removeNode(id int64) {
	sm.deleteIns(id)
	sm.pendingConf = false
}

type stepFunc func(sm *stateMachine, m Message) bool

func stepLeader(sm *stateMachine, m Message) bool {
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

func stepCandidate(sm *stateMachine, m Message) bool {
	switch m.Type {
	case msgProp:
		return false
	case msgApp:
		sm.becomeFollower(sm.term.Get(), m.From)
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
			sm.becomeFollower(sm.term.Get(), none)
		}
	}
	return true
}

func stepFollower(sm *stateMachine, m Message) bool {
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
		if (sm.vote == none || sm.vote == m.From) && sm.raftLog.isUpToDate(m.Index, m.LogTerm) {
			sm.setVote(m.From)
			sm.send(Message{To: m.From, Type: msgVoteResp, Index: sm.raftLog.lastIndex()})
		} else {
			sm.send(Message{To: m.From, Type: msgVoteResp, Index: -1})
		}
	}
	return true
}

func (sm *stateMachine) compact(d []byte) {
	sm.raftLog.snap(d, sm.raftLog.applied, sm.raftLog.term(sm.raftLog.applied), sm.nodes())
	sm.raftLog.compact(sm.raftLog.applied)
}

// restore recovers the statemachine from a snapshot. It restores the log and the
// configuration of statemachine.
func (sm *stateMachine) restore(s Snapshot) bool {
	if s.Index <= sm.raftLog.committed {
		return false
	}

	sm.raftLog.restore(s)
	sm.index.Set(sm.raftLog.lastIndex())
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

func (sm *stateMachine) needSnapshot(i int64) bool {
	if i < sm.raftLog.offset {
		if sm.raftLog.snapshot.IsEmpty() {
			panic("need non-empty snapshot")
		}
		return true
	}
	return false
}

func (sm *stateMachine) nodes() []int64 {
	nodes := make([]int64, 0, len(sm.ins))
	for k := range sm.ins {
		nodes = append(nodes, k)
	}
	return nodes
}

func (sm *stateMachine) setTerm(term int64) {
	sm.term.Set(term)
	sm.saveState()
}

func (sm *stateMachine) setVote(vote int64) {
	sm.vote = vote
	sm.saveState()
}

func (sm *stateMachine) addIns(id, match, next int64) {
	sm.ins[id] = &index{next: next, match: match}
	sm.saveState()
}

func (sm *stateMachine) deleteIns(id int64) {
	delete(sm.ins, id)
	sm.saveState()
}

// saveState saves the state to sm.unstableState
// When there is a term change, vote change or configuration change, raft
// must call saveState.
func (sm *stateMachine) saveState() {
	sm.setState(sm.vote, sm.term.Get(), sm.raftLog.committed)
}

func (sm *stateMachine) clearState() {
	sm.setState(0, 0, 0)
}

func (sm *stateMachine) setState(vote, term, commit int64) {
	sm.unstableState.Vote = vote
	sm.unstableState.Term = term
	sm.unstableState.Commit = commit
}

func (sm *stateMachine) loadEnts(ents []Entry) {
	if !sm.raftLog.isEmpty() {
		panic("cannot load entries when log is not empty")
	}
	sm.raftLog.append(0, ents...)
	sm.raftLog.unstable = sm.raftLog.lastIndex() + 1
}

func (sm *stateMachine) loadState(state State) {
	sm.raftLog.committed = state.Commit
	sm.setTerm(state.Term)
	sm.setVote(state.Vote)
}
