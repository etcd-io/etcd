// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file defines the Const SSA value type.

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/exp/typeparams"
	"honnef.co/go/tools/go/types/typeutil"
)

// NewConst returns a new constant of the specified value and type.
// val must be valid according to the specification of Const.Value.
func NewConst(val constant.Value, typ types.Type, source ast.Node) *Const {
	c := &Const{
		register: register{
			typ: typ,
		},
		Value: val,
	}
	c.setSource(source)
	return c
}

// intConst returns an 'int' constant that evaluates to i.
// (i is an int64 in case the host is narrower than the target.)
func intConst(i int64, source ast.Node) *Const {
	return NewConst(constant.MakeInt64(i), tInt, source)
}

// nilConst returns a nil constant of the specified type, which may
// be any reference type, including interfaces.
func nilConst(typ types.Type, source ast.Node) *Const {
	return NewConst(nil, typ, source)
}

// stringConst returns a 'string' constant that evaluates to s.
func stringConst(s string, source ast.Node) *Const {
	return NewConst(constant.MakeString(s), tString, source)
}

// zeroConst returns a new "zero" constant of the specified type.
func zeroConst(t types.Type, source ast.Node) Constant {
	if _, ok := t.Underlying().(*types.Interface); ok && !typeparams.IsTypeParam(t) {
		// Handle non-generic interface early to simplify following code.
		return nilConst(t, source)
	}

	tset := typeutil.NewTypeSet(t)

	switch typ := tset.CoreType().(type) {
	case *types.Struct:
		values := make([]Value, typ.NumFields())
		for i := 0; i < typ.NumFields(); i++ {
			values[i] = zeroConst(typ.Field(i).Type(), source)
		}
		ac := &AggregateConst{
			register: register{typ: t},
			Values:   values,
		}
		ac.setSource(source)
		return ac
	case *types.Tuple:
		values := make([]Value, typ.Len())
		for i := 0; i < typ.Len(); i++ {
			values[i] = zeroConst(typ.At(i).Type(), source)
		}
		ac := &AggregateConst{
			register: register{typ: t},
			Values:   values,
		}
		ac.setSource(source)
		return ac
	}

	isNillable := func(term *types.Term) bool {
		switch typ := term.Type().Underlying().(type) {
		case *types.Pointer, *types.Slice, *types.Interface, *types.Chan, *types.Map, *types.Signature, *typeutil.Iterator:
			return true
		case *types.Basic:
			switch typ.Kind() {
			case types.UnsafePointer, types.UntypedNil:
				return true
			default:
				return false
			}
		default:
			return false
		}
	}

	isInfo := func(info types.BasicInfo) func(*types.Term) bool {
		return func(term *types.Term) bool {
			basic, ok := term.Type().Underlying().(*types.Basic)
			if !ok {
				return false
			}
			return (basic.Info() & info) != 0
		}
	}

	isArray := func(term *types.Term) bool {
		_, ok := term.Type().Underlying().(*types.Array)
		return ok
	}

	switch {
	case tset.All(isInfo(types.IsNumeric)):
		return NewConst(constant.MakeInt64(0), t, source)
	case tset.All(isInfo(types.IsString)):
		return NewConst(constant.MakeString(""), t, source)
	case tset.All(isInfo(types.IsBoolean)):
		return NewConst(constant.MakeBool(false), t, source)
	case tset.All(isNillable):
		return nilConst(t, source)
	case tset.All(isArray):
		var k ArrayConst
		k.setType(t)
		k.setSource(source)
		return &k
	default:
		var k GenericConst
		k.setType(t)
		k.setSource(source)
		return &k
	}
}

func (c *Const) RelString(from *types.Package) string {
	var p string
	if c.Value == nil {
		p = "nil"
	} else if c.Value.Kind() == constant.String {
		v := constant.StringVal(c.Value)
		const max = 20
		// TODO(adonovan): don't cut a rune in half.
		if len(v) > max {
			v = v[:max-3] + "..." // abbreviate
		}
		p = strconv.Quote(v)
	} else {
		p = c.Value.String()
	}
	return fmt.Sprintf("Const <%s> {%s}", relType(c.Type(), from), p)
}

func (c *Const) String() string {
	if c.block == nil {
		// Constants don't have a block till late in the compilation process. But we want to print consts during
		// debugging.
		return c.RelString(nil)
	}
	return c.RelString(c.Parent().pkg())
}

func (v *ArrayConst) RelString(pkg *types.Package) string {
	return fmt.Sprintf("ArrayConst <%s>", relType(v.Type(), pkg))
}

func (v *ArrayConst) String() string {
	return v.RelString(v.Parent().pkg())
}

func (v *AggregateConst) RelString(pkg *types.Package) string {
	values := make([]string, len(v.Values))
	for i, v := range v.Values {
		if v != nil {
			values[i] = v.Name()
		} else {
			values[i] = "nil"
		}
	}
	return fmt.Sprintf("AggregateConst <%s> (%s)", relType(v.Type(), pkg), strings.Join(values, ", "))
}

func (v *AggregateConst) String() string {
	if v.block == nil {
		return v.RelString(nil)
	}
	return v.RelString(v.Parent().pkg())
}

func (v *GenericConst) RelString(pkg *types.Package) string {
	return fmt.Sprintf("GenericConst <%s>", relType(v.Type(), pkg))
}

func (v *GenericConst) String() string {
	return v.RelString(v.Parent().pkg())
}

// IsNil returns true if this constant represents a typed or untyped nil value.
func (c *Const) IsNil() bool {
	return c.Value == nil
}

// Int64 returns the numeric value of this constant truncated to fit
// a signed 64-bit integer.
func (c *Const) Int64() int64 {
	switch x := constant.ToInt(c.Value); x.Kind() {
	case constant.Int:
		if i, ok := constant.Int64Val(x); ok {
			return i
		}
		return 0
	case constant.Float:
		f, _ := constant.Float64Val(x)
		return int64(f)
	}
	panic(fmt.Sprintf("unexpected constant value: %T", c.Value))
}

// Uint64 returns the numeric value of this constant truncated to fit
// an unsigned 64-bit integer.
func (c *Const) Uint64() uint64 {
	switch x := constant.ToInt(c.Value); x.Kind() {
	case constant.Int:
		if u, ok := constant.Uint64Val(x); ok {
			return u
		}
		return 0
	case constant.Float:
		f, _ := constant.Float64Val(x)
		return uint64(f)
	}
	panic(fmt.Sprintf("unexpected constant value: %T", c.Value))
}

// Float64 returns the numeric value of this constant truncated to fit
// a float64.
func (c *Const) Float64() float64 {
	f, _ := constant.Float64Val(c.Value)
	return f
}

// Complex128 returns the complex value of this constant truncated to
// fit a complex128.
func (c *Const) Complex128() complex128 {
	re, _ := constant.Float64Val(constant.Real(c.Value))
	im, _ := constant.Float64Val(constant.Imag(c.Value))
	return complex(re, im)
}

func (c *Const) equal(o Constant) bool {
	// TODO(dh): don't use == for types, this will miss identical pointer types, among others
	oc, ok := o.(*Const)
	if !ok {
		return false
	}
	return c.typ == oc.typ && c.Value == oc.Value && c.source == oc.source
}

func (c *AggregateConst) equal(o Constant) bool {
	oc, ok := o.(*AggregateConst)
	if !ok {
		return false
	}
	// TODO(dh): don't use == for types, this will miss identical pointer types, among others
	if c.typ != oc.typ {
		return false
	}
	if c.source != oc.source {
		return false
	}
	for i, v := range c.Values {
		if !v.(Constant).equal(oc.Values[i].(Constant)) {
			return false
		}
	}
	return true
}

func (c *ArrayConst) equal(o Constant) bool {
	oc, ok := o.(*ArrayConst)
	if !ok {
		return false
	}
	// TODO(dh): don't use == for types, this will miss identical pointer types, among others
	return c.typ == oc.typ && c.source == oc.source
}

func (c *GenericConst) equal(o Constant) bool {
	oc, ok := o.(*GenericConst)
	if !ok {
		return false
	}
	// TODO(dh): don't use == for types, this will miss identical pointer types, among others
	return c.typ == oc.typ && c.source == oc.source
}
