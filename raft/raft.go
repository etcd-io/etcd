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

	commit  int
	applied int

	votes map[int]bool

	msgs []Message

	// the leader addr
	lead int
}

func newStateMachine(k, addr int) *stateMachine {
	log := make([]Entry, 1, 1024)
	sm := &stateMachine{k: k, addr: addr, log: log}
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

// send persists state to stable storage and then sends to its mailbox
func (sm *stateMachine) send(m Message) {
	m.From = sm.addr
	m.Term = sm.term
	sm.msgs = append(sm.msgs, m)
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
		m.Commit = sm.commit
		sm.send(m)
	}
}

func (sm *stateMachine) maybeCommit() bool {
	// TODO(bmizerany): optimize.. Currently naive
	mis := make([]int, len(sm.ins))
	for i := range mis {
		mis[i] = sm.ins[i].match
	}
	sort.Sort(sort.Reverse(sort.IntSlice(mis)))
	mci := mis[sm.q()-1]

	if mci > sm.commit && sm.log[mci].Term == sm.term {
		sm.commit = mci
		return true
	}

	return false
}

// nextEnts returns the appliable entries and updates the applied index
func (sm *stateMachine) nextEnts() (ents []Entry) {
	if sm.commit > sm.applied {
		ents = sm.log[sm.applied+1 : sm.commit+1]
		sm.applied = sm.commit
	}
	return ents
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

func (sm *stateMachine) becomeCandidate() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	sm.reset()
	sm.term++
	sm.vote = sm.addr
	sm.state = stateCandidate
	sm.poll(sm.addr, true)
}

func (sm *stateMachine) becomeLeader() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateFollower {
		panic("invalid transition [follower -> leader]")
	}
	sm.reset()
	sm.lead = sm.addr
	sm.state = stateLeader
}

func (sm *stateMachine) Msgs() []Message {
	msgs := sm.msgs
	sm.msgs = make([]Message, 0)

	return msgs
}

func (sm *stateMachine) Step(m Message) {
	switch m.Type {
	case msgHup:
		sm.becomeCandidate()
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
			sm.commit = m.Commit
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
				if sm.maybeCommit() {
					sm.sendAppend()
				}
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
				sm.becomeLeader()
				sm.sendAppend()
			case len(sm.votes) - gr:
				sm.becomeFollower(sm.term, none)
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
