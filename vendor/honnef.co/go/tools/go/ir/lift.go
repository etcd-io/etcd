// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file defines the lifting pass which tries to "lift" Alloc
// cells (new/local variables) into SSA registers, replacing loads
// with the dominating stored value, eliminating loads and stores, and
// inserting φ- and σ-nodes as needed.

// Cited papers and resources:
//
// Ron Cytron et al. 1991. Efficiently computing SSA form...
// https://doi.acm.org/10.1145/115372.115320
//
// Cooper, Harvey, Kennedy.  2001.  A Simple, Fast Dominance Algorithm.
// Software Practice and Experience 2001, 4:1-10.
// https://www.hipersoft.rice.edu/grads/publications/dom14.pdf
//
// Daniel Berlin, llvmdev mailing list, 2012.
// https://lists.cs.uiuc.edu/pipermail/llvmdev/2012-January/046638.html
// (Be sure to expand the whole thread.)
//
// C. Scott Ananian. 1997. The static single information form.
//
// Jeremy Singer. 2006. Static program analysis based on virtual register renaming.

// TODO(adonovan): opt: there are many optimizations worth evaluating, and
// the conventional wisdom for SSA construction is that a simple
// algorithm well engineered often beats those of better asymptotic
// complexity on all but the most egregious inputs.
//
// Danny Berlin suggests that the Cooper et al. algorithm for
// computing the dominance frontier is superior to Cytron et al.
// Furthermore he recommends that rather than computing the DF for the
// whole function then renaming all alloc cells, it may be cheaper to
// compute the DF for each alloc cell separately and throw it away.
//
// Consider exploiting liveness information to avoid creating dead
// φ-nodes which we then immediately remove.
//
// Also see many other "TODO: opt" suggestions in the code.

import (
	"encoding/binary"
	"fmt"
	"os"
	"slices"
)

// If true, show diagnostic information at each step of lifting.
// Very verbose.
const debugLifting = false

// domFrontier maps each block to the set of blocks in its dominance
// frontier.  The outer slice is conceptually a map keyed by
// Block.Index.  The inner slice is conceptually a set, possibly
// containing duplicates.
//
// TODO(adonovan): opt: measure impact of dups; consider a packed bit
// representation, e.g. big.Int, and bitwise parallel operations for
// the union step in the Children loop.
//
// domFrontier's methods mutate the slice's elements but not its
// length, so their receivers needn't be pointers.
type domFrontier BlockMap[[]*BasicBlock]

func (df domFrontier) add(u, v *BasicBlock) {
	df[u.Index] = append(df[u.Index], v)
}

// build builds the dominance frontier df for the dominator tree of
// fn, using the algorithm found in A Simple, Fast Dominance
// Algorithm, Figure 5.
//
// TODO(adonovan): opt: consider Berlin approach, computing pruned SSA
// by pruning the entire IDF computation, rather than merely pruning
// the DF -> IDF step.
func (df domFrontier) build(fn *Function) {
	for _, b := range fn.Blocks {
		preds := b.Preds[0:len(b.Preds):len(b.Preds)]
		if b == fn.Exit {
			for i, v := range fn.fakeExits.values {
				if v {
					preds = append(preds, fn.Blocks[i])
				}
			}
		}
		if len(preds) >= 2 {
			for _, p := range preds {
				runner := p
				for runner != b.dom.idom {
					df.add(runner, b)
					runner = runner.dom.idom
				}
			}
		}
	}
}

func buildDomFrontier(fn *Function) domFrontier {
	df := make(domFrontier, len(fn.Blocks))
	df.build(fn)
	return df
}

type postDomFrontier BlockMap[[]*BasicBlock]

func (rdf postDomFrontier) add(u, v *BasicBlock) {
	rdf[u.Index] = append(rdf[u.Index], v)
}

func (rdf postDomFrontier) build(fn *Function) {
	for _, b := range fn.Blocks {
		succs := b.Succs[0:len(b.Succs):len(b.Succs)]
		if fn.fakeExits.Has(b) {
			succs = append(succs, fn.Exit)
		}
		if len(succs) >= 2 {
			for _, s := range succs {
				runner := s
				for runner != b.pdom.idom {
					rdf.add(runner, b)
					runner = runner.pdom.idom
				}
			}
		}
	}
}

func buildPostDomFrontier(fn *Function) postDomFrontier {
	rdf := make(postDomFrontier, len(fn.Blocks))
	rdf.build(fn)
	return rdf
}

func removeInstr(refs []Instruction, instr Instruction) []Instruction {
	return removeInstrsIf(refs, func(i Instruction) bool { return i == instr })
}

func removeInstrsIf(refs []Instruction, p func(Instruction) bool) []Instruction {
	return slices.DeleteFunc(refs, p)
}

func clearInstrs(instrs []Instruction) {
	for i := range instrs {
		instrs[i] = nil
	}
}

func numberNodesPerBlock(f *Function) {
	for _, b := range f.Blocks {
		var base ID
		for _, instr := range b.Instrs {
			if instr == nil {
				continue
			}
			instr.setID(base)
			base++
		}
	}
}

// lift replaces local and new Allocs accessed only with
// load/store by IR registers, inserting φ- and σ-nodes where necessary.
// The result is a program in pruned SSI form.
//
// Preconditions:
// - fn has no dead blocks (blockopt has run).
// - Def/use info (Operands and Referrers) is up-to-date.
// - The dominator tree is up-to-date.
func lift(fn *Function) bool {
	// TODO(adonovan): opt: lots of little optimizations may be
	// worthwhile here, especially if they cause us to avoid
	// buildDomFrontier.  For example:
	//
	// - Alloc never loaded?  Eliminate.
	// - Alloc never stored?  Replace all loads with a zero constant.
	// - Alloc stored once?  Replace loads with dominating store;
	//   don't forget that an Alloc is itself an effective store
	//   of zero.
	// - Alloc used only within a single block?
	//   Use degenerate algorithm avoiding φ-nodes.
	// - Consider synergy with scalar replacement of aggregates (SRA).
	//   e.g. *(&x.f) where x is an Alloc.
	//   Perhaps we'd get better results if we generated this as x.f
	//   i.e. Field(x, .f) instead of Load(FieldIndex(x, .f)).
	//   Unclear.
	//
	// But we will start with the simplest correct code.
	var df domFrontier
	var rdf postDomFrontier
	var closure *closure
	var newPhis BlockMap[[]newPhi]
	var newSigmas BlockMap[[]newSigma]

	// During this pass we will replace some BasicBlock.Instrs
	// (allocs, loads and stores) with nil, keeping a count in
	// BasicBlock.gaps.  At the end we will reset Instrs to the
	// concatenation of all non-dead newPhis and non-nil Instrs
	// for the block, reusing the original array if space permits.

	// While we're here, we also eliminate 'rundefers'
	// instructions and ssa:deferstack() in functions that contain no
	// 'defer' instructions. Eliminate ssa:deferstack() if it does not
	// escape.
	usesDefer := false
	deferstackAlloc, deferstackCall := deferstackPreamble(fn)
	eliminateDeferStack := deferstackAlloc != nil && !deferstackAlloc.Heap

	// Determine which allocs we can lift and number them densely.
	// The renaming phase uses this numbering for compact maps.
	numAllocs := 0

	instructions := make(BlockMap[liftInstructions], len(fn.Blocks))
	for i := range instructions {
		instructions[i].insertInstructions = map[Instruction][]Instruction{}
	}

	// Number nodes, for liftable
	numberNodesPerBlock(fn)

	for _, b := range fn.Blocks {
		b.gaps = 0
		b.rundefers = 0

		for _, instr := range b.Instrs {
			switch instr := instr.(type) {
			case *Alloc:
				if !liftable(instr, instructions) {
					instr.index = -1
					continue
				}

				if numAllocs == 0 {
					df = buildDomFrontier(fn)
					rdf = buildPostDomFrontier(fn)
					if len(fn.Blocks) > 2 {
						closure = transitiveClosure(fn)
					}
					newPhis = make(BlockMap[[]newPhi], len(fn.Blocks))
					newSigmas = make(BlockMap[[]newSigma], len(fn.Blocks))

					if debugLifting {
						title := false
						for i, blocks := range df {
							if blocks != nil {
								if !title {
									fmt.Fprintf(os.Stderr, "Dominance frontier of %s:\n", fn)
									title = true
								}
								fmt.Fprintf(os.Stderr, "\t%s: %s\n", fn.Blocks[i], blocks)
							}
						}
					}
				}
				instr.index = numAllocs
				numAllocs++
			case *Defer:
				usesDefer = true
				if eliminateDeferStack {
					// Clear _DeferStack and remove references to loads
					if instr._DeferStack != nil {
						if refs := instr._DeferStack.Referrers(); refs != nil {
							*refs = removeInstr(*refs, instr)
						}
						instr._DeferStack = nil
					}
				}
			case *RunDefers:
				b.rundefers++
			}
		}
	}

	if numAllocs > 0 {
		for _, b := range fn.Blocks {
			work := instructions[b.Index]
			for _, rename := range work.renameAllocs {
				for _, instr_ := range b.Instrs[rename.startingAt:] {
					replace(instr_, rename.from, rename.to)
				}
			}
		}

		for _, b := range fn.Blocks {
			work := instructions[b.Index]
			if len(work.insertInstructions) != 0 {
				newInstrs := make([]Instruction, 0, len(fn.Blocks)+len(work.insertInstructions)*3)
				for _, instr := range b.Instrs {
					if add, ok := work.insertInstructions[instr]; ok {
						newInstrs = append(newInstrs, add...)
					}
					newInstrs = append(newInstrs, instr)
				}
				b.Instrs = newInstrs
			}
		}

		// TODO(dh): remove inserted allocs that end up unused after lifting.

		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				if instr, ok := instr.(*Alloc); ok && instr.index >= 0 {
					liftAlloc(closure, df, rdf, instr, newPhis, newSigmas)
				}
			}
		}

		// renaming maps an alloc (keyed by index) to its replacement
		// value.  Initially the renaming contains nil, signifying the
		// zero constant of the appropriate type; we construct the
		// Const lazily at most once on each path through the domtree.
		// TODO(adonovan): opt: cache per-function not per subtree.
		renaming := make([]Value, numAllocs)

		// Renaming.
		rename(fn.Blocks[0], renaming, newPhis, newSigmas)

		simplifyPhisAndSigmas(newPhis, newSigmas)

		// Eliminate dead φ- and σ-nodes.
		markLiveNodes(fn.Blocks, newPhis, newSigmas)

		// Eliminate ssa:deferstack() call.
		if eliminateDeferStack {
			b := deferstackCall.block
			for i, instr := range b.Instrs {
				if instr == deferstackCall {
					b.Instrs[i] = nil
					b.gaps++
					break
				}
			}
		}
	}

	// Prepend remaining live φ-nodes to each block and possibly kill rundefers.
	for _, b := range fn.Blocks {
		var head []Instruction
		if numAllocs > 0 {
			nps := newPhis[b.Index]
			head = make([]Instruction, 0, len(nps))
			for _, pred := range b.Preds {
				nss := newSigmas[pred.Index]
				idx := pred.succIndex(b)
				for _, newSigma := range nss {
					if sigma := newSigma.sigmas[idx]; sigma != nil && sigma.live {
						head = append(head, sigma)

						// we didn't populate referrers before, as most
						// sigma nodes will be killed
						if refs := sigma.X.Referrers(); refs != nil {
							*refs = append(*refs, sigma)
						}
					} else if sigma != nil {
						sigma.block = nil
					}
				}
			}
			for _, np := range nps {
				if np.phi.live {
					head = append(head, np.phi)
				} else {
					for _, edge := range np.phi.Edges {
						if refs := edge.Referrers(); refs != nil {
							*refs = removeInstr(*refs, np.phi)
						}
					}
					np.phi.block = nil
				}
			}
		}

		rundefersToKill := b.rundefers
		if usesDefer {
			rundefersToKill = 0
		}

		j := len(head)
		if j+b.gaps+rundefersToKill == 0 {
			continue // fast path: no new phis or gaps
		}

		// We could do straight copies instead of element-wise copies
		// when both b.gaps and rundefersToKill are zero. However,
		// that seems to only be the case ~1% of the time, which
		// doesn't seem worth the extra branch.

		// Remove dead instructions, add phis and sigmas
		ns := len(b.Instrs) + j - b.gaps - rundefersToKill
		if ns <= cap(b.Instrs) {
			// b.Instrs has enough capacity to store all instructions

			// OPT(dh): check cap vs the actually required space; if
			// there is a big enough difference, it may be worth
			// allocating a new slice, to avoid pinning memory.
			dst := b.Instrs[:cap(b.Instrs)]
			i := len(dst) - 1
			for n := len(b.Instrs) - 1; n >= 0; n-- {
				instr := dst[n]
				if instr == nil {
					continue
				}
				if !usesDefer {
					if _, ok := instr.(*RunDefers); ok {
						continue
					}
				}
				dst[i] = instr
				i--
			}
			off := i + 1 - len(head)
			// aid GC
			clearInstrs(dst[:off])
			dst = dst[off:]
			copy(dst, head)
			b.Instrs = dst
		} else {
			// not enough space, so allocate a new slice and copy
			// over.
			dst := make([]Instruction, ns)
			copy(dst, head)

			for _, instr := range b.Instrs {
				if instr == nil {
					continue
				}
				if !usesDefer {
					if _, ok := instr.(*RunDefers); ok {
						continue
					}
				}
				dst[j] = instr
				j++
			}
			b.Instrs = dst
		}
	}

	// Remove any fn.Locals that were lifted.
	j := 0
	for _, l := range fn.Locals {
		if l.index < 0 {
			fn.Locals[j] = l
			j++
		}
	}
	// Nil out fn.Locals[j:] to aid GC.
	for i := j; i < len(fn.Locals); i++ {
		fn.Locals[i] = nil
	}
	fn.Locals = fn.Locals[:j]

	return numAllocs > 0
}

func hasDirectReferrer(instr Instruction) bool {
	for _, instr := range *instr.Referrers() {
		switch instr.(type) {
		case *Phi, *Sigma:
			// ignore
		default:
			return true
		}
	}
	return false
}

func markLiveNodes(blocks []*BasicBlock, newPhis BlockMap[[]newPhi], newSigmas BlockMap[[]newSigma]) {
	// Phis and sigmas may become dead due to optimization passes. We may also insert more nodes than strictly
	// necessary, e.g. sigma nodes for constants, which will never be used.

	// Phi and sigma nodes are considered live if a non-phi, non-sigma
	// node uses them. Once we find a node that is live, we mark all
	// of its operands as used, too.
	for _, npList := range newPhis {
		for _, np := range npList {
			phi := np.phi
			if !phi.live && hasDirectReferrer(phi) {
				markLivePhi(phi)
			}
		}
	}
	for _, npList := range newSigmas {
		for _, np := range npList {
			for _, sigma := range np.sigmas {
				if sigma != nil && !sigma.live && hasDirectReferrer(sigma) {
					markLiveSigma(sigma)
				}
			}
		}
	}
	// Existing φ-nodes due to && and || operators
	// are all considered live (see Go issue 19622).
	for _, b := range blocks {
		for _, phi := range b.phis() {
			markLivePhi(phi.(*Phi))
		}
	}
}

func markLivePhi(phi *Phi) {
	phi.live = true
	for _, rand := range phi.Edges {
		switch rand := rand.(type) {
		case *Phi:
			if !rand.live {
				markLivePhi(rand)
			}
		case *Sigma:
			if !rand.live {
				markLiveSigma(rand)
			}
		}
	}
}

func markLiveSigma(sigma *Sigma) {
	sigma.live = true
	switch rand := sigma.X.(type) {
	case *Phi:
		if !rand.live {
			markLivePhi(rand)
		}
	case *Sigma:
		if !rand.live {
			markLiveSigma(rand)
		}
	}
}

// simplifyPhisAndSigmas removes duplicate phi and sigma nodes,
// and replaces trivial phis with non-phi alternatives. Phi
// nodes where all edges are identical, or consist of only the phi
// itself and one other value, may be replaced with the value.
func simplifyPhisAndSigmas(newPhis BlockMap[[]newPhi], newSigmas BlockMap[[]newSigma]) {
	// temporary numbering of values used in phis so that we can build map keys
	var id ID
	for _, npList := range newPhis {
		for _, np := range npList {
			for _, edge := range np.phi.Edges {
				edge.setID(id)
				id++
			}
		}
	}
	// find all phis that are trivial and can be replaced with a
	// non-phi value. run until we reach a fixpoint, because replacing
	// a phi may make other phis trivial.
	for changed := true; changed; {
		changed = false
		for _, npList := range newPhis {
			for _, np := range npList {
				if np.phi.live {
					// we're reusing 'live' to mean 'dead' in the context of simplifyPhisAndSigmas
					continue
				}
				if r, ok := isUselessPhi(np.phi); ok {
					// useless phi, replace its uses with the
					// replacement value. the dead phi pass will clean
					// up the phi afterwards.
					replaceAll(np.phi, r)
					np.phi.live = true
					changed = true
				}
			}
		}

		// Replace duplicate sigma nodes with a single node. These nodes exist when multiple allocs get replaced with the
		// same dominating store.
		for _, sigmaList := range newSigmas {
			primarySigmas := map[struct {
				succ int
				v    Value
			}]*Sigma{}
			for _, sigmas := range sigmaList {
				for succ, sigma := range sigmas.sigmas {
					if sigma == nil {
						continue
					}
					if sigma.live {
						// we're reusing 'live' to mean 'dead' in the context of simplifyPhisAndSigmas
						continue
					}
					key := struct {
						succ int
						v    Value
					}{succ, sigma.X}
					if alt, ok := primarySigmas[key]; ok {
						replaceAll(sigma, alt)
						sigma.live = true
						changed = true
					} else {
						primarySigmas[key] = sigma
					}
				}
			}
		}

		// Replace duplicate phi nodes with a single node. As far as we know, these duplicate nodes only ever exist
		// because of the previous sigma deduplication.
		keyb := make([]byte, 0, 4*8)
		for _, npList := range newPhis {
			primaryPhis := map[string]*Phi{}
			for _, np := range npList {
				if np.phi.live {
					continue
				}
				if n := len(np.phi.Edges) * 8; cap(keyb) >= n {
					keyb = keyb[:n]
				} else {
					keyb = make([]byte, n, n*2)
				}
				for i, e := range np.phi.Edges {
					binary.LittleEndian.PutUint64(keyb[i*8:i*8+8], uint64(e.ID()))
				}
				if alt, ok := primaryPhis[string(keyb)]; ok {
					replaceAll(np.phi, alt)
					np.phi.live = true
					changed = true
				} else {
					primaryPhis[string(keyb)] = np.phi
				}
			}
		}

	}

	for _, npList := range newPhis {
		for _, np := range npList {
			np.phi.live = false
			for _, edge := range np.phi.Edges {
				edge.setID(0)
			}
		}
	}

	for _, sigmaList := range newSigmas {
		for _, sigmas := range sigmaList {
			for _, sigma := range sigmas.sigmas {
				if sigma != nil {
					sigma.live = false
				}
			}
		}
	}
}

type BlockSet struct {
	idx    int
	values []bool
	count  int
}

func NewBlockSet(size int) *BlockSet {
	return &BlockSet{values: make([]bool, size)}
}

func (s *BlockSet) Set(s2 *BlockSet) {
	copy(s.values, s2.values)
	s.count = 0
	for _, v := range s.values {
		if v {
			s.count++
		}
	}
}

func (s *BlockSet) Num() int {
	return s.count
}

func (s *BlockSet) Has(b *BasicBlock) bool {
	if b.Index >= len(s.values) {
		return false
	}
	return s.values[b.Index]
}

// add adds b to the set and returns true if the set changed.
func (s *BlockSet) Add(b *BasicBlock) bool {
	if s.values[b.Index] {
		return false
	}
	s.count++
	s.values[b.Index] = true
	s.idx = b.Index

	return true
}

func (s *BlockSet) Clear() {
	for j := range s.values {
		s.values[j] = false
	}
	s.count = 0
}

// take removes an arbitrary element from a set s and
// returns its index, or returns -1 if empty.
func (s *BlockSet) Take() int {
	// [i, end]
	for i := s.idx; i < len(s.values); i++ {
		if s.values[i] {
			s.values[i] = false
			s.idx = i
			s.count--
			return i
		}
	}

	// [start, i)
	for i := 0; i < s.idx; i++ {
		if s.values[i] {
			s.values[i] = false
			s.idx = i
			s.count--
			return i
		}
	}

	return -1
}

type closure struct {
	span       []uint32
	reachables BlockMap[interval]
}

type interval uint32

const (
	flagMask   = 1 << 31
	numBits    = 20
	lengthBits = 32 - numBits - 1
	lengthMask = (1<<lengthBits - 1) << numBits
	numMask    = 1<<numBits - 1
)

func (c closure) has(s, v *BasicBlock) bool {
	idx := uint32(v.Index)
	if idx == 1 || s.Dominates(v) {
		return true
	}
	r := c.reachable(s.Index)
	for i := 0; i < len(r); i++ {
		inv := r[i]
		var start, end uint32
		if inv&flagMask == 0 {
			// small interval
			start = uint32(inv & numMask)
			end = start + uint32(inv&lengthMask)>>numBits
		} else {
			// large interval
			i++
			start = uint32(inv & numMask)
			end = uint32(r[i])
		}
		if idx >= start && idx <= end {
			return true
		}
	}
	return false
}

func (c closure) reachable(id int) []interval {
	return c.reachables[c.span[id]:c.span[id+1]]
}

func (c closure) walk(current *BasicBlock, b *BasicBlock, visited []bool) {
	// TODO(dh): the 'current' argument seems to be unused
	// TODO(dh): there's no reason for this to be a method
	visited[b.Index] = true
	for _, succ := range b.Succs {
		if visited[succ.Index] {
			continue
		}
		visited[succ.Index] = true
		c.walk(current, succ, visited)
	}
}

func transitiveClosure(fn *Function) *closure {
	reachable := make(BlockMap[bool], len(fn.Blocks))
	c := &closure{}
	c.span = make([]uint32, len(fn.Blocks)+1)

	addInterval := func(start, end uint32) {
		if l := end - start; l <= 1<<lengthBits-1 {
			n := interval(l<<numBits | start)
			c.reachables = append(c.reachables, n)
		} else {
			n1 := interval(1<<31 | start)
			n2 := interval(end)
			c.reachables = append(c.reachables, n1, n2)
		}
	}

	for i, b := range fn.Blocks[1:] {
		for i := range reachable {
			reachable[i] = false
		}

		c.walk(b, b, reachable)
		start := ^uint32(0)
		for id, isReachable := range reachable {
			if !isReachable {
				if start != ^uint32(0) {
					end := uint32(id) - 1
					addInterval(start, end)
					start = ^uint32(0)
				}
				continue
			} else if start == ^uint32(0) {
				start = uint32(id)
			}
		}
		if start != ^uint32(0) {
			addInterval(start, uint32(len(reachable))-1)
		}

		c.span[i+2] = uint32(len(c.reachables))
	}

	return c
}

// newPhi is a pair of a newly introduced φ-node and the lifted Alloc
// it replaces.
type newPhi struct {
	phi   *Phi
	alloc *Alloc
}

type newSigma struct {
	alloc  *Alloc
	sigmas []*Sigma
}

type liftInstructions struct {
	insertInstructions map[Instruction][]Instruction
	renameAllocs       []struct {
		from       *Alloc
		to         *Alloc
		startingAt int
	}
}

// liftable determines if alloc can be lifted, and records instructions to split partially liftable allocs.
//
// In the trivial case, all uses of the alloc can be lifted. This is the case when it is only used for storing into and
// loading from. In that case, no instructions are recorded.
//
// In the more complex case, the alloc is used for storing into and loading from, but it is also used as a value, for
// example because it gets passed to a function, e.g. fn(&x). In this case, uses of the alloc fall into one of two
// categories: those that can be lifted and those that can't. A boundary forms between these two categories in the
// function's control flow: Once an unliftable use is encountered, the alloc is no longer liftable for the remainder of
// the basic block the use is in, nor in any blocks reachable from it.
//
// We record instructions that split the alloc into two allocs: one that is used in liftable uses, and one that is used
// in unliftable uses. Whenever we encounter a boundary between liftable and unliftable uses or blocks, we emit a pair
// of Load and Store that copy the value from the liftable alloc into the unliftable alloc. Taking these instructions
// into account, the normal lifting machinery will completely lift the liftable alloc, store the correct lifted values
// into the unliftable alloc, and will not at all lift the unliftable alloc.
//
// In Go syntax, the transformation looks somewhat like this:
//
//	func foo() {
//		x := 32
//		if cond {
//			println(x)
//			escape(&x)
//			println(x)
//		} else {
//			println(x)
//		}
//		println(x)
//	}
//
// transforms into
//
//	func fooSplitAlloc() {
//		x := 32
//		var x_ int
//		if cond {
//			println(x)
//			x_ = x
//			escape(&x_)
//			println(x_)
//		} else {
//			println(x)
//			x_ = x
//		}
//		println(x_)
//	}
func liftable(alloc *Alloc, instructions BlockMap[liftInstructions]) bool {
	fn := alloc.block.parent

	// Don't lift result values in functions that defer
	// calls that may recover from panic.
	if fn.hasDefer {
		if slices.Contains(fn.results, alloc) {
			return false
		}
	}

	type blockDesc struct {
		// is the block (partially) unliftable, because it contains unliftable instructions or is reachable by an unliftable block
		isUnliftable     bool
		hasLiftableLoad  bool
		hasLiftableOther bool
		// we need to emit stores in predecessors because the unliftable use is in a phi
		storeInPreds bool

		lastLiftable    int
		firstUnliftable int
	}
	blocks := make(BlockMap[blockDesc], len(fn.Blocks))
	for _, b := range fn.Blocks {
		blocks[b.Index].lastLiftable = -1
		blocks[b.Index].firstUnliftable = len(b.Instrs) + 1
	}

	// Look at all uses of the alloc and deduce which blocks have liftable or unliftable instructions.
	for _, instr := range alloc.referrers {
		// Find the first unliftable use

		desc := &blocks[instr.Block().Index]
		hasUnliftable := false
		inHead := false
		switch instr := instr.(type) {
		case *Store:
			if instr.Val == alloc {
				hasUnliftable = true
			}
		case *Load:
		case *DebugRef:
		case *Phi, *Sigma:
			inHead = true
			hasUnliftable = true
		default:
			hasUnliftable = true
		}

		if hasUnliftable {
			desc.isUnliftable = true
			if int(instr.ID()) < desc.firstUnliftable {
				desc.firstUnliftable = int(instr.ID())
			}
			if inHead {
				desc.storeInPreds = true
				desc.firstUnliftable = 0
			}
		}
	}

	for _, instr := range alloc.referrers {
		// Find the last liftable use, taking the previously calculated firstUnliftable into consideration

		desc := &blocks[instr.Block().Index]
		if int(instr.ID()) >= desc.firstUnliftable {
			continue
		}
		hasLiftable := false
		switch instr := instr.(type) {
		case *Store:
			if instr.Val != alloc {
				desc.hasLiftableOther = true
				hasLiftable = true
			}
		case *Load:
			desc.hasLiftableLoad = true
			hasLiftable = true
		case *DebugRef:
			desc.hasLiftableOther = true
		}
		if hasLiftable {
			if int(instr.ID()) > desc.lastLiftable {
				desc.lastLiftable = int(instr.ID())
			}
		}
	}

	for i := range blocks {
		// Update firstUnliftable to be one after lastLiftable. We do this to include the unliftable's preceding
		// DebugRefs in the renaming.
		if blocks[i].lastLiftable == -1 && !blocks[i].storeInPreds {
			// There are no liftable instructions (for this alloc) in this block. Set firstUnliftable to the
			// first non-head instruction to avoid inserting the store before phi instructions, which would
			// fail validation.
			first := -1
		instrLoop:
			for i, instr := range fn.Blocks[i].Instrs {
				switch instr.(type) {
				case *Phi, *Sigma:
				default:
					first = i
					break instrLoop
				}
			}
			blocks[i].firstUnliftable = first
		} else {
			blocks[i].firstUnliftable = blocks[i].lastLiftable + 1
		}
	}

	// If a block is reachable by a (partially) unliftable block, then the entirety of the block is unliftable. In that
	// case, stores have to be inserted in the predecessors.
	//
	// TODO(dh): this isn't always necessary. If the block is reachable by itself, i.e. part of a loop, then if the
	// Alloc instruction is itself part of that loop, then there is a subset of instructions in the loop that can be
	// lifted. For example:
	//
	// 	for {
	// 		x := 42
	// 		println(x)
	// 		escape(&x)
	// 	}
	//
	// The x that escapes in one iteration of the loop isn't the same x that we read from on the next iteration.
	seen := make(BlockMap[bool], len(fn.Blocks))
	var dfs func(b *BasicBlock)
	dfs = func(b *BasicBlock) {
		if seen[b.Index] {
			return
		}
		seen[b.Index] = true
		desc := &blocks[b.Index]
		desc.hasLiftableLoad = false
		desc.hasLiftableOther = false
		desc.isUnliftable = true
		desc.firstUnliftable = 0
		desc.storeInPreds = true
		for _, succ := range b.Succs {
			dfs(succ)
		}
	}
	for _, b := range fn.Blocks {
		if blocks[b.Index].isUnliftable {
			for _, succ := range b.Succs {
				dfs(succ)
			}
		}
	}

	hasLiftableLoad := false
	hasLiftableOther := false
	hasUnliftable := false
	for _, b := range fn.Blocks {
		desc := blocks[b.Index]
		hasLiftableLoad = hasLiftableLoad || desc.hasLiftableLoad
		hasLiftableOther = hasLiftableOther || desc.hasLiftableOther
		if desc.isUnliftable {
			hasUnliftable = true
		}
	}
	if !hasLiftableLoad && !hasLiftableOther {
		// There are no liftable uses
		return false
	} else if !hasUnliftable {
		// The alloc is entirely liftable without splitting
		return true
	} else if !hasLiftableLoad {
		// The alloc is not entirely liftable, and the only liftable uses are stores. While some of those stores could
		// get lifted away, it would also lead to an infinite loop when lifting to a fixpoint, because the newly created
		// allocs also get stored into repeatable and that's their only liftable uses.
		return false
	}

	// We need to insert stores for the new alloc. If a (partially) unliftable block has no unliftable
	// predecessors and the use isn't in a phi node, then the store can be inserted right before the unliftable use.
	// Otherwise, stores have to be inserted at the end of all liftable predecessors.

	newAlloc := &Alloc{Heap: true}
	newAlloc.setBlock(alloc.block)
	newAlloc.setType(alloc.typ)
	newAlloc.setSource(alloc.source)
	newAlloc.index = -1
	newAlloc.comment = "split alloc"

	{
		work := instructions[alloc.block.Index]
		work.insertInstructions[alloc] = append(work.insertInstructions[alloc], newAlloc)
	}

	predHasStore := make(BlockMap[bool], len(fn.Blocks))
	for _, b := range fn.Blocks {
		desc := &blocks[b.Index]
		bWork := &instructions[b.Index]

		if desc.isUnliftable {
			bWork.renameAllocs = append(bWork.renameAllocs, struct {
				from       *Alloc
				to         *Alloc
				startingAt int
			}{
				alloc, newAlloc, int(desc.firstUnliftable),
			})
		}

		if !desc.isUnliftable {
			continue
		}

		propagate := func(in *BasicBlock, before Instruction) {
			load := &Load{
				X: alloc,
			}
			store := &Store{
				Addr: newAlloc,
				Val:  load,
			}
			load.setType(deref(alloc.typ))
			load.setBlock(in)
			load.comment = "split alloc"
			store.setBlock(in)
			updateOperandReferrers(load)
			updateOperandReferrers(store)
			store.comment = "split alloc"

			entry := &instructions[in.Index]
			entry.insertInstructions[before] = append(entry.insertInstructions[before], load, store)
		}

		if desc.storeInPreds {
			// emit stores at the end of liftable preds
			for _, pred := range b.Preds {
				if blocks[pred.Index].isUnliftable {
					continue
				}

				if !alloc.block.Dominates(pred) {
					// Consider this cfg:
					//
					//      1
					//     /|
					//    / |
					//   ↙  ↓
					//  2--→3
					//
					// with an Alloc in block 2. It doesn't make sense to insert a store in block 1 for the jump to
					// block 3, because 1 can never see the Alloc in the first place.
					//
					// Ignoring phi nodes, an Alloc always dominates all of its uses, and phi nodes don't matter here,
					// because for the incoming edges that do matter, we do emit the stores.

					continue
				}

				if predHasStore[pred.Index] {
					// Don't generate redundant propagations. Not only is it unnecessary, it can lead to infinite loops
					// when trying to lift to a fix point, because redundant stores are liftable.
					continue
				}

				predHasStore[pred.Index] = true

				before := pred.Instrs[len(pred.Instrs)-1]
				propagate(pred, before)
			}
		} else {
			// emit store before the first unliftable use
			before := b.Instrs[desc.firstUnliftable]
			propagate(b, before)
		}
	}

	return true
}

// liftAlloc lifts alloc into registers and populates newPhis and newSigmas with all the φ- and σ-nodes it may require.
func liftAlloc(closure *closure, df domFrontier, rdf postDomFrontier, alloc *Alloc, newPhis BlockMap[[]newPhi], newSigmas BlockMap[[]newSigma]) {
	fn := alloc.Parent()

	defblocks := fn.blockset(0)
	useblocks := fn.blockset(1)
	Aphi := fn.blockset(2)
	Asigma := fn.blockset(3)
	W := fn.blockset(4)

	// Compute defblocks, the set of blocks containing a
	// definition of the alloc cell.
	for _, instr := range *alloc.Referrers() {
		switch instr := instr.(type) {
		case *Store:
			defblocks.Add(instr.Block())
		case *Load:
			useblocks.Add(instr.Block())
			for _, ref := range *instr.Referrers() {
				useblocks.Add(ref.Block())
			}
		}
	}
	// The Alloc itself counts as a (zero) definition of the cell.
	defblocks.Add(alloc.Block())

	if debugLifting {
		fmt.Fprintln(os.Stderr, "\tlifting ", alloc, alloc.Name())
	}

	// Φ-insertion.
	//
	// What follows is the body of the main loop of the insert-φ
	// function described by Cytron et al, but instead of using
	// counter tricks, we just reset the 'hasAlready' and 'work'
	// sets each iteration.  These are bitmaps so it's pretty cheap.

	// Initialize W and work to defblocks.

	for change := true; change; {
		change = false
		{
			// Traverse iterated dominance frontier, inserting φ-nodes.
			W.Set(defblocks)

			for i := W.Take(); i != -1; i = W.Take() {
				n := fn.Blocks[i]
				for _, y := range df[n.Index] {
					if Aphi.Add(y) {
						if len(*alloc.Referrers()) == 0 {
							continue
						}
						live := false
						if closure == nil {
							live = true
						} else {
							for _, ref := range *alloc.Referrers() {
								if _, ok := ref.(*Load); ok {
									if closure.has(y, ref.Block()) {
										live = true
										break
									}
								}
							}
						}
						if !live {
							continue
						}

						// Create φ-node.
						// It will be prepended to v.Instrs later, if needed.
						if len(y.Preds) == 0 {
							// The exit block may be unreachable if the function doesn't
							// return, e.g. due to an infinite loop. In that case we
							// should not replace loads in the exit block with ϕ node that
							// have no edges. Such loads exist when the function has named
							// return parameters, as the exit block loads them to turn
							// them into a Return instruction. By not replacing the loads
							// with ϕ nodes, they will later be replaced by zero
							// constants. This is arguably more correct, and more
							// importantly, it doesn't break code that assumes that phis
							// have at least one edge.
							//
							// For one instance of breakage see
							// https://staticcheck.dev/issues/1533
							continue
						}
						phi := &Phi{
							Edges: make([]Value, len(y.Preds)),
						}
						phi.comment = alloc.comment
						phi.source = alloc.source
						phi.setType(deref(alloc.Type()))
						phi.block = y
						if debugLifting {
							fmt.Fprintf(os.Stderr, "\tplace %s = %s at block %s\n", phi.Name(), phi, y)
						}
						newPhis[y.Index] = append(newPhis[y.Index], newPhi{phi, alloc})

						for _, p := range y.Preds {
							useblocks.Add(p)
						}
						change = true
						if defblocks.Add(y) {
							W.Add(y)
						}
					}
				}
			}
		}

		{
			W.Set(useblocks)
			for i := W.Take(); i != -1; i = W.Take() {
				n := fn.Blocks[i]
				for _, y := range rdf[n.Index] {
					if Asigma.Add(y) {
						sigmas := make([]*Sigma, 0, len(y.Succs))
						anyLive := false
						for _, succ := range y.Succs {
							live := false
							for _, ref := range *alloc.Referrers() {
								if closure == nil || closure.has(succ, ref.Block()) {
									live = true
									anyLive = true
									break
								}
							}
							if live {
								sigma := &Sigma{
									From: y,
									X:    alloc,
								}
								sigma.comment = alloc.comment
								sigma.source = alloc.source
								sigma.setType(deref(alloc.Type()))
								sigma.block = succ
								sigmas = append(sigmas, sigma)
							} else {
								sigmas = append(sigmas, nil)
							}
						}

						if anyLive {
							newSigmas[y.Index] = append(newSigmas[y.Index], newSigma{alloc, sigmas})
							for _, s := range y.Succs {
								defblocks.Add(s)
							}
							change = true
							if useblocks.Add(y) {
								W.Add(y)
							}
						}
					}
				}
			}
		}
	}
}

// replaceAll replaces all intraprocedural uses of x with y,
// updating x.Referrers and y.Referrers.
// Precondition: x.Referrers() != nil, i.e. x must be local to some function.
func replaceAll(x, y Value) {
	var rands []*Value
	pxrefs := x.Referrers()
	pyrefs := y.Referrers()
	for _, instr := range *pxrefs {
		switch instr := instr.(type) {
		case *CompositeValue:
			// Special case CompositeValue because it might have very large lists of operands
			//
			// OPT(dh): this loop is still expensive for large composite values
			for i, rand := range instr.Values {
				if rand == x {
					instr.Values[i] = y
				}
			}
		default:
			rands = instr.Operands(rands[:0]) // recycle storage
			for _, rand := range rands {
				if *rand != nil {
					if *rand == x {
						*rand = y
					}
				}
			}
		}
		if pyrefs != nil {
			*pyrefs = append(*pyrefs, instr) // dups ok
		}
	}
	*pxrefs = nil // x is now unreferenced
}

func replace(instr Instruction, x, y Value) {
	args := instr.Operands(nil)
	matched := false
	for _, arg := range args {
		if *arg == x {
			*arg = y
			matched = true
		}
	}
	if matched {
		yrefs := y.Referrers()
		if yrefs != nil {
			*yrefs = append(*yrefs, instr)
		}

		xrefs := x.Referrers()
		if xrefs != nil {
			*xrefs = removeInstr(*xrefs, instr)
		}
	}
}

// renamed returns the value to which alloc is being renamed,
// constructing it lazily if it's the implicit zero initialization.
func renamed(fn *Function, renaming []Value, alloc *Alloc) Value {
	v := renaming[alloc.index]
	if v == nil {
		v = emitConst(fn, zeroConst(deref(alloc.Type()), alloc.source))
		renaming[alloc.index] = v
	}
	return v
}

func copyValue(v Value, why Instruction, info CopyInfo) *Copy {
	c := &Copy{
		X:    v,
		Why:  why,
		Info: info,
	}
	if refs := v.Referrers(); refs != nil {
		*refs = append(*refs, c)
	}
	c.setType(v.Type())
	c.setSource(v.Source())
	return c
}

func splitOnNewInformation(u *BasicBlock, renaming *StackMap) {
	renaming.Push()
	defer renaming.Pop()

	rename := func(v Value, why Instruction, info CopyInfo, i int) {
		c := copyValue(v, why, info)
		c.setBlock(u)
		renaming.Set(v, c)
		u.Instrs = append(u.Instrs, nil)
		copy(u.Instrs[i+2:], u.Instrs[i+1:])
		u.Instrs[i+1] = c
	}

	replacement := func(v Value) (Value, bool) {
		r, ok := renaming.Get(v)
		if !ok {
			return nil, false
		}
		for {
			rr, ok := renaming.Get(r)
			if !ok {
				// Store replacement in the map so that future calls to replacement(v) don't have to go through the
				// iterative process again.
				renaming.Set(v, r)
				return r, true
			}
			r = rr
		}
	}

	var hasInfo func(v Value, info CopyInfo) bool
	hasInfo = func(v Value, info CopyInfo) bool {
		switch v := v.(type) {
		case *Copy:
			return (v.Info&info) == info || hasInfo(v.X, info)
		case *FieldAddr, *IndexAddr, *TypeAssert, *MakeChan, *MakeMap, *MakeSlice, *Alloc:
			return info == CopyInfoNotNil
		case Member, *Builtin:
			return info == CopyInfoNotNil
		case *Sigma:
			return hasInfo(v.X, info)
		default:
			return false
		}
	}

	var args []*Value
	for i := 0; i < len(u.Instrs); i++ {
		instr := u.Instrs[i]
		if instr == nil {
			continue
		}
		args = instr.Operands(args[:0])
		for _, arg := range args {
			if *arg == nil {
				continue
			}
			if r, ok := replacement(*arg); ok {
				*arg = r
				replace(instr, *arg, r)
			}
		}

		// TODO write some bits on why we copy values instead of encoding the actual control flow and panics

		switch instr := instr.(type) {
		case *IndexAddr:
			// Note that we rename instr.Index and instr.X even if they're already copies, because unique combinations
			// of X and Index may lead to unique information.

			// OPT we should rename both variables at once and avoid one memmove
			rename(instr.Index, instr, CopyInfoNotNegative, i)
			rename(instr.X, instr, CopyInfoNotNil, i)
			i += 2 // skip over instructions we just inserted
		case *FieldAddr:
			if !hasInfo(instr.X, CopyInfoNotNil) {
				rename(instr.X, instr, CopyInfoNotNil, i)
				i++
			}
		case *TypeAssert:
			// If we've already type asserted instr.X without comma-ok before, then it can only contain a single type,
			// and successive type assertions, no matter the type, don't tell us anything new.
			if !hasInfo(instr.X, CopyInfoNotNil|CopyInfoSingleConcreteType) {
				rename(instr.X, instr, CopyInfoNotNil|CopyInfoSingleConcreteType, i)
				i++ // skip over instruction we just inserted
			}
		case *Load:
			if !hasInfo(instr.X, CopyInfoNotNil) {
				rename(instr.X, instr, CopyInfoNotNil, i)
				i++
			}
		case *Store:
			if !hasInfo(instr.Addr, CopyInfoNotNil) {
				rename(instr.Addr, instr, CopyInfoNotNil, i)
				i++
			}
		case *MapUpdate:
			if !hasInfo(instr.Map, CopyInfoNotNil) {
				rename(instr.Map, instr, CopyInfoNotNil, i)
				i++
			}
		case CallInstruction:
			off := 0
			if !instr.Common().IsInvoke() && !hasInfo(instr.Common().Value, CopyInfoNotNil) {
				rename(instr.Common().Value, instr, CopyInfoNotNil, i)
				off++
			}
			if f, ok := instr.Common().Value.(*Builtin); ok {
				switch f.name {
				case "close":
					arg := instr.Common().Args[0]
					if !hasInfo(arg, CopyInfoNotNil|CopyInfoClosed) {
						rename(arg, instr, CopyInfoNotNil|CopyInfoClosed, i)
						off++
					}
				}
			}
			i += off
		case *SliceToArrayPointer:
			// A slice to array pointer conversion tells us the minimum length of the slice
			rename(instr.X, instr, CopyInfoUnspecified, i)
			i++
		case *SliceToArray:
			// A slice to array conversion tells us the minimum length of the slice
			rename(instr.X, instr, CopyInfoUnspecified, i)
			i++
		case *Slice:
			// Slicing tells us about some of the bounds
			off := 0
			if instr.Low == nil && instr.High == nil && instr.Max == nil {
				// If all indices are unspecified, then we can only learn something about instr.X if it might've been
				// nil.
				if !hasInfo(instr.X, CopyInfoNotNil) {
					rename(instr.X, instr, CopyInfoUnspecified, i)
					off++
				}
			} else {
				rename(instr.X, instr, CopyInfoUnspecified, i)
				off++
			}
			// We copy the indices even if we already know they are not negative, because we can associate numeric
			// ranges with them.
			if instr.Low != nil {
				rename(instr.Low, instr, CopyInfoNotNegative, i)
				off++
			}
			if instr.High != nil {
				rename(instr.High, instr, CopyInfoNotNegative, i)
				off++
			}
			if instr.Max != nil {
				rename(instr.Max, instr, CopyInfoNotNegative, i)
				off++
			}
			i += off
		case *StringLookup:
			rename(instr.X, instr, CopyInfoUnspecified, i)
			rename(instr.Index, instr, CopyInfoNotNegative, i)
			i += 2
		case *Recv:
			if !hasInfo(instr.Chan, CopyInfoNotNil) {
				// Receiving from a nil channel never completes
				rename(instr.Chan, instr, CopyInfoNotNil, i)
				i++
			}
		case *Send:
			if !hasInfo(instr.Chan, CopyInfoNotNil) {
				// Sending to a nil channel never completes. Sending to a closed channel panics, but whether a channel
				// is closed isn't local to this function, so we didn't learn anything.
				rename(instr.Chan, instr, CopyInfoNotNil, i)
				i++
			}
		}
	}

	for _, v := range u.dom.children {
		splitOnNewInformation(v, renaming)
	}
}

// rename implements the Cytron et al-based SSI renaming algorithm, a
// preorder traversal of the dominator tree replacing all loads of
// Alloc cells with the value stored to that cell by the dominating
// store instruction.
//
// renaming is a map from *Alloc (keyed by index number) to its
// dominating stored value; newPhis[x] is the set of new φ-nodes to be
// prepended to block x.
func rename(u *BasicBlock, renaming []Value, newPhis BlockMap[[]newPhi], newSigmas BlockMap[[]newSigma]) {
	// Each φ-node becomes the new name for its associated Alloc.
	for _, np := range newPhis[u.Index] {
		phi := np.phi
		alloc := np.alloc
		renaming[alloc.index] = phi
	}

	// Rename loads and stores of allocs.
	for i, instr := range u.Instrs {
		switch instr := instr.(type) {
		case *Alloc:
			if instr.index >= 0 { // store of zero to Alloc cell
				// Replace dominated loads by the zero value.
				renaming[instr.index] = nil
				if debugLifting {
					fmt.Fprintf(os.Stderr, "\tkill alloc %s\n", instr)
				}
				// Delete the Alloc.
				u.Instrs[i] = nil
				u.gaps++
			}

		case *Store:
			if alloc, ok := instr.Addr.(*Alloc); ok && alloc.index >= 0 { // store to Alloc cell
				// Replace dominated loads by the stored value.
				renaming[alloc.index] = instr.Val
				if debugLifting {
					fmt.Fprintf(os.Stderr, "\tkill store %s; new value: %s\n",
						instr, instr.Val.Name())
				}
				if refs := instr.Addr.Referrers(); refs != nil {
					*refs = removeInstr(*refs, instr)
				}
				if refs := instr.Val.Referrers(); refs != nil {
					*refs = removeInstr(*refs, instr)
				}
				// Delete the Store.
				u.Instrs[i] = nil
				u.gaps++
			}

		case *Load:
			if alloc, ok := instr.X.(*Alloc); ok && alloc.index >= 0 { // load of Alloc cell
				// In theory, we wouldn't be able to replace loads directly, because a loaded value could be used in
				// different branches, in which case it should be replaced with different sigma nodes. But we can't
				// simply defer replacement, either, because then later stores might incorrectly affect this load.
				//
				// To avoid doing renaming on _all_ values (instead of just loads and stores like we're doing), we make
				// sure during code generation that each load is only used in one block. For example, in constant switch
				// statements, where the tag is only evaluated once, we store it in a temporary and load it for each
				// comparison, so that we have individual loads to replace.
				//
				// Because we only rename stores and loads, the end result will not contain sigma nodes for all
				// constants. Some constants may be used directly, e.g. in comparisons such as 'x == 5'. We may still
				// end up inserting dead sigma nodes in branches, but these will never get used in renaming and will be
				// cleaned up when we remove dead phis and sigmas.
				newval := renamed(u.Parent(), renaming, alloc)
				if debugLifting {
					fmt.Fprintf(os.Stderr, "\tupdate load %s = %s with %s\n",
						instr.Name(), instr, newval)
				}
				replaceAll(instr, newval)
				u.Instrs[i] = nil
				u.gaps++
			}

		case *DebugRef:
			if x, ok := instr.X.(*Alloc); ok && x.index >= 0 {
				if instr.IsAddr {
					instr.X = renamed(u.Parent(), renaming, x)
					instr.IsAddr = false

					// Add DebugRef to instr.X's referrers.
					if refs := instr.X.Referrers(); refs != nil {
						*refs = append(*refs, instr)
					}
				} else {
					// A source expression denotes the address
					// of an Alloc that was optimized away.
					instr.X = nil

					// Delete the DebugRef.
					u.Instrs[i] = nil
					u.gaps++
				}
			}
		}
	}

	// update all outgoing sigma nodes with the dominating store
	for _, sigmas := range newSigmas[u.Index] {
		for _, sigma := range sigmas.sigmas {
			if sigma == nil {
				continue
			}
			sigma.X = renamed(u.Parent(), renaming, sigmas.alloc)
		}
	}

	// For each φ-node in a CFG successor, rename the edge.
	for succi, v := range u.Succs {
		phis := newPhis[v.Index]
		if len(phis) == 0 {
			continue
		}
		i := v.predIndex(u)
		for _, np := range phis {
			phi := np.phi
			alloc := np.alloc
			// if there's a sigma node, use it, else use the dominating value
			var newval Value
			for _, sigmas := range newSigmas[u.Index] {
				if sigmas.alloc == alloc && sigmas.sigmas[succi] != nil {
					newval = sigmas.sigmas[succi]
					break
				}
			}
			if newval == nil {
				newval = renamed(u.Parent(), renaming, alloc)
			}
			if debugLifting {
				fmt.Fprintf(os.Stderr, "\tsetphi %s edge %s -> %s (#%d) (alloc=%s) := %s\n",
					phi.Name(), u, v, i, alloc.Name(), newval.Name())
			}
			phi.Edges[i] = newval
			if prefs := newval.Referrers(); prefs != nil {
				*prefs = append(*prefs, phi)
			}
		}
	}

	// Continue depth-first recursion over domtree, pushing a
	// fresh copy of the renaming map for each subtree.
	r := make([]Value, len(renaming))
	for _, v := range u.dom.children {
		copy(r, renaming)

		// on entry to a block, the incoming sigma nodes become the new values for their alloc
		if idx := u.succIndex(v); idx != -1 {
			for _, sigma := range newSigmas[u.Index] {
				if sigma.sigmas[idx] != nil {
					r[sigma.alloc.index] = sigma.sigmas[idx]
				}
			}
		}
		rename(v, r, newPhis, newSigmas)
	}

}

func simplifyConstantCompositeValues(fn *Function) bool {
	changed := false

	for _, b := range fn.Blocks {
		n := 0
		for _, instr := range b.Instrs {
			replaced := false

			if cv, ok := instr.(*CompositeValue); ok {
				ac := &AggregateConst{}
				ac.typ = cv.typ
				replaced = true
				for _, v := range cv.Values {
					if c, ok := v.(Constant); ok {
						ac.Values = append(ac.Values, c)
					} else {
						replaced = false
						break
					}
				}
				if replaced {
					replaceAll(cv, emitConst(fn, ac))
					killInstruction(cv)
				}

			}

			if replaced {
				changed = true
			} else {
				b.Instrs[n] = instr
				n++
			}
		}

		clearInstrs(b.Instrs[n:])
		b.Instrs = b.Instrs[:n]
	}

	return changed
}

func updateOperandReferrers(instr Instruction) {
	for _, op := range instr.Operands(nil) {
		refs := (*op).Referrers()
		if refs != nil {
			*refs = append(*refs, instr)
		}
	}
}

// deferstackPreamble returns the *Alloc and ssa:deferstack() call for fn.deferstack.
func deferstackPreamble(fn *Function) (*Alloc, *Call) {
	if alloc, _ := fn.vars[fn.deferstack].(*Alloc); alloc != nil {
		for _, ref := range *alloc.Referrers() {
			if ref, _ := ref.(*Store); ref != nil && ref.Addr == alloc {
				if call, _ := ref.Val.(*Call); call != nil {
					return alloc, call
				}
			}
		}
	}
	return nil, nil
}
