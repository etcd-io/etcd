package raft

import (
	"fmt"
	"reflect"
	"testing"
)

var defaultLog = []Entry{{}}

func TestLeaderElection(t *testing.T) {
	tests := []struct {
		*network
		state stateType
	}{
		{newNetwork(nil, nil, nil), stateLeader},
		{newNetwork(nil, nil, nopStepper), stateLeader},
		{newNetwork(nil, nopStepper, nopStepper), stateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil), stateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil, nil), stateLeader},

		// three nodes are have logs further along than 0
		{
			newNetwork(
				nil,
				&stateMachine{log: []Entry{{}, {Term: 1}}},
				&stateMachine{log: []Entry{{}, {Term: 2}}},
				&stateMachine{log: []Entry{{}, {Term: 1}, {Term: 3}}},
				nil,
			),
			stateFollower,
		},

		// logs converge
		{
			newNetwork(
				&stateMachine{log: []Entry{{}, {Term: 1}}},
				nil,
				&stateMachine{log: []Entry{{}, {Term: 2}}},
				&stateMachine{log: []Entry{{}, {Term: 1}}},
				nil,
			),
			stateLeader,
		},
	}

	for i, tt := range tests {
		tt.Step(Message{To: 0, Type: msgHup})
		sm := tt.network.ss[0].(*stateMachine)
		if sm.state != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, sm.state, tt.state)
		}
		if g := sm.term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestDualingCandidates(t *testing.T) {
	a := &stateMachine{log: defaultLog}
	c := &stateMachine{log: defaultLog}

	tt := newNetwork(a, nil, c)

	heal := false
	next := stepperFunc(func(m Message) {
		if heal {
			tt.Step(m)
		}
	})
	a.next = next
	c.next = next

	tt.tee = stepperFunc(func(m Message) {
		t.Logf("m = %+v", m)
	})
	tt.Step(Message{To: 0, Type: msgHup})
	tt.Step(Message{To: 2, Type: msgHup})

	t.Log("healing")
	heal = true
	tt.Step(Message{To: 2, Type: msgHup})

	tests := []struct {
		sm    *stateMachine
		state stateType
		term  int
	}{
		{a, stateFollower, 2},
		{c, stateLeader, 2},
	}

	for i, tt := range tests {
		if g := tt.sm.state; g != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, g, tt.state)
		}
		if g := tt.sm.term; g != tt.term {
			t.Errorf("#%d: term = %d, want %d", i, g, tt.term)
		}
	}
	if g := diffLogs(defaultLog, tt.logs()); g != nil {
		for _, diff := range g {
			t.Errorf("bag log:\n%s", diff)
		}
	}
}

func TestCandidateConcede(t *testing.T) {
	a := &stateMachine{log: defaultLog}

	tt := newNetwork(a, nil, nil)
	tt.tee = stepperFunc(func(m Message) {
		t.Logf("m = %+v", m)
	})

	a.next = nopStepper

	tt.Step(Message{To: 0, Type: msgHup})
	tt.Step(Message{To: 2, Type: msgHup})

	// heal the partition
	a.next = tt

	data := []byte("force follower")
	// send a proposal to 2 to flush out a msgApp to 0
	tt.Step(Message{To: 2, Type: msgProp, Data: data})

	if g := a.state; g != stateFollower {
		t.Errorf("state = %s, want %s", g, stateFollower)
	}
	if g := a.term; g != 1 {
		t.Errorf("term = %d, want %d", g, 1)
	}
	wantLog := []Entry{{}, {Term: 1, Data: data}}
	if g := diffLogs(wantLog, tt.logs()); g != nil {
		for _, diff := range g {
			t.Errorf("bag log:\n%s", diff)
		}
	}
}

func TestOldMessages(t *testing.T) {
	tt := newNetwork(nil, nil, nil)
	// make 0 leader @ term 3
	tt.Step(Message{To: 0, Type: msgHup})
	tt.Step(Message{To: 0, Type: msgHup})
	tt.Step(Message{To: 0, Type: msgHup})
	// pretend we're an old leader trying to make progress
	tt.Step(Message{To: 0, Type: msgApp, Term: 1, Entries: []Entry{{Term: 1}}})
	if g := diffLogs(defaultLog, tt.logs()); g != nil {
		for _, diff := range g {
			t.Errorf("bag log:\n%s", diff)
		}
	}
}

// TestOldMessagesReply - optimization - reply with new term.

func TestProposal(t *testing.T) {
	tests := []struct {
		*network
		success bool
	}{
		{newNetwork(nil, nil, nil), true},
		{newNetwork(nil, nil, nopStepper), true},
		{newNetwork(nil, nopStepper, nopStepper), false},
		{newNetwork(nil, nopStepper, nopStepper, nil), false},
		{newNetwork(nil, nopStepper, nopStepper, nil, nil), true},
	}

	for i, tt := range tests {
		tt.tee = stepperFunc(func(m Message) {
			t.Logf("#%d: m = %+v", i, m)
		})

		step := stepperFunc(func(m Message) {
			defer func() {
				// only recover is we expect it to panic so
				// panics we don't expect go up.
				if !tt.success {
					e := recover()
					if e != nil {
						t.Logf("#%d: err: %s", i, e)
					}
				}
			}()
			tt.Step(m)
		})

		data := []byte("somedata")

		// promote 0 the leader
		step(Message{To: 0, Type: msgHup})
		step(Message{To: 0, Type: msgProp, Data: data})

		var wantLog []Entry
		if tt.success {
			wantLog = []Entry{{}, {Term: 1, Data: data}}
		} else {
			wantLog = defaultLog
		}
		if g := diffLogs(wantLog, tt.logs()); g != nil {
			for _, diff := range g {
				t.Errorf("#%d: diff:%s", i, diff)
			}
		}
		sm := tt.network.ss[0].(*stateMachine)
		if g := sm.term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestProposalByProxy(t *testing.T) {
	data := []byte("somedata")
	tests := []*network{
		newNetwork(nil, nil, nil),
		newNetwork(nil, nil, nopStepper),
	}

	for i, tt := range tests {
		tt.tee = stepperFunc(func(m Message) {
			t.Logf("#%d: m = %+v", i, m)
		})

		// promote 0 the leader
		tt.Step(Message{To: 0, Type: msgHup})

		// propose via follower
		tt.Step(Message{To: 1, Type: msgProp, Data: []byte("somedata")})

		wantLog := []Entry{{}, {Term: 1, Data: data}}
		if g := diffLogs(wantLog, tt.logs()); g != nil {
			for _, diff := range g {
				t.Errorf("#%d: bad entry: %s", i, diff)
			}
		}
		sm := tt.ss[0].(*stateMachine)
		if g := sm.term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestVote(t *testing.T) {
	tests := []struct {
		i, term int
		w       int
	}{
		{0, 0, -1},
		{0, 1, -1},
		{0, 2, -1},
		{0, 3, 2},

		{1, 0, -1},
		{1, 1, -1},
		{1, 2, -1},
		{1, 3, 2},

		{2, 0, -1},
		{2, 1, -1},
		{2, 2, 2},
		{2, 3, 2},

		{3, 0, -1},
		{3, 1, -1},
		{3, 2, 2},
		{3, 3, 2},
	}

	for i, tt := range tests {
		called := false
		sm := &stateMachine{log: []Entry{{}, {Term: 2}, {Term: 2}}}
		sm.next = stepperFunc(func(m Message) {
			called = true
			if m.Index != tt.w {
				t.Errorf("#%d, m.Index = %d, want %d", i, m.Index, tt.w)
			}
		})
		sm.Step(Message{Type: msgVote, Index: tt.i, LogTerm: tt.term})
		if !called {
			t.Fatal("#%d: not called", i)
		}
	}
}

func TestLogDiff(t *testing.T) {
	a := []Entry{{}, {Term: 1}, {Term: 2}}
	b := []Entry{{}, {Term: 1}, {Term: 2}}
	c := []Entry{{}, {Term: 2}}
	d := []Entry(nil)

	w := []diff{
		diff{1, []*Entry{{Term: 1}, {Term: 1}, {Term: 2}, nilLogEntry}},
		diff{2, []*Entry{{Term: 2}, {Term: 2}, noEntry, nilLogEntry}},
	}

	if g := diffLogs(a, [][]Entry{b, c, d}); !reflect.DeepEqual(w, g) {
		t.Errorf("g = %s", g)
		t.Errorf("want %s", w)
	}
}

type network struct {
	tee Interface
	ss  []Interface
}

// newNetwork initializes a network from nodes. A nil node will be replaced
// with a new *stateMachine. A *stateMachine will get its k, addr, and next
// fields set.
func newNetwork(nodes ...Interface) *network {
	nt := &network{ss: nodes}
	for i, n := range nodes {
		switch v := n.(type) {
		case nil:
			nt.ss[i] = newStateMachine(len(nodes), i, nt)
		case *stateMachine:
			v.k = len(nodes)
			v.addr = i
			if v.next == nil {
				v.next = nt
			}
		default:
			nt.ss[i] = v
		}
	}
	return nt
}

func (nt network) Step(m Message) {
	if nt.tee != nil {
		nt.tee.Step(m)
	}
	nt.ss[m.To].Step(m)
}

// logs returns all logs in nt prepended with want. If a node is not a
// *stateMachine, its log will be nil.
func (nt network) logs() [][]Entry {
	ls := make([][]Entry, len(nt.ss))
	for i, node := range nt.ss {
		if sm, ok := node.(*stateMachine); ok {
			ls[i] = sm.log
		}
	}
	return ls
}

type diff struct {
	i    int
	ents []*Entry // pointers so they can be nil for N/A
}

var noEntry = &Entry{}
var nilLogEntry = &Entry{}

func (d diff) String() string {
	s := fmt.Sprintf("[%d] ", d.i)
	for i, e := range d.ents {
		switch e {
		case nilLogEntry:
			s += fmt.Sprintf("o")
		case noEntry:
			s += fmt.Sprintf("-")
		case nil:
			s += fmt.Sprintf("<nil>")
		default:
			s += fmt.Sprintf("<%d:%q>", e.Term, string(e.Data))
		}
		if i != len(d.ents)-1 {
			s += "\t\t"
		}
	}
	return s
}

func diffLogs(base []Entry, logs [][]Entry) []diff {
	var (
		d   []diff
		max int
	)
	logs = append([][]Entry{base}, logs...)
	for _, log := range logs {
		if l := len(log); l > max {
			max = l
		}
	}
	ediff := func(i int) (result []*Entry) {
		e := make([]*Entry, len(logs))
		found := false
		for j, log := range logs {
			if log == nil {
				e[j] = nilLogEntry
				continue
			}
			if len(log) <= i {
				e[j] = noEntry
				found = true
				continue
			}
			e[j] = &log[i]
			if j > 0 {
				switch prev := e[j-1]; {
				case prev == nilLogEntry:
				case prev == noEntry:
				case !reflect.DeepEqual(prev, e[j]):
					found = true
				}
			}
		}
		if found {
			return e
		}
		return nil
	}
	for i := 0; i < max; i++ {
		if e := ediff(i); e != nil {
			d = append(d, diff{i, e})
		}
	}
	return d
}

type stepperFunc func(Message)

func (f stepperFunc) Step(m Message) { f(m) }

var nopStepper = stepperFunc(func(Message) {})
