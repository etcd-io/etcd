package typeutil

import (
	"errors"
	"go/types"
	"slices"

	"golang.org/x/exp/typeparams"
)

type TypeSet struct {
	Terms []*types.Term
	empty bool
}

func NewTypeSet(typ types.Type) TypeSet {
	terms, err := typeparams.NormalTerms(typ)
	if err != nil {
		if errors.Is(err, typeparams.ErrEmptyTypeSet) {
			return TypeSet{nil, true}
		} else {
			// We couldn't determine the type set. Assume it's all types.
			return TypeSet{nil, false}
		}
	}
	return TypeSet{terms, false}
}

// CoreType returns the type set's core type, or nil if it has none.
// The function only looks at type terms and may thus return core types for some empty type sets, such as
// 'interface { map[int]string; foo() }'
func (ts TypeSet) CoreType() types.Type {
	if len(ts.Terms) == 0 {
		// Either the type set is empty, or it isn't constrained. Either way it doesn't have a core type.
		return nil
	}
	typ := ts.Terms[0].Type().Underlying()
	for _, term := range ts.Terms[1:] {
		ut := term.Type().Underlying()
		if types.Identical(typ, ut) {
			continue
		}

		ch1, ok := typ.(*types.Chan)
		if !ok {
			return nil
		}
		ch2, ok := ut.(*types.Chan)
		if !ok {
			return nil
		}
		if ch1.Dir() == types.SendRecv {
			// typ is currently a bidirectional channel. The term's type is either also bidirectional, or
			// unidirectional. Use the term's type.
			typ = ut
		} else if ch2.Dir() == types.SendRecv {
			// typ is currently a unidirectional channel and the term's type is bidirectional, which means it has no
			// effect.
			continue
		} else if ch1.Dir() != ch2.Dir() {
			// typ is not bidirectional and typ and term disagree about the direction
			return nil
		}
	}
	return typ
}

// CoreType is a wrapper for NewTypeSet(typ).CoreType()
func CoreType(typ types.Type) types.Type {
	return NewTypeSet(typ).CoreType()
}

// All calls fn for each term in the type set and reports whether all invocations returned true.
// If the type set is empty or unconstrained, All immediately returns false.
func (ts TypeSet) All(fn func(*types.Term) bool) bool {
	if len(ts.Terms) == 0 {
		return false
	}
	for _, term := range ts.Terms {
		if !fn(term) {
			return false
		}
	}
	return true
}

// Any calls fn for each term in the type set and reports whether any invocation returned true.
// It stops after the first call that returned true.
func (ts TypeSet) Any(fn func(*types.Term) bool) bool {
	return slices.ContainsFunc(ts.Terms, fn)
}

// All is a wrapper for NewTypeSet(typ).All(fn).
func All(typ types.Type, fn func(*types.Term) bool) bool {
	return NewTypeSet(typ).All(fn)
}

// Any is a wrapper for NewTypeSet(typ).Any(fn).
func Any(typ types.Type, fn func(*types.Term) bool) bool {
	return NewTypeSet(typ).Any(fn)
}

func IsSlice(term *types.Term) bool {
	_, ok := term.Type().Underlying().(*types.Slice)
	return ok
}
