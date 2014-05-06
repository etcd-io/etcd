package raft

import (
	"fmt"
	"reflect"
	"testing"
)

func TestLeaderElection(t *testing.T) {
	tests := []struct {
		network
		state stateType
	}{
		{newNetwork(nil, nil, nil), stateLeader},
		{newNetwork(nil, nil, nopStepper), stateLeader},
		{newNetwork(nil, nopStepper, nopStepper), stateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil), stateCandidate},
		{newNetwork(nil, nopStepper, nopStepper, nil, nil), stateLeader},
	}

	for i, tt := range tests {
		tt.step(Message{To: 0, Type: msgHup})
		sm := tt.network[0].(*stateMachine)
		if sm.state != tt.state {
			t.Errorf("#%d: state = %s, want %s", i, sm.state, tt.state)
		}
		if g := sm.term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestProposal(t *testing.T) {
	tests := []struct {
		network
		success bool
	}{
		{newNetwork(nil, nil, nil), true},
		{newNetwork(nil, nil, nopStepper), true},
		{newNetwork(nil, nopStepper, nopStepper), false},
		{newNetwork(nil, nopStepper, nopStepper, nil), false},
		{newNetwork(nil, nopStepper, nopStepper, nil, nil), true},
	}

	for i, tt := range tests {
		step := stepperFunc(func(m Message) {
			defer func() {
				if !tt.success {
					// not expected success implies there
					// will be no known leader which will
					// cause step to panic - swallow it.
					e := recover()
					if e != nil {
						t.Logf("#%d: err: %s", i, e)
					}
				}
			}()
			t.Logf("#%d: m = %+v", i, m)
			tt.step(m)
		})

		var data = []byte("somedata")

		// promote 0 the leader
		step(Message{To: 0, Type: msgHup})
		step(Message{To: 0, Type: msgProp, Data: data})

		w := []Entry{{}}
		if tt.success {
			w = append(w, Entry{Term: 1, Data: data})
		}
		ls := append([][]Entry{w}, tt.logs()...)

		if g := diffLogs(ls); g != nil {
			for _, diff := range g {
				t.Errorf("#%d: bag log:\n%s", i, diff)
			}
		}
		sm := tt.network[0].(*stateMachine)
		if g := sm.term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

func TestProposalByProxy(t *testing.T) {
	tests := []struct {
		network
		success bool
	}{
		{newNetwork(nil, nil, nil), true},
		{newNetwork(nil, nil, nopStepper), true},
	}

	for i, tt := range tests {
		step := stepperFunc(func(m Message) {
			t.Logf("#%d: m = %+v", i, m)
			tt.step(m)
		})

		// promote 0 the leader
		step(Message{To: 0, Type: msgHup})

		// propose via follower
		step(Message{To: 1, Type: msgProp, Data: []byte("somedata")})

		if g := diffLogs(tt.logs()); g != nil {
			for _, diff := range g {
				t.Errorf("#%d: bag log:\n%s", i, diff)
			}
		}
		sm := tt.network[0].(*stateMachine)
		if g := sm.term; g != 1 {
			t.Errorf("#%d: term = %d, want %d", i, g, 1)
		}
	}
}

type network []stepper

// newNetwork initializes a network from nodes. A nil node will be replaced
// with a new *stateMachine. A *stateMachine will get its k, addr, and next
// fields set.
func newNetwork(nodes ...stepper) network {
	nt := network(nodes)
	for i, n := range nodes {
		switch v := n.(type) {
		case nil:
			nt[i] = newStateMachine(len(nodes), i, &nt)
		case *stateMachine:
			v.k = len(nodes)
			v.addr = i
			v.next = &nt
		}
	}
	return nt
}

func (nt network) step(m Message) {
	nt[m.To].step(m)
}

func (nt network) logs() [][]Entry {
	ls := make([][]Entry, len(nt))
	for i, node := range nt {
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

var naEntry = &Entry{}
var nologEntry = &Entry{}

func (d diff) String() string {
	s := fmt.Sprintf("[%d] ", d.i)
	for i, e := range d.ents {
		switch e {
		case nologEntry:
			s += fmt.Sprintf("<NL>")
		case naEntry:
			s += fmt.Sprintf("<N/A>")
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

func diffLogs(logs [][]Entry) []diff {
	var (
		d   []diff
		max int
	)
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
				e[j] = nologEntry
				continue
			}
			if len(log) <= i {
				e[j] = naEntry
				found = true
				continue
			}
			e[j] = &log[i]
			if j > 0 {
				switch prev := e[j-1]; {
				case prev == nologEntry:
				case prev == naEntry:
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

func (f stepperFunc) step(m Message) { f(m) }

var nopStepper = stepperFunc(func(Message) {})
