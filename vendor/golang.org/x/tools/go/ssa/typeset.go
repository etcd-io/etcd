// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"go/types"

	"golang.org/x/tools/internal/typeparams"
)

// Utilities for dealing with type sets.

const debug = false

// typeset is an iterator over the (type/underlying type) pairs of the
// specific type terms of the type set implied by t.
// If t is a type parameter, the implied type set is the type set of t's constraint.
// In that case, if there are no specific terms, typeset calls yield with (nil, nil).
// If t is not a type parameter, the implied type set consists of just t.
// In any case, typeset is guaranteed to call yield at least once.
func typeset(typ types.Type, yield func(t, u types.Type) bool) {
	switch typ := types.Unalias(typ).(type) {
	case *types.TypeParam, *types.Interface:
		terms := termListOf(typ)
		if len(terms) == 0 {
			yield(nil, nil)
			return
		}
		for _, term := range terms {
			u := types.Unalias(term.Type())
			if !term.Tilde() {
				u = u.Underlying()
			}
			if debug {
				assert(types.Identical(u, u.Underlying()), "Unalias(x) == under(x) for ~x terms")
			}
			if !yield(term.Type(), u) {
				break
			}
		}
		return
	default:
		yield(typ, typ.Underlying())
	}
}

// termListOf returns the type set of typ as a normalized term set. Returns an empty set on an error.
func termListOf(typ types.Type) []*types.Term {
	terms, err := typeparams.NormalTerms(typ)
	if err != nil {
		return nil
	}
	return terms
}

// typeSetIsEmpty returns true if a typeset is empty.
func typeSetIsEmpty(typ types.Type) bool {
	var empty bool
	typeset(typ, func(t, _ types.Type) bool {
		empty = t == nil
		return false
	})
	return empty
}

// isBytestring returns true if T has the same terms as interface{[]byte | string}.
// These act like a core type for some operations: slice expressions, append and copy.
//
// See https://go.dev/ref/spec#Core_types for the details on bytestring.
func isBytestring(T types.Type) bool {
	U := T.Underlying()
	if _, ok := U.(*types.Interface); !ok {
		return false
	}

	hasBytes, hasString := false, false
	ok := underIs(U, func(t types.Type) bool {
		switch {
		case isString(t):
			hasString = true
			return true
		case isByteSlice(t):
			hasBytes = true
			return true
		default:
			return false
		}
	})
	return ok && hasBytes && hasString
}

// underIs calls f with the underlying types of the type terms
// of the type set of typ and reports whether all calls to f returned true.
// If there are no specific terms, underIs returns the result of f(nil).
func underIs(typ types.Type, f func(types.Type) bool) bool {
	var ok bool
	typeset(typ, func(t, u types.Type) bool {
		ok = f(u)
		return ok
	})
	return ok
}

// indexType returns the element type and index mode of a IndexExpr over a type.
// It returns an invalid mode if the type is not indexable; this should never occur in a well-typed program.
func indexType(typ types.Type) (types.Type, indexMode) {
	switch U := typ.Underlying().(type) {
	case *types.Array:
		return U.Elem(), ixArrVar
	case *types.Pointer:
		if arr, ok := U.Elem().Underlying().(*types.Array); ok {
			return arr.Elem(), ixVar
		}
	case *types.Slice:
		return U.Elem(), ixVar
	case *types.Map:
		return U.Elem(), ixMap
	case *types.Basic:
		return tByte, ixValue // must be a string
	case *types.Interface:
		var elem types.Type
		mode := ixInvalid
		typeset(typ, func(t, _ types.Type) bool {
			if t == nil {
				return false // empty set
			}
			e, m := indexType(t)
			if elem == nil {
				elem, mode = e, m
			}
			if debug && !types.Identical(elem, e) { // if type checked, just a sanity check
				mode = ixInvalid
				return false
			}
			// Update the mode to the most constrained address type.
			mode = mode.meet(m)
			return mode != ixInvalid
		})
		return elem, mode
	}
	return nil, ixInvalid
}

// An indexMode specifies the (addressing) mode of an index operand.
//
// Addressing mode of an index operation is based on the set of
// underlying types.
// Hasse diagram of the indexMode meet semi-lattice:
//
//	ixVar     ixMap
//	  |          |
//	ixArrVar     |
//	  |          |
//	ixValue      |
//	   \        /
//	  ixInvalid
type indexMode byte

const (
	ixInvalid indexMode = iota // index is invalid
	ixValue                    // index is a computed value (not addressable)
	ixArrVar                   // like ixVar, but index operand contains an array
	ixVar                      // index is an addressable variable
	ixMap                      // index is a map index expression (acts like a variable on lhs, commaok on rhs of an assignment)
)

// meet is the address type that is constrained by both x and y.
func (x indexMode) meet(y indexMode) indexMode {
	if (x == ixMap || y == ixMap) && x != y {
		return ixInvalid
	}
	// Use int representation and return min.
	if x < y {
		return y
	}
	return x
}
