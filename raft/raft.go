package raft

import (
	"errors"
	"sort"
)

const none = -1

type messageType int

const (
	msgHup messageType = iota
	msgBeat
	msgProp
	msgApp
	msgAppResp
	msgVote
	msgVoteResp
)

var mtmap = [...]string{
	msgHup:      "msgHup",
	msgBeat:     "msgBeat",
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
	id int

	// the term we are participating in at any time
	term int

	// who we voted for in term
	vote int

	// the log
	log *log

	ins map[int]*index

	state stateType

	votes map[int]bool

	msgs []Message

	// the leader id
	lead int

	// pending reconfiguration
	pendingConf bool
}

func newStateMachine(id int, peers []int) *stateMachine {
	sm := &stateMachine{id: id, log: newLog(), ins: make(map[int]*index)}
	for p := range peers {
		sm.ins[p] = &index{}
	}
	sm.reset()
	return sm
}

func (sm *stateMachine) canStep(m Message) bool {
	if m.Type == msgProp {
		return sm.lead != none
	}
	return true
}

func (sm *stateMachine) poll(id int, v bool) (granted int) {
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
	m.From = sm.id
	m.Term = sm.term
	sm.msgs = append(sm.msgs, m)
}

// sendAppend sends RRPC, with entries to the given peer.
func (sm *stateMachine) sendAppend(to int) {
	in := sm.ins[to]
	m := Message{}
	m.Type = msgApp
	m.To = to
	m.Index = in.next - 1
	m.LogTerm = sm.log.term(in.next - 1)
	m.Entries = sm.log.entries(in.next)
	m.Commit = sm.log.committed
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

func (sm *stateMachine) maybeCommit() bool {
	// TODO(bmizerany): optimize.. Currently naive
	mis := make([]int, len(sm.ins))
	for i := range mis {
		mis[i] = sm.ins[i].match
	}
	sort.Sort(sort.Reverse(sort.IntSlice(mis)))
	mci := mis[sm.q()-1]

	return sm.log.maybeCommit(mci, sm.term)
}

// nextEnts returns the appliable entries and updates the applied index
func (sm *stateMachine) nextEnts() (ents []Entry) {
	return sm.log.nextEnts()
}

func (sm *stateMachine) reset() {
	sm.lead = none
	sm.vote = none
	sm.votes = make(map[int]bool)
	for i := range sm.ins {
		sm.ins[i] = &index{next: sm.log.lastIndex() + 1}
		if i == sm.id {
			sm.ins[i].match = sm.log.lastIndex()
		}
	}
}

func (sm *stateMachine) q() int {
	return len(sm.ins)/2 + 1
}

func (sm *stateMachine) becomeFollower(term, lead int) {
	sm.reset()
	sm.term = term
	sm.lead = lead
	sm.state = stateFollower
	sm.pendingConf = false
}

func (sm *stateMachine) becomeCandidate() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateLeader {
		panic("invalid transition [leader -> candidate]")
	}
	sm.reset()
	sm.term++
	sm.vote = sm.id
	sm.state = stateCandidate
}

func (sm *stateMachine) becomeLeader() {
	// TODO(xiangli) remove the panic when the raft implementation is stable
	if sm.state == stateFollower {
		panic("invalid transition [follower -> leader]")
	}
	sm.reset()
	sm.lead = sm.id
	sm.state = stateLeader

	for _, e := range sm.log.ents[sm.log.committed:] {
		if e.Type == configAdd || e.Type == configRemove {
			sm.pendingConf = true
		}
	}
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
		if sm.q() == sm.poll(sm.id, true) {
			sm.becomeLeader()
			return
		}
		for i := range sm.ins {
			if i == sm.id {
				continue
			}
			lasti := sm.log.lastIndex()
			sm.send(Message{To: i, Type: msgVote, Index: lasti, LogTerm: sm.log.term(lasti)})
		}
		return
	case msgBeat:
		if sm.state != stateLeader {
			return
		}
		sm.bcastAppend()
		return
	case msgProp:
		if len(m.Entries) != 1 {
			panic("unexpected length(entries) of a msgProp")
		}

		switch sm.lead {
		case sm.id:
			e := m.Entries[0]
			if e.Type == configAdd || e.Type == configRemove {
				if sm.pendingConf {
					// todo: deny
					return
				}
				sm.pendingConf = true
			}
			e.Term = sm.term

			sm.log.append(sm.log.lastIndex(), e)
			sm.ins[sm.id].update(sm.log.lastIndex())
			sm.maybeCommit()
			sm.bcastAppend()
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
		if sm.log.maybeAppend(m.Index, m.LogTerm, m.Commit, m.Entries...) {
			sm.send(Message{To: m.From, Type: msgAppResp, Index: sm.log.lastIndex()})
		} else {
			sm.send(Message{To: m.From, Type: msgAppResp, Index: -1})
		}
	}

	switch sm.state {
	case stateLeader:
		switch m.Type {
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
	case stateCandidate:
		switch m.Type {
		case msgApp:
			sm.becomeFollower(sm.term, m.From)
			handleAppendEntries()
		case msgVote:
			sm.send(Message{To: m.From, Type: msgVoteResp, Index: -1})
		case msgVoteResp:
			gr := sm.poll(m.From, m.Index >= 0)
			switch sm.q() {
			case gr:
				sm.becomeLeader()
				sm.bcastAppend()
			case len(sm.votes) - gr:
				sm.becomeFollower(sm.term, none)
			}
		}
	case stateFollower:
		switch m.Type {
		case msgApp:
			handleAppendEntries()
		case msgVote:
			if (sm.vote == none || sm.vote == m.From) && sm.log.isUpToDate(m.Index, m.LogTerm) {
				sm.vote = m.From
				sm.send(Message{To: m.From, Type: msgVoteResp, Index: sm.log.lastIndex()})
			} else {
				sm.send(Message{To: m.From, Type: msgVoteResp, Index: -1})
			}
		}
	}
}

func (sm *stateMachine) Add(id int) {
	sm.ins[id] = &index{next: sm.log.lastIndex() + 1}
	sm.pendingConf = false
}

func (sm *stateMachine) Remove(id int) {
	delete(sm.ins, id)
	sm.pendingConf = false
}
