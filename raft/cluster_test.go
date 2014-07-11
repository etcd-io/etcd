package raft

import (
	"reflect"
	"testing"
)

// TestBuildCluster ensures cluster with various size could be built.
func TestBuildCluster(t *testing.T) {
	tests := []struct {
		size int
		ids  []int64
	}{
		{1, nil},
		{3, nil},
		{5, nil},
		{7, nil},
		{9, nil},
		{13, nil},
		{51, nil},
		{1, []int64{1}},
		{3, []int64{1, 3, 5}},
		{5, []int64{1, 4, 7, 10, 13}},
	}

	for i, tt := range tests {
		_, nodes := buildCluster(tt.size, tt.ids)

		base := ltoa(nodes[0].sm.log)
		for j, n := range nodes {
			// ensure same log
			l := ltoa(n.sm.log)
			if g := diffu(base, l); g != "" {
				t.Errorf("#%d.%d: log diff:\n%s", i, j, g)
			}

			// ensure same leader
			var w int64
			if tt.ids != nil {
				w = tt.ids[0]
			}
			if g := n.sm.lead.Get(); g != w {
				t.Errorf("#%d.%d: lead = %d, want %d", i, j, g, w)
			}

			// ensure same peer map
			p := map[int64]struct{}{}
			for k := range n.sm.ins {
				p[k] = struct{}{}
			}
			wp := map[int64]struct{}{}
			for k := 0; k < tt.size; k++ {
				if tt.ids != nil {
					wp[tt.ids[k]] = struct{}{}
				} else {
					wp[int64(k)] = struct{}{}
				}
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
		nt, nodes := buildCluster(tt.size, nil)

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
func buildCluster(size int, ids []int64) (nt *network, nodes []*Node) {
	if ids == nil {
		ids = make([]int64, size)
		for i := 0; i < size; i++ {
			ids[i] = int64(i)
		}
	}

	nodes = make([]*Node, size)
	nis := make([]Interface, size)
	for i := range nodes {
		nodes[i] = New(ids[i], defaultHeartbeat, defaultElection)
		nis[i] = nodes[i]
	}
	nt = newNetwork(nis...)

	lead := dictate(nodes[0])
	lead.Next()
	for i := 1; i < size; i++ {
		lead.Add(ids[i], "", nil)
		nt.send(lead.Msgs()...)
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
