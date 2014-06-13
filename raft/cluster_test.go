package raft

import (
	"reflect"
	"testing"
)

// TestBuildCluster ensures cluster with various size could be built.
func TestBuildCluster(t *testing.T) {
	tests := []int{1, 3, 5, 7, 9, 13, 51}

	for i, tt := range tests {
		_, nodes := buildCluster(tt)

		base := ltoa(nodes[0].sm.log)
		for j, n := range nodes {
			// ensure same log
			l := ltoa(n.sm.log)
			if g := diffu(base, l); g != "" {
				t.Errorf("#%d.%d: log diff:\n%s", i, j, g)
			}

			// ensure same leader
			if n.sm.lead != 0 {
				t.Errorf("#%d.%d: lead = %d, want 0", i, j, n.sm.lead)
			}

			// ensure same peer map
			p := map[int]struct{}{}
			for k := range n.sm.ins {
				p[k] = struct{}{}
			}
			wp := map[int]struct{}{}
			for k := 0; k < tt; k++ {
				wp[k] = struct{}{}
			}
			if !reflect.DeepEqual(p, wp) {
				t.Errorf("#%d.%d: peers = %+v, want %+v", i, j, p, wp)
			}
		}
	}
}

// TestBasicCluster ensures all nodes can send proposal to the cluster.
// And all the proposals will get committed.
func TestBasicCluster(t *testing.T) {
	tests := []struct {
		size  int
		round int
	}{
		{1, 3},
		{3, 3},
		{5, 3},
		{7, 3},
		{13, 1},
	}

	for i, tt := range tests {
		nt, nodes := buildCluster(tt.size)

		for j := 0; j < tt.round; j++ {
			for _, n := range nodes {
				data := []byte{byte(n.Id())}
				nt.send(Message{Type: msgProp, To: n.Id(), Entries: []Entry{{Data: data}}})

				base := nodes[0].Next()
				if len(base) != 1 {
					t.Fatalf("#%d: len(ents) = %d, want 1", i, len(base))
				}
				if !reflect.DeepEqual(base[0].Data, data) {
					t.Errorf("#%d: data = %s, want %s", i, base[0].Data, data)
				}
				for k := 1; k < tt.size; k++ {
					g := nodes[k].Next()
					if !reflect.DeepEqual(g, base) {
						t.Errorf("#%d.%d: ent = %v, want %v", i, k, g, base)
					}
				}
			}
		}
	}
}

// This function is full of heck now. It will go away when we finish our
// network Interface, and ticker infrastructure.
func buildCluster(size int) (nt *network, nodes []*Node) {
	nodes = make([]*Node, size)
	nis := make([]Interface, size)
	for i := range nodes {
		nodes[i] = New(i, defaultHeartbeat, defaultElection)
		nis[i] = nodes[i]
	}
	nt = newNetwork(nis...)

	Dictate(nodes[0]).Next()
	for i := 1; i < size; i++ {
		nt.send(nodes[0].newConfMessage(configAdd, &Config{NodeId: i}))
		for j := 0; j < i; j++ {
			nodes[j].Next()
		}
	}

	for i := 0; i < 10*defaultHeartbeat; i++ {
		nodes[0].Tick()
	}
	msgs := nodes[0].Msgs()
	nt.send(msgs...)

	for _, n := range nodes {
		n.Next()
	}
	return
}
