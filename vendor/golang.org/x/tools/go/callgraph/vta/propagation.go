// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vta

import (
	"go/types"
	"iter"
	"slices"

	"golang.org/x/tools/go/callgraph/vta/internal/trie"
	"golang.org/x/tools/go/ssa"

	"golang.org/x/tools/go/types/typeutil"
)

// scc computes strongly connected components (SCCs) of `g` using the
// classical Tarjan's algorithm for SCCs. The result is two slices:
//   - sccs: the SCCs, each represented as a slice of node indices
//   - idxToSccID: the inverse map, from node index to SCC number.
//
// The SCCs are sorted in reverse topological order: for SCCs
// with ids X and Y s.t. X < Y, Y comes before X in the topological order.
func scc(g *vtaGraph) (sccs [][]idx, idxToSccID []int) {
	// standard data structures used by Tarjan's algorithm.
	type state struct {
		pre     int // preorder of the node (0 if unvisited)
		lowLink int
		onStack bool
	}
	states := make([]state, g.numNodes())
	var stack []idx

	idxToSccID = make([]int, g.numNodes())
	nextPre := 0

	var doSCC func(idx)
	doSCC = func(n idx) {
		nextPre++
		ns := &states[n]
		*ns = state{pre: nextPre, lowLink: nextPre, onStack: true}
		stack = append(stack, n)

		for s := range g.successors(n) {
			if ss := &states[s]; ss.pre == 0 {
				// Analyze successor s that has not been visited yet.
				doSCC(s)
				ns.lowLink = min(ns.lowLink, ss.lowLink)
			} else if ss.onStack {
				// The successor is on the stack, meaning it has to be
				// in the current SCC.
				ns.lowLink = min(ns.lowLink, ss.pre)
			}
		}

		// if n is a root node, pop the stack and generate a new SCC.
		if ns.lowLink == ns.pre {
			sccStart := slicesLastIndex(stack, n)
			scc := slices.Clone(stack[sccStart:])
			stack = stack[:sccStart]
			sccID := len(sccs)
			sccs = append(sccs, scc)
			for _, w := range scc {
				states[w].onStack = false
				idxToSccID[w] = sccID
			}
		}
	}

	for n, nn := 0, g.numNodes(); n < nn; n++ {
		if states[n].pre == 0 {
			doSCC(idx(n))
		}
	}

	return sccs, idxToSccID
}

// slicesLastIndex returns the index of the last occurrence of v in s, or -1 if v is
// not present in s.
//
// slicesLastIndex iterates backwards through the elements of s, stopping when the ==
// operator determines an element is equal to v.
func slicesLastIndex[S ~[]E, E comparable](s S, v E) int {
	// TODO: move to / dedup with slices.LastIndex
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == v {
			return i
		}
	}
	return -1
}

// propType represents type information being propagated
// over the vta graph. f != nil only for function nodes
// and nodes reachable from function nodes. There, we also
// remember the actual *ssa.Function in order to more
// precisely model higher-order flow.
type propType struct {
	typ types.Type
	f   *ssa.Function
}

// propTypeMap is an auxiliary structure that serves
// the role of a map from nodes to a set of propTypes.
type propTypeMap map[node]*trie.MutMap

// propTypes returns an iterator for the propTypes associated with
// node `n` in map `ptm`.
func (ptm propTypeMap) propTypes(n node) iter.Seq[propType] {
	return func(yield func(propType) bool) {
		if types := ptm[n]; types != nil {
			types.M.Range(func(_ uint64, elem any) bool {
				return yield(elem.(propType))
			})
		}
	}
}

// propagate reduces the `graph` based on its SCCs and
// then propagates type information through the reduced
// graph. The result is a map from nodes to a set of types
// and functions, stemming from higher-order data flow,
// reaching the node. `canon` is used for type uniqueness.
func propagate(graph *vtaGraph, canon *typeutil.Map) propTypeMap {
	sccs, idxToSccID := scc(graph)

	// propTypeIds are used to create unique ids for
	// propType, to be used for trie-based type sets.
	propTypeIds := make(map[propType]uint64)
	// Id creation is based on == equality, which works
	// as types are canonicalized (see getPropType).
	propTypeId := func(p propType) uint64 {
		if id, ok := propTypeIds[p]; ok {
			return id
		}
		id := uint64(len(propTypeIds))
		propTypeIds[p] = id
		return id
	}
	builder := trie.NewBuilder()
	// Initialize sccToTypes to avoid repeated check
	// for initialization later.
	sccToTypes := make([]*trie.MutMap, len(sccs))
	for sccID, scc := range sccs {
		typeSet := builder.MutEmpty()
		for _, idx := range scc {
			if n := graph.node[idx]; hasInitialTypes(n) {
				// add the propType for idx to typeSet.
				pt := getPropType(n, canon)
				typeSet.Update(propTypeId(pt), pt)
			}
		}
		sccToTypes[sccID] = &typeSet
	}

	for i, scc := range slices.Backward(sccs) {
		nextSccs := make(map[int]empty)
		for _, n := range scc {
			for succ := range graph.successors(n) {
				nextSccs[idxToSccID[succ]] = empty{}
			}
		}
		// Propagate types to all successor SCCs.
		for nextScc := range nextSccs {
			sccToTypes[nextScc].Merge(sccToTypes[i].M)
		}
	}
	nodeToTypes := make(propTypeMap, graph.numNodes())
	for sccID, scc := range sccs {
		types := sccToTypes[sccID]
		for _, idx := range scc {
			nodeToTypes[graph.node[idx]] = types
		}
	}
	return nodeToTypes
}

// hasInitialTypes check if a node can have initial types.
// Returns true iff `n` is not a panic, recover, nestedPtr*
// node, nor a node whose type is an interface.
func hasInitialTypes(n node) bool {
	switch n.(type) {
	case panicArg, recoverReturn, nestedPtrFunction, nestedPtrInterface:
		return false
	default:
		return !types.IsInterface(n.Type())
	}
}

// getPropType creates a propType for `node` based on its type.
// propType.typ is always node.Type(). If node is function, then
// propType.val is the underlying function; nil otherwise.
func getPropType(node node, canon *typeutil.Map) propType {
	t := canonicalize(node.Type(), canon)
	if fn, ok := node.(function); ok {
		return propType{f: fn.f, typ: t}
	}
	return propType{f: nil, typ: t}
}
