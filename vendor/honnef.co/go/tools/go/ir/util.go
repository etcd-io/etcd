// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file defines a number of miscellaneous utility functions.

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"os"

	"honnef.co/go/tools/go/ast/astutil"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/exp/typeparams"
)

//// AST utilities

func unparen(e ast.Expr) ast.Expr { return astutil.Unparen(e) }

// isBlankIdent returns true iff e is an Ident with name "_".
// They have no associated types.Object, and thus no type.
func isBlankIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}

//// Type utilities.  Some of these belong in go/types.

// isPointer returns true for types whose underlying type is a pointer,
// and for type parameters whose core type is a pointer.
func isPointer(typ types.Type) bool {
	if ctyp := typeutil.CoreType(typ); ctyp != nil {
		_, ok := ctyp.(*types.Pointer)
		return ok
	}
	_, ok := typ.Underlying().(*types.Pointer)
	return ok
}

// deref returns a pointer's element type; otherwise it returns typ.
func deref(typ types.Type) types.Type {
	orig := typ
	typ = types.Unalias(typ)

	if t, ok := typ.(*types.TypeParam); ok {
		if ctyp := typeutil.CoreType(t); ctyp != nil {
			// This can happen, for example, with len(T) where T is a
			// type parameter whose core type is a pointer to array.
			typ = ctyp
		}
	}
	if p, ok := typ.Underlying().(*types.Pointer); ok {
		return p.Elem()
	}
	return orig
}

// recvType returns the receiver type of method obj.
func recvType(obj *types.Func) types.Type {
	return obj.Type().(*types.Signature).Recv().Type()
}

// logStack prints the formatted "start" message to stderr and
// returns a closure that prints the corresponding "end" message.
// Call using 'defer logStack(...)()' to show builder stack on panic.
// Don't forget trailing parens!
func logStack(format string, args ...any) func() {
	msg := fmt.Sprintf(format, args...)
	io.WriteString(os.Stderr, msg)
	io.WriteString(os.Stderr, "\n")
	return func() {
		io.WriteString(os.Stderr, msg)
		io.WriteString(os.Stderr, " end\n")
	}
}

// newVar creates a 'var' for use in a types.Tuple.
func newVar(name string, typ types.Type) *types.Var {
	return types.NewParam(token.NoPos, nil, name, typ)
}

// anonVar creates an anonymous 'var' for use in a types.Tuple.
func anonVar(typ types.Type) *types.Var {
	return newVar("", typ)
}

var lenResults = types.NewTuple(anonVar(tInt))

// makeLen returns the len builtin specialized to type func(T)int.
func makeLen(T types.Type) *Builtin {
	lenParams := types.NewTuple(anonVar(T))
	return &Builtin{
		name: "len",
		sig:  types.NewSignatureType(nil, nil, nil, lenParams, lenResults, false),
	}
}

type StackMap struct {
	m []map[Value]Value
}

func (m *StackMap) Push() {
	m.m = append(m.m, map[Value]Value{})
}

func (m *StackMap) Pop() {
	m.m = m.m[:len(m.m)-1]
}

func (m *StackMap) Get(key Value) (Value, bool) {
	for i := len(m.m) - 1; i >= 0; i-- {
		if v, ok := m.m[i][key]; ok {
			return v, true
		}
	}
	return nil, false
}

func (m *StackMap) Set(k Value, v Value) {
	m.m[len(m.m)-1][k] = v
}

// Unwrap recursively unwraps Sigma and Copy nodes.
func Unwrap(v Value) Value {
	for {
		switch vv := v.(type) {
		case *Sigma:
			v = vv.X
		case *Copy:
			v = vv.X
		default:
			return v
		}
	}
}

func assert(x bool) {
	if !x {
		panic("failed assertion")
	}
}

// BlockMap is a mapping from basic blocks (identified by their indices) to values.
type BlockMap[T any] []T

// isBasic reports whether t is a basic type.
func isBasic(t types.Type) bool {
	_, ok := t.(*types.Basic)
	return ok
}

// isNonTypeParamInterface reports whether t is an interface type but not a type parameter.
func isNonTypeParamInterface(t types.Type) bool {
	return !typeparams.IsTypeParam(t) && types.IsInterface(t)
}
