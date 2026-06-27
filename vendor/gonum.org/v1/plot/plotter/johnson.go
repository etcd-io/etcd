// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

// johnson implements Johnson's "Finding all the elementary
// circuits of a directed graph" algorithm. SIAM J. Comput. 4(1):1975.
//
// Comments in the johnson methods are kept in sync with the comments
// and labels from the paper.
type johnson struct {
	adjacent graph // SCC adjacency list.
	b        []set // Johnson's "B-list".
	blocked  []bool
	s        int

	stack []int

	result [][]int
}

// cyclesIn returns the set of elementary cycles in the graph g.
func cyclesIn(g graph) [][]int {
	j := johnson{
		adjacent: g.clone(),
		b:        make([]set, len(g)),
		blocked:  make([]bool, len(g)),
	}

	// len(j.adjacent) will be the order of g until Tarjan's analysis
	// finds no SCC, at which point t.sccSubGraph returns nil and the
	// loop breaks.
	for j.s < len(j.adjacent)-1 {
		// We use the previous SCC adjacency to reduce the work needed.
		t := newTarjan(j.adjacent.subgraph(j.s))
		// A_k = adjacency structure of strong component K with least
		//       vertex in subgraph of G induced by {s, s+1, ... ,n}.
		j.adjacent = t.sccSubGraph(2) // Only allow SCCs with >= 2 vertices.
		if len(j.adjacent) == 0 {
			break
		}

		// s = least vertex in V_k
		for _, v := range j.adjacent {
			s := len(j.adjacent)
			for n := range v {
				if n < s {
					s = n
				}
			}
			if s < j.s {
				j.s = s
			}
		}
		for i, v := range j.adjacent {
			if len(v) > 0 {
				j.blocked[i] = false
				j.b[i] = make(set)
			}
		}

		//L3:
		_ = j.circuit(j.s)
		j.s++
	}

	return j.result
}

// circuit is the CIRCUIT sub-procedure in the paper.
func (j *johnson) circuit(v int) bool {
	f := false
	j.stack = append(j.stack, v)
	j.blocked[v] = true

	//L1:
	for w := range j.adjacent[v] {
		if w == j.s {
			// Output circuit composed of stack followed by s.
			r := make([]int, len(j.stack)+1)
			copy(r, j.stack)
			r[len(r)-1] = j.s
			j.result = append(j.result, r)
			f = true
		} else if !j.blocked[w] {
			if j.circuit(w) {
				f = true
			}
		}
	}

	//L2:
	if f {
		j.unblock(v)
	} else {
		for w := range j.adjacent[v] {
			j.b[w][v] = struct{}{}
		}
	}
	j.stack = j.stack[:len(j.stack)-1]

	return f
}

// unblock is the UNBLOCK sub-procedure in the paper.
func (j *johnson) unblock(u int) {
	j.blocked[u] = false
	for w := range j.b[u] {
		delete(j.b[u], w)
		if j.blocked[w] {
			j.unblock(w)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// tarjan implements Tarjan's strongly connected component finding
// algorithm. The implementation is from the pseudocode at
//
// http://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm?oldid=642744644
type tarjan struct {
	g graph

	index      int
	indexTable []int
	lowLink    []int
	onStack    []bool

	stack []int

	sccs [][]int
}

// newTarjan returns a tarjan with the sccs field filled with the
// strongly connected components of the directed graph g.
func newTarjan(g graph) *tarjan {
	t := tarjan{
		g: g,

		indexTable: make([]int, len(g)),
		lowLink:    make([]int, len(g)),
		onStack:    make([]bool, len(g)),
	}
	for v := range t.g {
		if t.indexTable[v] == 0 {
			t.strongconnect(v)
		}
	}
	return &t
}

// strongconnect is the strongconnect function described in the
// wikipedia article.
func (t *tarjan) strongconnect(v int) {
	// Set the depth index for v to the smallest unused index.
	t.index++
	t.indexTable[v] = t.index
	t.lowLink[v] = t.index
	t.stack = append(t.stack, v)
	t.onStack[v] = true

	// Consider successors of v.
	for w := range t.g[v] {
		if t.indexTable[w] == 0 {
			// Successor w has not yet been visited; recur on it.
			t.strongconnect(w)
			t.lowLink[v] = min(t.lowLink[v], t.lowLink[w])
		} else if t.onStack[w] {
			// Successor w is in stack s and hence in the current SCC.
			t.lowLink[v] = min(t.lowLink[v], t.indexTable[w])
		}
	}

	// If v is a root node, pop the stack and generate an SCC.
	if t.lowLink[v] == t.indexTable[v] {
		// Start a new strongly connected component.
		var (
			scc []int
			w   int
		)
		for {
			w, t.stack = t.stack[len(t.stack)-1], t.stack[:len(t.stack)-1]
			t.onStack[w] = false
			// Add w to current strongly connected component.
			scc = append(scc, w)
			if w == v {
				break
			}
		}
		// Output the current strongly connected component.
		t.sccs = append(t.sccs, scc)
	}
}

// sccSubGraph returns the graph of the tarjan's strongly connected
// components with each SCC containing at least min vertices.
// sccSubGraph returns nil if there is no SCC with at least min
// members.
func (t *tarjan) sccSubGraph(min int) graph {
	if len(t.g) == 0 {
		return nil
	}
	sub := make(graph, len(t.g))

	var n int
	for _, scc := range t.sccs {
		if len(scc) < min {
			continue
		}
		n++
		for _, u := range scc {
			for _, v := range scc {
				if _, ok := t.g[u][v]; ok {
					if sub[u] == nil {
						sub[u] = make(set)
					}
					sub[u][v] = struct{}{}
				}
			}
		}
	}
	if n == 0 {
		return nil
	}

	return sub
}

// set is an integer set.
type set map[int]struct{}

// graph is an edge list representation of a graph.
type graph []set

// remove deletes edges that make up the given paths from the graph.
func (g graph) remove(paths [][]int) {
	for _, p := range paths {
		for i, u := range p[:len(p)-1] {
			delete(g[u], p[i+1])
		}
	}
}

// subgraph returns a subgraph of g induced by {s, s+1, ... , n}. The
// subgraph is destructively generated in g.
func (g graph) subgraph(s int) graph {
	for u := range g[:s] {
		g[u] = nil
	}
	for u, e := range g[s:] {
		for v := range e {
			if v < s {
				delete(g[u+s], v)
			}
		}
	}
	return g
}

// clone returns a deep copy of the graph g.
func (g graph) clone() graph {
	c := make(graph, len(g))
	for u, e := range g {
		for v := range e {
			if c[u] == nil {
				c[u] = make(set)
			}
			c[u][v] = struct{}{}
		}
	}
	return c
}
