// Copyright 2015-2018 Auburn University and others. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dataflow

import (
	"go/ast"
	"go/types"

	"github.com/godoctor/godoctor/analysis/cfg"
	"github.com/willf/bitset"
	"golang.org/x/tools/go/loader"
)

// File defines live variables analysis for a statement
// level control flow graph. Defer has quirks, see LiveVars func.
//
// based on algo from ch 9.2, p.610 Dragonbook, v2.2,
// "Iterative algorithm to compute live variables":
//
// IN[EXIT] = use[each D in Defers];
// for(each basic block B other than EXIT) IN[B} = {};
// for(changes to any IN occur)
//    for(each basic block B other than EXIT) {
//      OUT[B] = Union(S a successor of B) IN[S];
//      IN[B] = use[b] Union (OUT[B] - def[b]);
//    }

// NOTE: for extract function: defers in the block to extract can
// (probably?) be extracted if all variables used in the defer statement are
// not live at the beginning and the end of the block to extract

// LiveAt returns the in and out set of live variables for each block in
// a given control flow graph (cfg) in the context of a loader.Program,
// including the cfg.Entry and cfg.Exit nodes.
//
// The traditional approach of holding the live variables at the exit node
// to the empty set has been deviated from in order to handle defers.
// The live variables in set of the cfg.Exit node will be set to the variables used
// in all cfg.Defers. No liveness is analyzed for the cfg.Defers themselves.
//
// More formally:
//  IN[EXIT] = USE(each d in cfg.Defers)
//  OUT[EXIT] = {}
func LiveVars(cfg *cfg.CFG, info *loader.PackageInfo) (in, out map[ast.Stmt]map[*types.Var]struct{}) {
	vars, def, use := defUseBitsets(cfg, info)
	ins, outs := liveVarsBitsets(cfg, def, use)
	return liveVarsResultSets(cfg, vars, ins, outs)
}

// defUseBitsets builds def and use bitsets for the given cfg in the context
// of the given loader.PackageInfo. Each entry in the resulting bitsets maps back to
// the same index in the returned vars slice.
func defUseBitsets(cfg *cfg.CFG, info *loader.PackageInfo) (vars []*types.Var, def, use map[ast.Stmt]*bitset.BitSet) {
	blocks := cfg.Blocks()

	def = make(map[ast.Stmt]*bitset.BitSet, len(blocks))
	use = make(map[ast.Stmt]*bitset.BitSet, len(blocks))
	varIndices := make(map[*types.Var]uint) // map var to its index in vars

	for _, block := range blocks {
		// prime the def-uses sets
		def[block] = new(bitset.BitSet)
		use[block] = new(bitset.BitSet)

		d := defs(block, info)
		u := uses(block, info)

		// use[Exit] = use(each d in cfg.Defers)
		if block == cfg.Exit {
			for _, dfr := range cfg.Defers {
				u = append(u, uses(dfr, info)...)
			}
		}

		for _, d := range d {
			// if we have it already, uses that index
			// if we don't, add it to our slice and save its index
			k, ok := varIndices[d]
			if !ok {
				k = uint(len(vars))
				varIndices[d] = k
				vars = append(vars, d)
			}

			def[block].Set(k)
		}

		for _, u := range u {
			k, ok := varIndices[u]
			if !ok {
				k = uint(len(vars))
				varIndices[u] = k
				vars = append(vars, u)
			}

			use[block].Set(k)
		}
	}
	return vars, def, use
}

// liveVarsBitsets generatates live variable analysis in and out bitsets from def and use sets
func liveVarsBitsets(cfg *cfg.CFG, def, use map[ast.Stmt]*bitset.BitSet) (in, out map[ast.Stmt]*bitset.BitSet) {
	blocks := cfg.Blocks()
	in = make(map[ast.Stmt]*bitset.BitSet, len(blocks))
	out = make(map[ast.Stmt]*bitset.BitSet, len(blocks))
	// for(each basic block B) IN[B} = {};
	for _, block := range blocks {
		in[block] = new(bitset.BitSet)
		out[block] = new(bitset.BitSet)
	}

	// for(changes to any IN occur)
	for {
		var change bool

		// for(each basic block B) {
		for _, block := range blocks {

			// OUT[B] = Union(S a succ of B) IN[S]
			for _, s := range cfg.Succs(block) {
				out[block].InPlaceUnion(in[s])
			}

			old := in[block].Clone()

			// IN[B] = uses[B] U (OUT[B] - def[B])
			in[block] = use[block].Union(out[block].Difference(def[block]))

			change = change || !old.Equal(in[block])
		}

		if !change {
			break
		}
	}
	return in, out
}

// liveVarsResultsSets maps in and out bitsets back to their respective vars, such that
// each statement has a set of variables that are live upon entry and a set of
// variables that are live upon exit.
func liveVarsResultSets(cfg *cfg.CFG, vars []*types.Var, ins, outs map[ast.Stmt]*bitset.BitSet) (in, out map[ast.Stmt]map[*types.Var]struct{}) {
	blocks := cfg.Blocks()
	in = make(map[ast.Stmt]map[*types.Var]struct{}, len(blocks))
	out = make(map[ast.Stmt]map[*types.Var]struct{}, len(blocks))

	for _, block := range blocks {
		in[block] = make(map[*types.Var]struct{})
		out[block] = make(map[*types.Var]struct{})

		for i := uint(0); i < ins[block].Len(); i++ {
			if ins[block].Test(i) {
				in[block][vars[i]] = struct{}{}
			}
		}

		for i := uint(0); i < outs[block].Len(); i++ {
			if outs[block].Test(i) {
				out[block][vars[i]] = struct{}{}
			}
		}
	}
	return in, out
}
