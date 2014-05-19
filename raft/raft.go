package raft

import (
	"errors"
	"sort"
)

const none = -1

type messageType int

const (
	msgHup messageType = iota
	msgProp
	msgApp
	msgAppResp
	msgVote
	msgVoteResp
)

var mtmap = [...]string{
	msgHup:      "msgHup",
	msgProp:     "msgProp",
	msgApp:      "msgApp",
	msgAppResp:  "msgAppResp",
	msgVote:     "msgVote",
	msgVoteResp: "msgVoteResp",
}

func (mt messageType) String() string {
	return mtmap[int(mt)]
}

var errNoLeader = errors.New("no leader")

const (
	stateFollower stateType = iota
	stateCandidate
	stateLeader
)

type stateType int

var stmap = [...]string{
	stateFollower:  "stateFollower",
	stateCandidate: "stateCandidate",
	stateLeader:    "stateLeader",
}

func (st stateType) String() string {
	return stmap[int(st)]
}

type Entry struct {
	Term int
	Data []byte
}

type Message struct {
	Type     messageType
	To       int
	From     int
	Term     int
	LogTerm  int
	Index    int
	PrevTerm int
	Entries  []Entry
	Commit   int
	Data     []byte
}

type index struct {
	match, next int
}

func (in *index) update(n int) {
	in.match = n
	in.next = n + 1
}

func (in *index) decr() {
	if in.next--; in.next < 1 {
		in.next = 1
	}
}

type stateMachine struct {
	// k is the number of peers
	k int

	// addr is an integer representation of our address amoungst our peers. It is 0 <= addr < k.
	addr int

	// the term we are participating in at any time
	term int

	// who we voted for in term
	vote int

	// the log
	log []Entry

	ins []*index

	state stateType

	commit int

	votes map[int]bool

	next Interface

	// the leader addr
	lead int
}

func newStateMachine(k, addr int, next Interface) *stateMachine {
	log := make([]Entry, 1, 1024)
	sm := &stateMachine{k: k, addr: addr, next: next, log: log}
	sm.reset()
	return sm
}

func (sm *stateMachine) canStep(m Message) bool {
	if m.Type == msgProp {
		return sm.lead != none
	}
	return true
}

func (sm *stateMachine) poll(addr int, v bool) (granted int) {
	if _, ok := sm.votes[addr]; !ok {
		sm.votes[addr] = v
	}
	for _, vv := range sm.votes {
		if vv {
			granted++
		}
	}
	return granted
}

func (sm *stateMachine) append(after int, ents ...Entry) int {
	sm.log = append(sm.log[:after+1], ents...)
	return len(sm.log) - 1
}

func (sm *stateMachine) isLogOk(i, term int) bool {
	if i > sm.li() {
		return false
	}
	return sm.log[i].Term == term
}

// send persists state to stable storage and then sends m over the network to m.To
func (sm *stateMachine) send(m Message) {
	m.From = sm.addr
	m.Term = sm.term
	sm.next.Step(m)
}

// sendAppend sends RRPC, with entries to all peers that are not up-to-date according to sm.mis.
func (sm *stateMachine) sendAppend() {
	for i := 0; i < sm.k; i++ {
		if i == sm.addr {
			continue
		}
		in := sm.ins[i]
		m := Message{}
		m.Type = msgApp
		m.To = i
		m.Index = in.next - 1
		m.LogTerm = sm.log[in.next-1].Term
		m.Entries = sm.log[in.next:]
		sm.send(m)
	}
}

func (sm *stateMachine) theN() int {
	// TODO(bmizerany): optimize.. Currently naive
	mis := make([]int, len(sm.ins))
	for i := range mis {
		mis[i] = sm.ins[i].match
	}
	sort.Ints(mis)
	for _, mi := range mis[sm.k/2+1:] {
		if sm.log[mi].Term == sm.term {
			return mi
		}
	}
	return -1
}

func (sm *stateMachine) maybeAdvanceCommit() int {
	ci := sm.theN()
	if ci > sm.commit {
		sm.commit = ci
	}
	return sm.commit
}

func (sm *stateMachine) reset() {
	sm.lead = none
	sm.vote = none
	sm.votes = make(map[int]bool)
	sm.ins = make([]*index, sm.k)
	for i := range sm.ins {
		sm.ins[i] = &index{next: len(sm.log)}
	}
}

func (sm *stateMachine) q() int {
	return sm.k/2 + 1
}

func (sm *stateMachine) voteWorthy(i, term int) bool {
	// LET logOk == \/ m.mlastLogTerm > LastTerm(log[i])
	//              \/ /\ m.mlastLogTerm = LastTerm(log[i])
	//                 /\ m.mlastLogIndex >= Len(log[i])
	e := sm.log[sm.li()]
	return term > e.Term || (term == e.Term && i >= sm.li())
}

func (sm *stateMachine) li() int {
	return len(sm.log) - 1
}

func (sm *stateMachine) becomeFollower(term, lead int) {
	sm.reset()
	sm.term = term
	sm.lead = lead
	sm.state = stateFollower
}

func (sm *stateMachine) Step(m Message) {
	switch m.Type {
	case msgHup:
		sm.term++
		sm.reset()
		sm.state = stateCandidate
		sm.vote = sm.addr
		sm.poll(sm.addr, true)
		for i := 0; i < sm.k; i++ {
			if i == sm.addr {
				continue
			}
			lasti := sm.li()
			sm.send(Message{To: i, Type: msgVote, Index: lasti, LogTerm: sm.log[lasti].Term})
		}
		return
	case msgProp:
		switch sm.lead {
		case sm.addr:
			sm.append(sm.li(), Entry{Term: sm.term, Data: m.Data})
			sm.sendAppend()
		case none:
			panic("msgProp given without leader")
		default:
			m.To = sm.lead
			sm.send(m)
		}
		return
	}

	switch {
	case m.Term > sm.term:
		sm.becomeFollower(m.Term, m.From)
	case m.Term < sm.term:
		// ignore
		return
	}

	handleAppendEntries := func() {
		if sm.isLogOk(m.Index, m.LogTerm) {
			sm.append(m.Index, m.Entries...)
			sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.li()})
		} else {
			sm.send(Message{To: m.From, Type: msgAppResp, Index: -1})
		}
	}

	switch sm.state {
	case stateLeader:
		switch m.Type {
		case msgAppResp:
			in := sm.ins[m.From]
			if m.Index < 0 {
				in.decr()
				sm.sendAppend()
			} else {
				in.update(m.Index)
			}
		}
	case stateCandidate:
		switch m.Type {
		case msgApp:
			sm.becomeFollower(sm.term, m.From)
			handleAppendEntries()
		case msgVoteResp:
			gr := sm.poll(m.From, m.Index >= 0)
			switch sm.q() {
			case gr:
				sm.state = stateLeader
				sm.lead = sm.addr
				sm.sendAppend()
			case len(sm.votes) - gr:
				sm.state = stateFollower
			}
		}
	case stateFollower:
		switch m.Type {
		case msgApp:
			handleAppendEntries()
		case msgVote:
			if sm.voteWorthy(m.Index, m.LogTerm) {
				sm.send(Message{To: m.From, Type: msgVoteResp, Index: sm.li()})
			} else {
				sm.send(Message{To: m.From, Type: msgVoteResp, Index: -1})
			}
		}
	}
}
