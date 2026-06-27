// This file was copy from https://cs.opensource.google/go/x/tools/+/master:go/analysis/passes/nilness/nilness.go
// I modified some to check the error return

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nilnesserr

import (
	"go/token"
	"go/types"

	"github.com/alingse/nilnesserr/internal/typeparams"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

func (a *analyzer) checkNilnesserr(pass *analysis.Pass) (interface{}, error) {
	ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range ssainput.SrcFuncs {
		runFunc(pass, fn)
	}
	return nil, nil
}

func runFunc(pass *analysis.Pass, fn *ssa.Function) {
	// visit visits reachable blocks of the CFG in dominance order,
	// maintaining a stack of dominating nilness facts.
	//
	// By traversing the dom tree, we can pop facts off the stack as
	// soon as we've visited a subtree.  Had we traversed the CFG,
	// we would need to retain the set of facts for each block.
	seen := make([]bool, len(fn.Blocks)) // seen[i] means visit should ignore block i

	var visit func(b *ssa.BasicBlock, stack []fact, errors []errFact)

	visit = func(b *ssa.BasicBlock, stack []fact, errors []errFact) {
		if seen[b.Index] {
			return
		}
		seen[b.Index] = true

		// check this block return a nil value error
		checkNilnesserr(
			pass, b,
			errors,
			func(v ssa.Value) bool {
				return nilnessOf(stack, v) == isnil
			})

		// For nil comparison blocks, report an error if the condition
		// is degenerate, and push a nilness fact on the stack when
		// visiting its true and false successor blocks.
		if binop, tsucc, fsucc := eq(b); binop != nil {
			// extract the err != nil or err == nil
			errValue := extractCheckedErrorValue(binop)

			xnil := nilnessOf(stack, binop.X)
			ynil := nilnessOf(stack, binop.Y)

			if ynil != unknown && xnil != unknown && (xnil == isnil || ynil == isnil) {
				// Degenerate condition:
				// the nilness of both operands is known,
				// and at least one of them is nil.

				// If tsucc's or fsucc's sole incoming edge is impossible,
				// it is unreachable.  Prune traversal of it and
				// all the blocks it dominates.
				// (We could be more precise with full dataflow
				// analysis of control-flow joins.)
				var skip *ssa.BasicBlock
				if xnil == ynil {
					skip = fsucc
				} else {
					skip = tsucc
				}
				for _, d := range b.Dominees() {
					if d == skip && len(d.Preds) == 1 {
						continue
					}

					visit(d, stack, errors)
				}

				return
			}

			// "if x == nil" or "if nil == y" condition; x, y are unknown.
			if xnil == isnil || ynil == isnil {
				var newFacts facts
				if xnil == isnil {
					// x is nil, y is unknown:
					// t successor learns y is nil.
					newFacts = expandFacts(fact{binop.Y, isnil})
				} else {
					// y is nil, x is unknown:
					// t successor learns x is nil.
					newFacts = expandFacts(fact{binop.X, isnil})
				}

				for _, d := range b.Dominees() {
					// Successor blocks learn a fact
					// only at non-critical edges.
					// (We could do be more precise with full dataflow
					// analysis of control-flow joins.)
					s := stack
					errs := errors
					if len(d.Preds) == 1 {
						if d == tsucc {
							s = append(s, newFacts...)
							// add nil error
							if errValue != nil {
								errs = append(errs, errFact{value: errValue, nilness: isnil})
							}
						} else if d == fsucc {
							s = append(s, newFacts.negate()...)
							// add non-nil error
							if errValue != nil {
								errs = append(errs, errFact{value: errValue, nilness: isnonnil})
							}
						}
					}

					visit(d, s, errs)
				}
				return
			}
		}

		// In code of the form:
		//
		// 	if ptr, ok := x.(*T); ok { ... } else { fsucc }
		//
		// the fsucc block learns that ptr == nil,
		// since that's its zero value.
		if If, ok := b.Instrs[len(b.Instrs)-1].(*ssa.If); ok {
			// Handle "if ok" and "if !ok" variants.
			cond, fsucc := If.Cond, b.Succs[1]
			if unop, ok := cond.(*ssa.UnOp); ok && unop.Op == token.NOT {
				cond, fsucc = unop.X, b.Succs[0]
			}

			// Match pattern:
			//   t0 = typeassert (pointerlike)
			//   t1 = extract t0 #0  // ptr
			//   t2 = extract t0 #1  // ok
			//   if t2 goto tsucc, fsucc
			if extract1, ok := cond.(*ssa.Extract); ok && extract1.Index == 1 {
				if assert, ok := extract1.Tuple.(*ssa.TypeAssert); ok &&
					isNillable(assert.AssertedType) {
					for _, pinstr := range *assert.Referrers() {
						if extract0, ok := pinstr.(*ssa.Extract); ok &&
							extract0.Index == 0 &&
							extract0.Tuple == extract1.Tuple {
							for _, d := range b.Dominees() {
								if len(d.Preds) == 1 && d == fsucc {
									visit(d, append(stack, fact{extract0, isnil}), errors)
								}
							}
						}
					}
				}
			}
		}

		for _, d := range b.Dominees() {
			visit(d, stack, errors)
		}
	}

	// Visit the entry block.  No need to visit fn.Recover.
	if fn.Blocks != nil {
		visit(fn.Blocks[0], make([]fact, 0, 20), nil) // 20 is plenty
	}
}

// A fact records that a block is dominated
// by the condition v == nil or v != nil.
type fact struct {
	value   ssa.Value
	nilness nilness
}

func (f fact) negate() fact { return fact{f.value, -f.nilness} }

type nilness int

const (
	isnonnil         = -1
	unknown  nilness = 0
	isnil            = 1
)

var nilnessStrings = []string{"non-nil", "unknown", "nil"}

func (n nilness) String() string { return nilnessStrings[n+1] }

// nilnessOf reports whether v is definitely nil, definitely not nil,
// or unknown given the dominating stack of facts.
func nilnessOf(stack []fact, v ssa.Value) nilness {
	switch v := v.(type) {
	// unwrap ChangeInterface and Slice values recursively, to detect if underlying
	// values have any facts recorded or are otherwise known with regard to nilness.
	//
	// This work must be in addition to expanding facts about
	// ChangeInterfaces during inference/fact gathering because this covers
	// cases where the nilness of a value is intrinsic, rather than based
	// on inferred facts, such as a zero value interface variable. That
	// said, this work alone would only inform us when facts are about
	// underlying values, rather than outer values, when the analysis is
	// transitive in both directions.
	case *ssa.ChangeInterface:
		if underlying := nilnessOf(stack, v.X); underlying != unknown {
			return underlying
		}
	case *ssa.MakeInterface:
		// A MakeInterface is non-nil unless its operand is a type parameter.
		tparam, ok := types.Unalias(v.X.Type()).(*types.TypeParam)
		if !ok {
			return isnonnil
		}

		// A MakeInterface of a type parameter is non-nil if
		// the type parameter cannot be instantiated as an
		// interface type (#66835).
		if terms, err := typeparams.NormalTerms(tparam.Constraint()); err == nil && len(terms) > 0 {
			return isnonnil
		}

		// If the type parameter can be instantiated as an
		// interface (and thus also as a concrete type),
		// we can't determine the nilness.

	case *ssa.Slice:
		if underlying := nilnessOf(stack, v.X); underlying != unknown {
			return underlying
		}
	case *ssa.SliceToArrayPointer:
		nn := nilnessOf(stack, v.X)
		if slice2ArrayPtrLen(v) > 0 {
			if nn == isnil {
				// We know that *(*[1]byte)(nil) is going to panic because of the
				// conversion. So return unknown to the caller, prevent useless
				// nil deference reporting due to * operator.
				return unknown
			}
			// Otherwise, the conversion will yield a non-nil pointer to array.
			// Note that the instruction can still panic if array length greater
			// than slice length. If the value is used by another instruction,
			// that instruction can assume the panic did not happen when that
			// instruction is reached.
			return isnonnil
		}
		// In case array length is zero, the conversion result depends on nilness of the slice.
		if nn != unknown {
			return nn
		}
	}

	// Is value intrinsically nil or non-nil?
	switch v := v.(type) {
	case *ssa.Alloc,
		*ssa.FieldAddr,
		*ssa.FreeVar,
		*ssa.Function,
		*ssa.Global,
		*ssa.IndexAddr,
		*ssa.MakeChan,
		*ssa.MakeClosure,
		*ssa.MakeMap,
		*ssa.MakeSlice:
		return isnonnil

	case *ssa.Const:
		if v.IsNil() {
			return isnil // nil or zero value of a pointer-like type
		} else {
			return unknown // non-pointer
		}
	}

	// Search dominating control-flow facts.
	for _, f := range stack {
		if f.value == v {
			return f.nilness
		}
	}
	return unknown
}

func slice2ArrayPtrLen(v *ssa.SliceToArrayPointer) int64 {
	return v.Type().(*types.Pointer).Elem().Underlying().(*types.Array).Len()
}

// If b ends with an equality comparison, eq returns the operation and
// its true (equal) and false (not equal) successors.
func eq(b *ssa.BasicBlock) (op *ssa.BinOp, tsucc, fsucc *ssa.BasicBlock) {
	if If, ok := b.Instrs[len(b.Instrs)-1].(*ssa.If); ok {
		if binop, ok := If.Cond.(*ssa.BinOp); ok {
			switch binop.Op {
			case token.EQL:
				return binop, b.Succs[0], b.Succs[1]
			case token.NEQ:
				return binop, b.Succs[1], b.Succs[0]
			}
		}
	}
	return nil, nil, nil
}

// expandFacts takes a single fact and returns the set of facts that can be
// known about it or any of its related values. Some operations, like
// ChangeInterface, have transitive nilness, such that if you know the
// underlying value is nil, you also know the value itself is nil, and vice
// versa. This operation allows callers to match on any of the related values
// in analyses, rather than just the one form of the value that happened to
// appear in a comparison.
//
// This work must be in addition to unwrapping values within nilnessOf because
// while this work helps give facts about transitively known values based on
// inferred facts, the recursive check within nilnessOf covers cases where
// nilness facts are intrinsic to the underlying value, such as a zero value
// interface variables.
//
// ChangeInterface is the only expansion currently supported, but others, like
// Slice, could be added. At this time, this tool does not check slice
// operations in a way this expansion could help. See
// https://play.golang.org/p/mGqXEp7w4fR for an example.
func expandFacts(f fact) []fact {
	ff := []fact{f}

Loop:
	for {
		switch v := f.value.(type) {
		case *ssa.ChangeInterface:
			f = fact{v.X, f.nilness}
			ff = append(ff, f)
		default:
			break Loop
		}
	}

	return ff
}

type facts []fact

func (ff facts) negate() facts {
	nn := make([]fact, len(ff))
	for i, f := range ff {
		nn[i] = f.negate()
	}
	return nn
}

func isNillable(t types.Type) bool {
	// TODO(adonovan): CoreType (+ case *Interface) looks wrong.
	// This should probably use Underlying, and handle TypeParam
	// by computing the union across its normal terms.
	switch t := typeparams.CoreType(t).(type) {
	case *types.Pointer,
		*types.Map,
		*types.Signature,
		*types.Chan,
		*types.Interface,
		*types.Slice:
		return true
	case *types.Basic:
		return t == types.Typ[types.UnsafePointer]
	}
	return false
}
