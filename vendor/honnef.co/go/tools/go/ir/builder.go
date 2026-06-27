// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file implements the BUILD phase of IR construction.
//
// IR construction has two phases, CREATE and BUILD.  In the CREATE phase
// (create.go), all packages are constructed and type-checked and
// definitions of all package members are created, method-sets are
// computed, and wrapper methods are synthesized.
// ir.Packages are created in arbitrary order.
//
// In the BUILD phase (builder.go), the builder traverses the AST of
// each Go source function and generates IR instructions for the
// function body.  Initializer expressions for package-level variables
// are emitted to the package's init() function in the order specified
// by go/types.Info.InitOrder, then code for each function in the
// package is generated in lexical order.
//
// The builder's and Program's indices (maps) are populated and
// mutated during the CREATE phase, but during the BUILD phase they
// remain constant.  The sole exception is Prog.methodSets and its
// related maps, which are protected by a dedicated mutex.

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"go/version"
	"os"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/exp/typeparams"
)

var (
	varOk    = newVar("ok", tBool)
	varIndex = newVar("index", tInt)

	// Type constants.
	tBool       = types.Typ[types.Bool]
	tInt        = types.Typ[types.Int]
	tInvalid    = types.Typ[types.Invalid]
	tString     = types.Typ[types.String]
	tUntypedNil = types.Typ[types.UntypedNil]
	tEface      = types.NewInterfaceType(nil, nil).Complete()
	tDeferStack = types.NewPointer(typeutil.NewDeferStack())

	vDeferStack = &Builtin{
		name: "ssa:deferstack",
		sig:  types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(anonVar(tDeferStack)), false),
	}
)

// range-over-func jump is READY
func jReady() *Const {
	c := intConst(0, nil)
	c.comment = "rangefunc.exit.ready"
	return c
}

// range-over-func jump is BUSY
func jBusy() *Const {
	c := intConst(-1, nil)
	c.comment = "rangefunc.exit.busy"
	return c
}

// range-over-func jump is DONE
func jDone() *Const {
	c := intConst(-2, nil)
	c.comment = "rangefunc.exit.done"
	return c
}

// builder holds state associated with the package currently being built.
// Its methods contain all the logic for AST-to-IR conversion.
type builder struct {
	printFunc string

	blocksets [5]BlockSet
}

// cond emits to fn code to evaluate boolean condition e and jump
// to t or f depending on its value, performing various simplifications.
//
// Postcondition: fn.currentBlock is nil.
func (b *builder) cond(fn *Function, e ast.Expr, t, f *BasicBlock) *If {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return b.cond(fn, e.X, t, f)

	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND:
			ltrue := fn.newBasicBlock("cond.true")
			b.cond(fn, e.X, ltrue, f)
			fn.currentBlock = ltrue
			return b.cond(fn, e.Y, t, f)

		case token.LOR:
			lfalse := fn.newBasicBlock("cond.false")
			b.cond(fn, e.X, t, lfalse)
			fn.currentBlock = lfalse
			return b.cond(fn, e.Y, t, f)
		}

	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			return b.cond(fn, e.X, f, t)
		}
	}

	// A traditional compiler would simplify "if false" (etc) here
	// but we do not, for better fidelity to the source code.
	//
	// The value of a constant condition may be platform-specific,
	// and may cause blocks that are reachable in some configuration
	// to be hidden from subsequent analyses such as bug-finding tools.
	return emitIf(fn, b.expr(fn, e), t, f, e)
}

// logicalBinop emits code to fn to evaluate e, a &&- or
// ||-expression whose reified boolean value is wanted.
// The value is returned.
func (b *builder) logicalBinop(fn *Function, e *ast.BinaryExpr) Value {
	rhs := fn.newBasicBlock("binop.rhs")
	done := fn.newBasicBlock("binop.done")

	// T(e) = T(e.X) = T(e.Y) after untyped constants have been
	// eliminated.
	// TODO(adonovan): not true; MyBool==MyBool yields UntypedBool.
	t := fn.Pkg.typeOf(e)

	var short Value // value of the short-circuit path
	switch e.Op {
	case token.LAND:
		b.cond(fn, e.X, rhs, done)
		short = emitConst(fn, NewConst(constant.MakeBool(false), t, e))

	case token.LOR:
		b.cond(fn, e.X, done, rhs)
		short = emitConst(fn, NewConst(constant.MakeBool(true), t, e))
	}

	// Is rhs unreachable?
	if rhs.Preds == nil {
		// Simplify false&&y to false, true||y to true.
		fn.currentBlock = done
		return short
	}

	// Is done unreachable?
	if done.Preds == nil {
		// Simplify true&&y (or false||y) to y.
		fn.currentBlock = rhs
		return b.expr(fn, e.Y)
	}

	// All edges from e.X to done carry the short-circuit value.
	var edges []Value
	for range done.Preds {
		edges = append(edges, short)
	}

	// The edge from e.Y to done carries the value of e.Y.
	fn.currentBlock = rhs
	edges = append(edges, b.expr(fn, e.Y))
	emitJump(fn, done, e)
	fn.currentBlock = done

	phi := &Phi{Edges: edges}
	phi.typ = t
	phi.comment = e.Op.String()
	return done.emit(phi, e)
}

// exprN lowers a multi-result expression e to IR form, emitting code
// to fn and returning a single Value whose type is a *types.Tuple.
// The caller must access the components via Extract.
//
// Multi-result expressions include CallExprs in a multi-value
// assignment or return statement, and "value,ok" uses of
// TypeAssertExpr, IndexExpr (when X is a map), and Recv.
func (b *builder) exprN(fn *Function, e ast.Expr) Value {
	typ := fn.Pkg.typeOf(e).(*types.Tuple)
	switch e := e.(type) {
	case *ast.ParenExpr:
		return b.exprN(fn, e.X)

	case *ast.CallExpr:
		// Currently, no built-in function nor type conversion
		// has multiple results, so we can avoid some of the
		// cases for single-valued CallExpr.
		var c Call
		b.setCall(fn, e, &c.Call)
		c.typ = typ
		return emitCall(fn, &c, e)

	case *ast.IndexExpr:
		mapt := typeutil.CoreType(fn.Pkg.typeOf(e.X)).Underlying().(*types.Map)
		lookup := &MapLookup{
			X:       b.expr(fn, e.X),
			Index:   emitConv(fn, b.expr(fn, e.Index), mapt.Key(), e),
			CommaOk: true,
		}
		lookup.setType(typ)
		return fn.emit(lookup, e)

	case *ast.TypeAssertExpr:
		return emitTypeTest(fn, b.expr(fn, e.X), typ.At(0).Type(), e)

	case *ast.UnaryExpr: // must be receive <-
		return emitRecv(fn, b.expr(fn, e.X), true, typ, e)
	}
	panic(fmt.Sprintf("exprN(%T) in %s", e, fn))
}

// builtin emits to fn IR instructions to implement a call to the
// built-in function obj with the specified arguments
// and return type.  It returns the value defined by the result.
//
// The result is nil if no special handling was required; in this case
// the caller should treat this like an ordinary library function
// call.
func (b *builder) builtin(fn *Function, obj *types.Builtin, args []ast.Expr, typ types.Type, source ast.Node) Value {
	switch obj.Name() {
	case "make":
		styp := typ.Underlying()
		if _, ok := typ.Underlying().(*types.Interface); ok {
			// This must be a type parameter with a core type.
			// Set styp to the core type and generate instructions based on it.
			assert(typeparams.IsTypeParam(typ))
			styp = typeutil.CoreType(typ)
			assert(styp != nil)
		}
		switch styp.(type) {
		case *types.Slice:
			n := b.expr(fn, args[1])
			m := n
			if len(args) == 3 {
				m = b.expr(fn, args[2])
			}
			if m, ok := m.(*Const); ok {
				// treat make([]T, n, m) as new([m]T)[:n]
				cap := m.Int64()
				at := types.NewArray(styp.Underlying().(*types.Slice).Elem(), cap)
				v := &Slice{
					X:    emitNew(fn, at, source, "makeslice"),
					High: n,
				}
				v.setType(typ)
				return fn.emit(v, source)
			}
			v := &MakeSlice{
				Len: n,
				Cap: m,
			}
			v.setType(typ)
			return fn.emit(v, source)

		case *types.Map:
			var res Value
			if len(args) == 2 {
				res = b.expr(fn, args[1])
			}
			v := &MakeMap{Reserve: res}
			v.setType(typ)
			return fn.emit(v, source)

		case *types.Chan:
			var sz Value = emitConst(fn, intConst(0, source))
			if len(args) == 2 {
				sz = b.expr(fn, args[1])
			}
			v := &MakeChan{Size: sz}
			v.setType(typ)
			return fn.emit(v, source)

		default:
			lint.ExhaustiveTypeSwitch(typ.Underlying())
		}

	case "new":
		alloc := emitNew(fn, deref(typ), source, "new")
		if !fn.Pkg.info.Types[args[0]].IsType() {
			v := b.expr(fn, args[0])
			emitStore(fn, alloc, v, source)
		}
		return alloc

	case "len", "cap":
		// Special case: len or cap of an array or *array is based on the type, not the value which may be nil. We must
		// still evaluate the value, though. (If it was side-effect free, the whole call would have been
		// constant-folded.)
		//
		// For example, for len(gen()), we need to evaluate gen() for its side-effects, but don't need the returned
		// value to determine the length of the array, which is constant.
		//
		// Technically this shouldn't apply to type parameters because their length/capacity is never constant. We still
		// choose to treat them as constant so that users of the IR get the practically constant length for free.
		t := typeutil.CoreType(deref(fn.Pkg.typeOf(args[0])))
		if at, ok := t.(*types.Array); ok {
			b.expr(fn, args[0]) // for effects only
			return emitConst(fn, intConst(at.Len(), args[0]))
		}
		// Otherwise treat as normal.

	case "panic":
		fn.emit(&Panic{
			X: emitConv(fn, b.expr(fn, args[0]), tEface, source),
		}, source)
		addEdge(fn.currentBlock, fn.Exit)
		fn.currentBlock = fn.newBasicBlock("unreachable")
		return emitConst(fn, NewConst(constant.MakeBool(true), tBool, nil)) // any non-nil Value will do
	}
	return nil // treat all others as a regular function call
}

// addr lowers a single-result addressable expression e to IR form,
// emitting code to fn and returning the location (an lvalue) defined
// by the expression.
//
// If escaping is true, addr marks the base variable of the
// addressable expression e as being a potentially escaping pointer
// value.  For example, in this code:
//
//	a := A{
//	  b: [1]B{B{c: 1}}
//	}
//	return &a.b[0].c
//
// the application of & causes a.b[0].c to have its address taken,
// which means that ultimately the local variable a must be
// heap-allocated.  This is a simple but very conservative escape
// analysis.
//
// Operations forming potentially escaping pointers include:
// - &x, including when implicit in method call or composite literals.
// - a[:] iff a is an array (not *array)
// - references to variables in lexically enclosing functions.
func (b *builder) addr(fn *Function, e ast.Expr, escaping bool) (RET lvalue) {
	switch e := e.(type) {
	case *ast.Ident:
		if isBlankIdent(e) {
			return blank{}
		}
		obj := fn.Pkg.objectOf(e)
		v := fn.Prog.packageLevelValue(obj) // var (address)
		if v == nil {
			v = fn.lookup(obj.(*types.Var), escaping)
		}
		return &address{addr: v, expr: e}

	case *ast.CompositeLit:
		t := deref(fn.Pkg.typeOf(e))
		var v *Alloc
		if escaping {
			v = emitNew(fn, t, e, "complit")
		} else {
			v = emitLocal(fn, t, e, "complit")
		}
		var sb storebuf
		b.compLit(fn, v, e, true, &sb)
		sb.emit(fn)
		return &address{addr: v, expr: e}

	case *ast.ParenExpr:
		return b.addr(fn, e.X, escaping)

	case *ast.SelectorExpr:
		sel, ok := fn.Pkg.info.Selections[e]
		if !ok {
			// qualified identifier
			return b.addr(fn, e.Sel, escaping)
		}
		if sel.Kind() != types.FieldVal {
			panic(sel)
		}
		wantAddr := true
		v := b.receiver(fn, e.X, wantAddr, escaping, sel, e)
		index := sel.Index()[len(sel.Index())-1]
		vut := typeutil.CoreType(deref(v.Type())).Underlying().(*types.Struct)
		fld := vut.Field(index)
		// Due to the two phases of resolving AssignStmt, a panic from x.f = p()
		// when x is nil is required to come after the side-effects of
		// evaluating x and p().
		emit := func(fn *Function) Value {
			return emitFieldSelection(fn, v, index, true, e.Sel)
		}
		return &lazyAddress{addr: emit, t: fld.Type(), expr: e.Sel}

	case *ast.IndexExpr:
		var x Value
		var et types.Type
		xt := fn.Pkg.typeOf(e.X)

		// Indexing doesn't need a core type, it only requires all types to be similar enough. For example, []int64 |
		// [5]int64 can be indexed. The element types do have to match though.

		terms, err := typeparams.NormalTerms(xt)
		if err != nil {
			panic(fmt.Sprintf("unexpected error: %s", err))
		}
		isArrayLike := func() (types.Type, bool) {
			for _, term := range terms {
				arr, ok := term.Type().Underlying().(*types.Array)
				if ok {
					return arr.Elem(), true
				}
			}
			return nil, false
		}

		isSliceLike := func() (types.Type, bool) {
			for _, term := range terms {
				switch t := term.Type().Underlying().(type) {
				case *types.Slice:
					return t.Elem(), true
				case *types.Pointer:
					return t.Elem().Underlying().(*types.Array).Elem(), true
				}
			}
			return nil, false
		}

		if elem, ok := isArrayLike(); ok {
			// array
			x = b.addr(fn, e.X, escaping).address(fn)
			et = types.NewPointer(elem)
		} else if elem, ok := isSliceLike(); ok {
			// slice or *array
			x = b.expr(fn, e.X)
			et = types.NewPointer(elem)
		} else if t, ok := typeutil.CoreType(xt).Underlying().(*types.Map); ok {
			return &element{
				m: b.expr(fn, e.X),
				k: emitConv(fn, b.expr(fn, e.Index), t.Key(), e.Index),
				t: t.Elem(),
			}
		} else {
			panic("unexpected container type in IndexExpr: " + t.String())
		}

		// Due to the two phases of resolving AssignStmt, a panic from x[i] = p()
		// when x is nil or i is out-of-bounds is required to come after the
		// side-effects of evaluating x, i and p().
		index := b.expr(fn, e.Index)
		emit := func(fn *Function) Value {
			v := &IndexAddr{
				X:     x,
				Index: index,
			}
			v.setType(et)
			return fn.emit(v, e)
		}
		return &lazyAddress{addr: emit, t: deref(et), expr: e}

	case *ast.StarExpr:
		return &address{addr: b.expr(fn, e.X), expr: e}
	}

	panic(fmt.Sprintf("unexpected address expression: %T", e))
}

type store struct {
	lhs    lvalue
	rhs    Value
	source ast.Node

	// if debugRef is set no other fields will be set
	debugRef *DebugRef
}

type storebuf struct{ stores []store }

func (sb *storebuf) store(lhs lvalue, rhs Value, source ast.Node) {
	sb.stores = append(sb.stores, store{lhs, rhs, source, nil})
}

func (sb *storebuf) storeDebugRef(ref *DebugRef) {
	sb.stores = append(sb.stores, store{debugRef: ref})
}

func (sb *storebuf) emit(fn *Function) {
	for _, s := range sb.stores {
		if s.debugRef == nil {
			s.lhs.store(fn, s.rhs, s.source)
		} else {
			fn.emit(s.debugRef, nil)
		}
	}
}

// assign emits to fn code to initialize the lvalue loc with the value
// of expression e.  If isZero is true, assign assumes that loc holds
// the zero value for its type.
//
// This is equivalent to loc.store(fn, b.expr(fn, e)), but may generate
// better code in some cases, e.g., for composite literals in an
// addressable location.
//
// If sb is not nil, assign generates code to evaluate expression e, but
// not to update loc.  Instead, the necessary stores are appended to the
// storebuf sb so that they can be executed later.  This allows correct
// in-place update of existing variables when the RHS is a composite
// literal that may reference parts of the LHS.
func (b *builder) assign(fn *Function, loc lvalue, e ast.Expr, isZero bool, sb *storebuf, source ast.Node) {
	// Can we initialize it in place?
	if e, ok := unparen(e).(*ast.CompositeLit); ok {
		// A CompositeLit never evaluates to a pointer,
		// so if the type of the location is a pointer,
		// an &-operation is implied.
		if _, ok := loc.(blank); !ok { // avoid calling blank.typ()
			if isPointer(loc.typ()) {
				// Example input that hits this code:
				//
				// 	type S1 struct{ X int }
				// 	x := []*S1{
				// 		{1}, // <-- & is implied
				// 	}
				// 	_ = x
				ptr := b.addr(fn, e, true).address(fn)
				// copy address
				if sb != nil {
					sb.store(loc, ptr, source)
				} else {
					loc.store(fn, ptr, source)
				}
				return
			}
		}

		if _, ok := loc.(*address); ok {
			if types.IsInterface(loc.typ()) && !typeparams.IsTypeParam(loc.typ()) {
				// e.g. var x interface{} = T{...}
				// Can't in-place initialize an interface value.
				// Fall back to copying.
			} else {
				// x = T{...} or x := T{...}
				addr := loc.address(fn)
				if sb != nil {
					b.compLit(fn, addr, e, isZero, sb)
				} else {
					var sb storebuf
					b.compLit(fn, addr, e, isZero, &sb)
					sb.emit(fn)
				}

				// Subtle: emit debug ref for aggregate types only;
				// slice and map are handled by store ops in compLit.
				switch typeutil.CoreType(loc.typ()).Underlying().(type) {
				case *types.Struct, *types.Array:
					if sb != nil {
						// Make sure we don't emit DebugRefs before the store has actually occurred
						if ref := makeDebugRef(fn, e, addr, true); ref != nil {
							sb.storeDebugRef(ref)
						}
					} else {
						emitDebugRef(fn, e, addr, true)
					}
				}

				return
			}
		}
	}

	// simple case: just copy
	rhs := b.expr(fn, e)
	if sb != nil {
		sb.store(loc, rhs, source)
	} else {
		loc.store(fn, rhs, source)
	}
}

// expr lowers a single-result expression e to IR form, emitting code
// to fn and returning the Value defined by the expression.
func (b *builder) expr(fn *Function, e ast.Expr) Value {
	e = unparen(e)

	tv := fn.Pkg.info.Types[e]

	// Is expression a constant?
	if tv.Value != nil {
		return emitConst(fn, NewConst(tv.Value, tv.Type, e))
	}

	var v Value
	if tv.Addressable() {
		// Prefer pointer arithmetic ({Index,Field}Addr) followed
		// by Load over subelement extraction (e.g. Index, Field),
		// to avoid large copies.
		v = b.addr(fn, e, false).load(fn, e)
	} else {
		v = b.expr0(fn, e, tv)
	}
	if fn.debugInfo() {
		emitDebugRef(fn, e, v, false)
	}
	return v
}

func (b *builder) expr0(fn *Function, e ast.Expr, tv types.TypeAndValue) Value {
	switch e := e.(type) {
	case *ast.BasicLit:
		panic("non-constant BasicLit") // unreachable

	case *ast.FuncLit:
		fn2 := &Function{
			name:         fmt.Sprintf("%s$%d", fn.Name(), 1+len(fn.AnonFuncs)),
			Signature:    fn.Pkg.typeOf(e.Type).Underlying().(*types.Signature),
			parent:       fn,
			Pkg:          fn.Pkg,
			Prog:         fn.Prog,
			functionBody: new(functionBody),
			goversion:    fn.goversion, // share the parent's goversion
		}
		fn2.uniq = fn.uniq // start from parent's unique values
		fn2.source = e
		fn.AnonFuncs = append(fn.AnonFuncs, fn2)
		fn2.initHTML(b.printFunc)
		b.buildFunction(fn2)
		fn.uniq = fn2.uniq // resume after anon's unique values
		if fn2.FreeVars == nil {
			return fn2
		}
		v := &MakeClosure{Fn: fn2}
		v.setType(tv.Type)
		for _, fv := range fn2.FreeVars {
			v.Bindings = append(v.Bindings, fv.outer)
			fv.outer = nil
		}
		return fn.emit(v, e)

	case *ast.TypeAssertExpr: // single-result form only
		return emitTypeAssert(fn, b.expr(fn, e.X), tv.Type, e)

	case *ast.CallExpr:
		if fn.Pkg.info.Types[e.Fun].IsType() {
			// Explicit type conversion, e.g. string(x) or big.Int(x)
			x := b.expr(fn, e.Args[0])
			y := emitConv(fn, x, tv.Type, e)
			return y
		}
		// Call to "intrinsic" built-ins, e.g. new, make, panic.
		if id, ok := unparen(e.Fun).(*ast.Ident); ok {
			if obj, ok := fn.Pkg.info.Uses[id].(*types.Builtin); ok {
				if v := b.builtin(fn, obj, e.Args, tv.Type, e); v != nil {
					return v
				}
			}
		}
		// Regular function call.
		var v Call
		b.setCall(fn, e, &v.Call)
		v.setType(tv.Type)
		return emitCall(fn, &v, e)

	case *ast.UnaryExpr:
		switch e.Op {
		case token.AND: // &X --- potentially escaping.
			addr := b.addr(fn, e.X, true)
			if _, ok := unparen(e.X).(*ast.StarExpr); ok {
				// &*p must panic if p is nil (https://golang.org/s/go12nil).
				// For simplicity, we'll just (suboptimally) rely
				// on the side effects of a load.
				// TODO(adonovan): emit dedicated nilcheck.
				addr.load(fn, e)
			}
			return addr.address(fn)
		case token.ADD:
			return b.expr(fn, e.X)
		case token.NOT, token.SUB, token.XOR: // ! <- - ^
			v := &UnOp{
				Op: e.Op,
				X:  b.expr(fn, e.X),
			}
			v.setType(tv.Type)
			return fn.emit(v, e)
		case token.ARROW:
			return emitRecv(fn, b.expr(fn, e.X), false, tv.Type, e)
		default:
			panic(e.Op)
		}

	case *ast.BinaryExpr:
		switch e.Op {
		case token.LAND, token.LOR:
			return b.logicalBinop(fn, e)
		case token.SHL, token.SHR:
			fallthrough
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT:
			return emitArith(fn, e.Op, b.expr(fn, e.X), b.expr(fn, e.Y), tv.Type, e)

		case token.EQL, token.NEQ, token.GTR, token.LSS, token.LEQ, token.GEQ:
			cmp := emitCompare(fn, e.Op, b.expr(fn, e.X), b.expr(fn, e.Y), e)
			// The type of x==y may be UntypedBool.
			return emitConv(fn, cmp, types.Default(tv.Type), e)
		default:
			panic("illegal op in BinaryExpr: " + e.Op.String())
		}

	case *ast.SliceExpr:
		var x Value
		if core := typeutil.CoreType(fn.Pkg.typeOf(e.X)); core != nil {
			switch core.Underlying().(type) {
			case *types.Array:
				// Potentially escaping.
				x = b.addr(fn, e.X, true).address(fn)
			case *types.Basic, *types.Slice, *types.Pointer: // *array
				x = b.expr(fn, e.X)
			default:
				panic("unreachable")
			}
		} else {
			// We're indexing a string | []byte. Note that other combinations such as []byte | [4]byte are currently not
			// allowed by the language.
			x = b.expr(fn, e.X)
		}

		var low, high, max Value
		if e.Low != nil {
			low = b.expr(fn, e.Low)
		}
		if e.High != nil {
			high = b.expr(fn, e.High)
		}
		if e.Slice3 {
			max = b.expr(fn, e.Max)
		}
		v := &Slice{
			X:    x,
			Low:  low,
			High: high,
			Max:  max,
		}
		v.setType(tv.Type)
		return fn.emit(v, e)

	case *ast.Ident:
		obj := fn.Pkg.info.Uses[e]
		// Universal built-in or nil?
		switch obj := obj.(type) {
		case *types.Builtin:
			return &Builtin{name: obj.Name(), sig: tv.Type.(*types.Signature)}
		case *types.Nil:
			return emitConst(fn, nilConst(tv.Type, e))
		}
		// Package-level func or var?
		if v := fn.Prog.packageLevelValue(obj); v != nil {
			if _, ok := obj.(*types.Var); ok {
				return emitLoad(fn, v, e) // var (address)
			}
			if instance, ok := fn.Pkg.info.Instances[e]; ok {
				// Instantiated generic function
				return makeInstance(fn.Prog, v.(*Function), instance.Type.(*types.Signature), instance.TypeArgs)
			}
			return v // (func)
		}
		// Local var.
		return emitLoad(fn, fn.lookup(obj.(*types.Var), false), e) // var (address)

	case *ast.SelectorExpr:
		sel, ok := fn.Pkg.info.Selections[e]
		if !ok {
			// builtin unsafe.{Add,Slice}
			if obj, ok := fn.Pkg.info.Uses[e.Sel].(*types.Builtin); ok {
				return &Builtin{name: "Unsafe" + obj.Name(), sig: tv.Type.(*types.Signature)}
			}
			// qualified identifier
			return b.expr(fn, e.Sel)
		}
		switch sel.Kind() {
		case types.MethodExpr:
			// (*T).f or T.f, the method f from the method-set of type T.
			// The result is a "thunk".
			return emitConv(fn, makeThunk(fn.Prog, sel), tv.Type, e)

		case types.MethodVal:
			// e.f where e is an expression and f is a method.
			// The result is a "bound".
			obj := sel.Obj().(*types.Func)
			rt := recvType(obj)
			wantAddr := isPointer(rt)
			escaping := true
			v := b.receiver(fn, e.X, wantAddr, escaping, sel, e)
			if types.IsInterface(rt) {
				// If v has interface type I,
				// we must emit a check that v is non-nil.
				// We use: typeassert v.(I).
				emitTypeAssert(fn, v, rt, e)
			}
			c := &MakeClosure{
				Fn:       makeBound(fn.Prog, obj),
				Bindings: []Value{v},
			}
			c.source = e.Sel
			c.setType(tv.Type)
			return fn.emit(c, e)

		case types.FieldVal:
			indices := sel.Index()
			last := len(indices) - 1
			v := b.expr(fn, e.X)
			v = emitImplicitSelections(fn, v, indices[:last], e)
			v = emitFieldSelection(fn, v, indices[last], false, e.Sel)
			return v
		}

		panic("unexpected expression-relative selector")

	case *ast.IndexExpr:
		// IndexExpr might either be an actual indexing operation, or an instantiation
		xt := fn.Pkg.typeOf(e.X)

		terms, err := typeparams.NormalTerms(xt)
		if err != nil {
			panic(fmt.Sprintf("unexpected error: %s", err))
		}
		isNonAddressableIndexable := func() (types.Type, bool) {
			for _, term := range terms {
				switch t := term.Type().Underlying().(type) {
				case *types.Array:
					return t.Elem(), true
				case *types.Basic:
					// a string
					return types.Universe.Lookup("byte").Type(), true
				}
			}
			return nil, false
		}

		isAddressableIndexable := func() (types.Type, bool) {
			for _, term := range terms {
				switch t := term.Type().Underlying().(type) {
				case *types.Slice:
					return t.Elem(), true
				case *types.Pointer:
					return t.Elem().Underlying().(*types.Array).Elem(), true
				}
			}
			return nil, false
		}

		if elem, ok := isNonAddressableIndexable(); ok {
			// At least one of the types is non-addressable
			v := &Index{
				X:     b.expr(fn, e.X),
				Index: b.expr(fn, e.Index),
			}
			v.setType(elem)
			return fn.emit(v, e)
		} else if _, ok := isAddressableIndexable(); ok {
			// All types are addressable (otherwise the previous branch would've fired)
			return b.addr(fn, e, false).load(fn, e)
		} else if t, ok := typeutil.CoreType(xt).Underlying().(*types.Map); ok {
			// Maps are not addressable.
			v := &MapLookup{
				X:     b.expr(fn, e.X),
				Index: emitConv(fn, b.expr(fn, e.Index), t.Key(), e.Index),
			}
			v.setType(t.Elem())
			return fn.emit(v, e)
		} else if _, ok := xt.Underlying().(*types.Signature); ok {
			// Instantiating a generic function
			return b.expr(fn, e.X)
		} else {
			panic("unexpected container type in IndexExpr: " + t.String())
		}

	case *ast.IndexListExpr:
		// Instantiating a generic function
		return b.expr(fn, e.X)

	case *ast.CompositeLit, *ast.StarExpr:
		// Addressable types (lvalues)
		return b.addr(fn, e, false).load(fn, e)
	}

	panic(fmt.Sprintf("unexpected expr: %T", e))
}

// stmtList emits to fn code for all statements in list.
func (b *builder) stmtList(fn *Function, list []ast.Stmt) {
	for _, s := range list {
		b.stmt(fn, s)
	}
}

// receiver emits to fn code for expression e in the "receiver"
// position of selection e.f (where f may be a field or a method) and
// returns the effective receiver after applying the implicit field
// selections of sel.
//
// wantAddr requests that the result is an address.  If
// !sel.Indirect(), this may require that e be built in addr() mode; it
// must thus be addressable.
//
// escaping is defined as per builder.addr().
func (b *builder) receiver(fn *Function, e ast.Expr, wantAddr, escaping bool, sel *types.Selection, source ast.Node) Value {
	var v Value
	if wantAddr && !sel.Indirect() && !isPointer(fn.Pkg.typeOf(e)) {
		v = b.addr(fn, e, escaping).address(fn)
	} else {
		v = b.expr(fn, e)
	}

	last := len(sel.Index()) - 1
	v = emitImplicitSelections(fn, v, sel.Index()[:last], source)
	if !wantAddr && isPointer(v.Type()) {
		v = emitLoad(fn, v, e)
	}
	return v
}

// setCallFunc populates the function parts of a CallCommon structure
// (Func, Method, Recv, Args[0]) based on the kind of invocation
// occurring in e.
func (b *builder) setCallFunc(fn *Function, e *ast.CallExpr, c *CallCommon) {
	// Is this a method call?
	if selector, ok := unparen(e.Fun).(*ast.SelectorExpr); ok {
		sel, ok := fn.Pkg.info.Selections[selector]
		if ok && sel.Kind() == types.MethodVal {
			obj := sel.Obj().(*types.Func)
			recv := recvType(obj)
			wantAddr := isPointer(recv)
			escaping := true
			v := b.receiver(fn, selector.X, wantAddr, escaping, sel, selector)
			if types.IsInterface(recv) {
				// Invoke-mode call.

				// Methods in interfaces cannot have their own type parameters, so we needn't do anything for type
				// parameters.
				c.Value = v
				c.Method = obj
			} else {
				// "Call"-mode call.

				// declaredFunc takes care of creating wrappers for functions with type parameters.
				c.Value = fn.Prog.declaredFunc(obj)
				c.Args = append(c.Args, v)
			}
			return
		}

		// sel.Kind()==MethodExpr indicates T.f() or (*T).f():
		// a statically dispatched call to the method f in the
		// method-set of T or *T.  T may be an interface.
		//
		// e.Fun would evaluate to a concrete method, interface
		// wrapper function, or promotion wrapper.
		//
		// For now, we evaluate it in the usual way.
		//
		// TODO(adonovan): opt: inline expr() here, to make the
		// call static and to avoid generation of wrappers.
		// It's somewhat tricky as it may consume the first
		// actual parameter if the call is "invoke" mode.
		//
		// Examples:
		//  type T struct{}; func (T) f() {}   // "call" mode
		//  type T interface { f() }           // "invoke" mode
		//
		//  type S struct{ T }
		//
		//  var s S
		//  S.f(s)
		//  (*S).f(&s)
		//
		// Suggested approach:
		// - consume the first actual parameter expression
		//   and build it with b.expr().
		// - apply implicit field selections.
		// - use MethodVal logic to populate fields of c.
	}
	// Evaluate the function operand in the usual way.
	//
	// Code in expr takes care of creating wrappers for functions with type parameters.
	c.Value = b.expr(fn, e.Fun)
}

// emitCallArgs emits to f code for the actual parameters of call e to
// a (possibly built-in) function of effective type sig.
// The argument values are appended to args, which is then returned.
func (b *builder) emitCallArgs(fn *Function, sig *types.Signature, e *ast.CallExpr, args []Value) []Value {
	// f(x, y, z...): pass slice z straight through.
	if e.Ellipsis != 0 {
		for i, arg := range e.Args {
			v := emitConv(fn, b.expr(fn, arg), sig.Params().At(i).Type(), arg)
			args = append(args, v)
		}
		return args
	}

	offset := len(args) // 1 if call has receiver, 0 otherwise

	// Evaluate actual parameter expressions.
	//
	// If this is a chained call of the form f(g()) where g has
	// multiple return values (MRV), they are flattened out into
	// args; a suffix of them may end up in a varargs slice.
	for _, arg := range e.Args {
		v := b.expr(fn, arg)
		if ttuple, ok := v.Type().(*types.Tuple); ok { // MRV chain
			for i, n := 0, ttuple.Len(); i < n; i++ {
				args = append(args, emitExtract(fn, v, i, arg))
			}
		} else {
			args = append(args, v)
		}
	}

	// Actual->formal assignability conversions for normal parameters.
	np := sig.Params().Len() // number of normal parameters
	if sig.Variadic() {
		np--
	}
	for i := 0; i < np; i++ {
		args[offset+i] = emitConv(fn, args[offset+i], sig.Params().At(i).Type(), args[offset+i].Source())
	}

	// Actual->formal assignability conversions for variadic parameter,
	// and construction of slice.
	if sig.Variadic() {
		varargs := args[offset+np:]
		st := sig.Params().At(np).Type().(*types.Slice)
		vt := st.Elem()
		if len(varargs) == 0 {
			args = append(args, emitConst(fn, nilConst(st, nil)))
		} else {
			// Replace a suffix of args with a slice containing it.
			at := types.NewArray(vt, int64(len(varargs)))
			a := emitNew(fn, at, e, "varargs")
			a.source = e
			for i, arg := range varargs {
				iaddr := &IndexAddr{
					X:     a,
					Index: emitConst(fn, intConst(int64(i), nil)),
				}
				iaddr.setType(types.NewPointer(vt))
				fn.emit(iaddr, e)
				emitStore(fn, iaddr, arg, arg.Source())
			}
			s := &Slice{X: a}
			s.setType(st)
			args[offset+np] = fn.emit(s, args[offset+np].Source())
			args = args[:offset+np+1]
		}
	}
	return args
}

// setCall emits to fn code to evaluate all the parameters of a function
// call e, and populates *c with those values.
func (b *builder) setCall(fn *Function, e *ast.CallExpr, c *CallCommon) {
	// First deal with the f(...) part and optional receiver.
	b.setCallFunc(fn, e, c)

	// Then append the other actual parameters.
	sig, _ := typeutil.CoreType(fn.Pkg.typeOf(e.Fun)).(*types.Signature)
	if sig == nil {
		panic(fmt.Sprintf("no signature for call of %s", e.Fun))
	}
	c.Args = b.emitCallArgs(fn, sig, e, c.Args)
}

// assignOp emits to fn code to perform loc <op>= val.
func (b *builder) assignOp(fn *Function, loc lvalue, val Value, op token.Token, source ast.Node) {
	loc.store(fn, emitArith(fn, op, loc.load(fn, source), val, loc.typ(), source), source)
}

// localValueSpec emits to fn code to define all of the vars in the
// function-local ValueSpec, spec.
func (b *builder) localValueSpec(fn *Function, spec *ast.ValueSpec) {
	switch {
	case len(spec.Values) == len(spec.Names):
		// e.g. var x, y = 0, 1
		// 1:1 assignment
		for i, id := range spec.Names {
			if !isBlankIdent(id) {
				emitLocalVar(fn, identVar(fn, id), id)
			}
			lval := b.addr(fn, id, false) // non-escaping
			b.assign(fn, lval, spec.Values[i], true, nil, spec)
		}

	case len(spec.Values) == 0:
		// e.g. var x, y int
		// Locals are implicitly zero-initialized.
		for _, id := range spec.Names {
			if !isBlankIdent(id) {
				lhs := emitLocalVar(fn, identVar(fn, id), id)
				if fn.debugInfo() {
					emitDebugRef(fn, id, lhs, true)
				}
			}
		}

	default:
		// e.g. var x, y = pos()
		tuple := b.exprN(fn, spec.Values[0])
		for i, id := range spec.Names {
			if !isBlankIdent(id) {
				emitLocalVar(fn, identVar(fn, id), id)
				lhs := b.addr(fn, id, false) // non-escaping
				lhs.store(fn, emitExtract(fn, tuple, i, id), id)
			}
		}
	}
}

// assignStmt emits code to fn for a parallel assignment of rhss to lhss.
// isDef is true if this is a short variable declaration (:=).
//
// Note the similarity with localValueSpec.
func (b *builder) assignStmt(fn *Function, lhss, rhss []ast.Expr, isDef bool, source ast.Node) {
	// Side effects of all LHSs and RHSs must occur in left-to-right order.
	lvals := make([]lvalue, len(lhss))
	isZero := make([]bool, len(lhss))
	for i, lhs := range lhss {
		var lval lvalue = blank{}
		if !isBlankIdent(lhs) {
			if isDef {
				if obj, ok := fn.Pkg.info.Defs[lhs.(*ast.Ident)].(*types.Var); ok {
					emitLocalVar(fn, obj, lhs)
					isZero[i] = true
				}
			}
			lval = b.addr(fn, lhs, false) // non-escaping
		}
		lvals[i] = lval
	}
	if len(lhss) == len(rhss) {
		// Simple assignment:   x     = f()        (!isDef)
		// Parallel assignment: x, y  = f(), g()   (!isDef)
		// or short var decl:   x, y := f(), g()   (isDef)
		//
		// In all cases, the RHSs may refer to the LHSs,
		// so we need a storebuf.
		var sb storebuf
		for i := range rhss {
			b.assign(fn, lvals[i], rhss[i], isZero[i], &sb, source)
		}
		sb.emit(fn)
	} else {
		// e.g. x, y = pos()
		tuple := b.exprN(fn, rhss[0])
		emitDebugRef(fn, rhss[0], tuple, false)
		for i, lval := range lvals {
			lval.store(fn, emitExtract(fn, tuple, i, source), source)
		}
	}
}

// arrayLen returns the length of the array whose composite literal elements are elts.
func (b *builder) arrayLen(fn *Function, elts []ast.Expr) int64 {
	var max int64 = -1
	var i int64 = -1
	for _, e := range elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			i = b.expr(fn, kv.Key).(*Const).Int64()
		} else {
			i++
		}
		if i > max {
			max = i
		}
	}
	return max + 1
}

// compLit emits to fn code to initialize a composite literal e at
// address addr with type typ.
//
// Nested composite literals are recursively initialized in place
// where possible. If isZero is true, compLit assumes that addr
// holds the zero value for typ.
//
// Because the elements of a composite literal may refer to the
// variables being updated, as in the second line below,
//
//	x := T{a: 1}
//	x = T{a: x.a}
//
// all the reads must occur before all the writes. This is implicitly handled by the write buffering effected by
// compositeElement and explicitly by the storebuf for when we don't use CompositeValue.
//
// A CompositeLit may have pointer type only in the recursive (nested)
// case when the type name is implicit.  e.g. in []*T{{}}, the inner
// literal has type *T behaves like &T{}.
// In that case, addr must hold a T, not a *T.
func (b *builder) compLit(fn *Function, addr Value, e *ast.CompositeLit, isZero bool, sb *storebuf) {
	typ := deref(fn.Pkg.typeOf(e))
	switch t := typeutil.CoreType(typ).(type) {
	case *types.Struct:
		lvalue := &address{addr: addr, expr: e}
		if len(e.Elts) == 0 {
			if !isZero {
				sb.store(lvalue, zeroValue(fn, deref(addr.Type()), e), e)
			}
		} else {
			v := &CompositeValue{
				Values: make([]Value, t.NumFields()),
			}
			for i := 0; i < t.NumFields(); i++ {
				v.Values[i] = emitConst(fn, zeroConst(t.Field(i).Type(), e))
			}
			v.setType(typ)

			for i, e := range e.Elts {
				fieldIndex := i
				if kv, ok := e.(*ast.KeyValueExpr); ok {
					fname := kv.Key.(*ast.Ident).Name
					for i, n := 0, t.NumFields(); i < n; i++ {
						sf := t.Field(i)
						if sf.Name() == fname {
							fieldIndex = i
							e = kv.Value
							break
						}
					}
				}

				ce := &compositeElement{
					cv:   v,
					idx:  fieldIndex,
					t:    t.Field(fieldIndex).Type(),
					expr: e,
				}
				b.assign(fn, ce, e, isZero, sb, e)
				v.Bitmap.SetBit(&v.Bitmap, fieldIndex, 1)
				v.NumSet++
			}
			fn.emit(v, e)
			sb.store(lvalue, v, e)
		}

	case *types.Array, *types.Slice:
		var at *types.Array
		var array Value
		switch t := t.(type) {
		case *types.Slice:
			at = types.NewArray(t.Elem(), b.arrayLen(fn, e.Elts))
			array = emitNew(fn, at, e, "slicelit")
		case *types.Array:
			at = t
			array = addr
		}

		var final Value
		if len(e.Elts) == 0 {
			if !isZero {
				zc := emitConst(fn, zeroConst(at, e))
				final = zc
			}
		} else {
			if at.Len() == int64(len(e.Elts)) {
				// The literal specifies all elements, so we can use a composite value
				v := &CompositeValue{
					Values: make([]Value, at.Len()),
				}
				zc := emitConst(fn, zeroConst(at.Elem(), e))
				for i := range v.Values {
					v.Values[i] = zc
				}
				v.setType(at)

				var idx *Const
				for _, e := range e.Elts {
					if kv, ok := e.(*ast.KeyValueExpr); ok {
						idx = b.expr(fn, kv.Key).(*Const)
						e = kv.Value
					} else {
						var idxval int64
						if idx != nil {
							idxval = idx.Int64() + 1
						}
						idx = emitConst(fn, intConst(idxval, e)).(*Const)
					}

					iaddr := &compositeElement{
						cv:   v,
						idx:  int(idx.Int64()),
						t:    at.Elem(),
						expr: e,
					}

					b.assign(fn, iaddr, e, true, sb, e)
					v.Bitmap.SetBit(&v.Bitmap, int(idx.Int64()), 1)
					v.NumSet++
				}
				final = v
				fn.emit(v, e)
			} else {
				// Not all elements are specified. Populate the array with a series of stores, to guard against literals
				// like []int{1<<62: 1}.
				if !isZero {
					// memclear
					sb.store(&address{array, nil}, zeroValue(fn, deref(array.Type()), e), e)
				}

				var idx *Const
				for _, e := range e.Elts {
					if kv, ok := e.(*ast.KeyValueExpr); ok {
						idx = b.expr(fn, kv.Key).(*Const)
						e = kv.Value
					} else {
						var idxval int64
						if idx != nil {
							idxval = idx.Int64() + 1
						}
						idx = emitConst(fn, intConst(idxval, e)).(*Const)
					}
					iaddr := &IndexAddr{
						X:     array,
						Index: idx,
					}
					iaddr.setType(types.NewPointer(at.Elem()))
					fn.emit(iaddr, e)
					if t != at { // slice
						// backing array is unaliased => storebuf not needed.
						b.assign(fn, &address{addr: iaddr, expr: e}, e, true, nil, e)
					} else {
						b.assign(fn, &address{addr: iaddr, expr: e}, e, true, sb, e)
					}
				}
			}
		}
		if t != at { // slice
			if final != nil {
				sb.store(&address{addr: array}, final, e)
			}
			s := &Slice{X: array}
			s.setType(typ)
			sb.store(&address{addr: addr, expr: e}, fn.emit(s, e), e)
		} else if final != nil {
			sb.store(&address{addr: array, expr: e}, final, e)
		}

	case *types.Map:
		m := &MakeMap{Reserve: emitConst(fn, intConst(int64(len(e.Elts)), e))}
		m.setType(typ)
		fn.emit(m, e)
		for _, e := range e.Elts {
			e := e.(*ast.KeyValueExpr)

			// If a key expression in a map literal is  itself a
			// composite literal, the type may be omitted.
			// For example:
			//	map[*struct{}]bool{{}: true}
			// An &-operation may be implied:
			//	map[*struct{}]bool{&struct{}{}: true}
			var key Value
			if _, ok := unparen(e.Key).(*ast.CompositeLit); ok && isPointer(t.Key()) {
				// A CompositeLit never evaluates to a pointer,
				// so if the type of the location is a pointer,
				// an &-operation is implied.
				key = b.addr(fn, e.Key, true).address(fn)
			} else {
				key = b.expr(fn, e.Key)
			}

			loc := element{
				m: m,
				k: emitConv(fn, key, t.Key(), e),
				t: t.Elem(),
			}

			// We call assign() only because it takes care
			// of any &-operation required in the recursive
			// case, e.g.,
			// map[int]*struct{}{0: {}} implies &struct{}{}.
			// In-place update is of course impossible,
			// and no storebuf is needed.
			b.assign(fn, &loc, e.Value, true, nil, e)
		}
		sb.store(&address{addr: addr, expr: e}, m, e)

	default:
		panic("unexpected CompositeLit type: " + t.String())
	}
}

func (b *builder) switchStmt(fn *Function, s *ast.SwitchStmt, label *lblock) {
	if s.Tag == nil {
		b.switchStmtDynamic(fn, s, label)
		return
	}
	dynamic := false
	for _, iclause := range s.Body.List {
		clause := iclause.(*ast.CaseClause)
		for _, cond := range clause.List {
			if fn.Pkg.info.Types[unparen(cond)].Value == nil {
				dynamic = true
				break
			}
		}
	}

	if dynamic {
		b.switchStmtDynamic(fn, s, label)
		return
	}

	if s.Init != nil {
		b.stmt(fn, s.Init)
	}

	entry := fn.currentBlock
	tag := b.expr(fn, s.Tag)

	heads := make([]*BasicBlock, 0, len(s.Body.List))
	bodies := make([]*BasicBlock, len(s.Body.List))
	conds := make([]Value, 0, len(s.Body.List))

	hasDefault := false
	done := fn.newBasicBlock("switch.done")
	if label != nil {
		label._break = done
	}
	for i, stmt := range s.Body.List {
		body := fn.newBasicBlock(fmt.Sprintf("switch.body.%d", i))
		bodies[i] = body
		cas := stmt.(*ast.CaseClause)
		if cas.List == nil {
			// default branch
			hasDefault = true
			head := fn.newBasicBlock(fmt.Sprintf("switch.head.%d", i))
			conds = append(conds, nil)
			heads = append(heads, head)
			fn.currentBlock = head
			emitJump(fn, body, cas)
		}
		for j, cond := range stmt.(*ast.CaseClause).List {
			fn.currentBlock = entry
			head := fn.newBasicBlock(fmt.Sprintf("switch.head.%d.%d", i, j))
			conds = append(conds, b.expr(fn, cond))
			heads = append(heads, head)
			fn.currentBlock = head
			emitJump(fn, body, cond)
		}
	}

	for i, stmt := range s.Body.List {
		clause := stmt.(*ast.CaseClause)
		body := bodies[i]
		fn.currentBlock = body
		fallthru := done
		if i+1 < len(bodies) {
			fallthru = bodies[i+1]
		}
		fn.targets = &targets{
			tail:         fn.targets,
			_break:       done,
			_fallthrough: fallthru,
		}
		b.stmtList(fn, clause.Body)
		fn.targets = fn.targets.tail
		emitJump(fn, done, stmt)
	}

	if !hasDefault {
		head := fn.newBasicBlock("switch.head.implicit-default")
		body := fn.newBasicBlock("switch.body.implicit-default")
		fn.currentBlock = head
		emitJump(fn, body, s)
		fn.currentBlock = body
		emitJump(fn, done, s)
		heads = append(heads, head)
		conds = append(conds, nil)
	}

	if len(heads) != len(conds) {
		panic(fmt.Sprintf("internal error: %d heads for %d conds", len(heads), len(conds)))
	}
	for _, head := range heads {
		addEdge(entry, head)
	}
	fn.currentBlock = entry
	entry.emit(&ConstantSwitch{
		Tag:   tag,
		Conds: conds,
	}, s)
	fn.currentBlock = done
}

// switchStmt emits to fn code for the switch statement s, optionally
// labelled by label.
func (b *builder) switchStmtDynamic(fn *Function, s *ast.SwitchStmt, label *lblock) {
	// We treat SwitchStmt like a sequential if-else chain.
	// Multiway dispatch can be recovered later by irutil.Switches()
	// to those cases that are free of side effects.
	if s.Init != nil {
		b.stmt(fn, s.Init)
	}
	kTrue := emitConst(fn, NewConst(constant.MakeBool(true), tBool, nil))

	var tagv Value = kTrue
	var tagSource ast.Node = s
	if s.Tag != nil {
		tagv = b.expr(fn, s.Tag)
		tagSource = s.Tag
	}
	// lifting only considers loads and stores, but we want different
	// sigma nodes for the different comparisons. use a temporary and
	// load it in every branch.
	tag := emitLocal(fn, tagv.Type(), tagSource, "switch.value")
	tag.comment = "switch.tag"
	emitStore(fn, tag, tagv, tagSource)

	done := fn.newBasicBlock("switch.done")
	if label != nil {
		label._break = done
	}
	// We pull the default case (if present) down to the end.
	// But each fallthrough label must point to the next
	// body block in source order, so we preallocate a
	// body block (fallthru) for the next case.
	// Unfortunately this makes for a confusing block order.
	var dfltBody *[]ast.Stmt
	var dfltFallthrough *BasicBlock
	var fallthru, dfltBlock *BasicBlock
	ncases := len(s.Body.List)
	for i, clause := range s.Body.List {
		body := fallthru
		if body == nil {
			body = fn.newBasicBlock("switch.body") // first case only
		}

		// Preallocate body block for the next case.
		fallthru = done
		if i+1 < ncases {
			fallthru = fn.newBasicBlock("switch.body")
		}

		cc := clause.(*ast.CaseClause)
		if cc.List == nil {
			// Default case.
			dfltBody = &cc.Body
			dfltFallthrough = fallthru
			dfltBlock = body
			continue
		}

		var nextCond *BasicBlock
		for _, cond := range cc.List {
			nextCond = fn.newBasicBlock("switch.next")
			if tagv == kTrue {
				// emit a proper if/else chain instead of a comparison
				// of a value against true.
				//
				// NOTE(dh): adonovan had a todo saying "don't forget
				// conversions though". As far as I can tell, there
				// aren't any conversions that we need to take care of
				// here. `case bool(a) && bool(b)` as well as `case
				// bool(a && b)` are being taken care of by b.cond,
				// and `case a` where a is not of type bool is
				// invalid.
				b.cond(fn, cond, body, nextCond)
			} else {
				cond := emitCompare(fn, token.EQL, emitLoad(fn, tag, cond), b.expr(fn, cond), cond)
				emitIf(fn, cond, body, nextCond, cond.Source())
			}

			fn.currentBlock = nextCond
		}
		fn.currentBlock = body
		fn.targets = &targets{
			tail:         fn.targets,
			_break:       done,
			_fallthrough: fallthru,
		}
		b.stmtList(fn, cc.Body)
		fn.targets = fn.targets.tail
		emitJump(fn, done, s)
		fn.currentBlock = nextCond
	}
	if dfltBlock != nil {
		// The lack of a Source for the jump doesn't matter, block
		// fusing will get rid of the jump later.

		emitJump(fn, dfltBlock, s)
		fn.currentBlock = dfltBlock
		fn.targets = &targets{
			tail:         fn.targets,
			_break:       done,
			_fallthrough: dfltFallthrough,
		}
		b.stmtList(fn, *dfltBody)
		fn.targets = fn.targets.tail
	}
	emitJump(fn, done, s)
	fn.currentBlock = done
}

func (b *builder) typeSwitchStmt(fn *Function, s *ast.TypeSwitchStmt, label *lblock) {
	if s.Init != nil {
		b.stmt(fn, s.Init)
	}

	var tag Value
	switch e := s.Assign.(type) {
	case *ast.ExprStmt: // x.(type)
		tag = b.expr(fn, unparen(e.X).(*ast.TypeAssertExpr).X)
	case *ast.AssignStmt: // y := x.(type)
		tag = b.expr(fn, unparen(e.Rhs[0]).(*ast.TypeAssertExpr).X)
	default:
		panic("unreachable")
	}
	tagPtr := emitLocal(fn, tag.Type(), tag.Source(), "")
	emitStore(fn, tagPtr, tag, tag.Source())

	// +1 in case there's no explicit default case
	heads := make([]*BasicBlock, 0, len(s.Body.List)+1)

	entry := fn.currentBlock
	done := fn.newBasicBlock("done")
	if label != nil {
		label._break = done
	}

	// set up type switch and constant switch, populate their conditions
	tswtch := &TypeSwitch{
		Tag:   emitLoad(fn, tagPtr, tag.Source()),
		Conds: make([]types.Type, 0, len(s.Body.List)+1),
	}
	cswtch := &ConstantSwitch{
		Conds: make([]Value, 0, len(s.Body.List)+1),
	}

	rets := make([]types.Type, 0, len(s.Body.List)+1)
	index := 0
	var default_ *ast.CaseClause
	for _, clause := range s.Body.List {
		cc := clause.(*ast.CaseClause)
		if obj, ok := fn.Pkg.info.Implicits[cc].(*types.Var); ok {
			emitLocalVar(fn, obj, cc)
		}
		if cc.List == nil {
			// default case
			default_ = cc
		} else {
			for _, expr := range cc.List {
				tswtch.Conds = append(tswtch.Conds, fn.Pkg.typeOf(expr))
				cswtch.Conds = append(cswtch.Conds, emitConst(fn, intConst(int64(index), expr)))
				index++
			}
			if len(cc.List) == 1 {
				rets = append(rets, fn.Pkg.typeOf(cc.List[0]))
			} else {
				for range cc.List {
					rets = append(rets, tag.Type())
				}
			}
		}
	}

	// default branch
	rets = append(rets, tag.Type())

	var vars []*types.Var
	vars = append(vars, varIndex)
	for _, typ := range rets {
		vars = append(vars, anonVar(typ))
	}
	tswtch.setType(types.NewTuple(vars...))
	// default branch
	fn.currentBlock = entry
	fn.emit(tswtch, s)
	cswtch.Conds = append(cswtch.Conds, emitConst(fn, intConst(int64(-1), nil)))
	// in theory we should add a local and stores/loads for tswtch, to
	// generate sigma nodes in the branches. however, there isn't any
	// useful information we could possibly attach to it.
	cswtch.Tag = emitExtract(fn, tswtch, 0, s)
	fn.emit(cswtch, s)

	// build heads and bodies
	index = 0
	for _, clause := range s.Body.List {
		cc := clause.(*ast.CaseClause)
		if cc.List == nil {
			continue
		}

		body := fn.newBasicBlock("typeswitch.body")
		for _, expr := range cc.List {
			head := fn.newBasicBlock("typeswitch.head")
			heads = append(heads, head)
			fn.currentBlock = head

			if obj, ok := fn.Pkg.info.Implicits[cc].(*types.Var); ok {
				// In a switch y := x.(type), each case clause
				// implicitly declares a distinct object y.
				// In a single-type case, y has that type.
				// In multi-type cases, 'case nil' and default,
				// y has the same type as the interface operand.

				l := fn.vars[obj]
				if rets[index] == tUntypedNil {
					emitStore(fn, l, emitConst(fn, nilConst(tswtch.Tag.Type(), nil)), s.Assign)
				} else {
					x := emitExtract(fn, tswtch, index+1, s.Assign)
					emitStore(fn, l, x, nil)
				}
			}

			emitJump(fn, body, expr)
			index++
		}
		fn.currentBlock = body
		fn.targets = &targets{
			tail:   fn.targets,
			_break: done,
		}
		b.stmtList(fn, cc.Body)
		fn.targets = fn.targets.tail
		emitJump(fn, done, clause)
	}

	if default_ == nil {
		// implicit default
		heads = append(heads, done)
	} else {
		body := fn.newBasicBlock("typeswitch.default")
		heads = append(heads, body)
		fn.currentBlock = body
		fn.targets = &targets{
			tail:   fn.targets,
			_break: done,
		}
		if obj, ok := fn.Pkg.info.Implicits[default_].(*types.Var); ok {
			l := fn.vars[obj]
			x := emitExtract(fn, tswtch, index+1, s.Assign)
			emitStore(fn, l, x, s)
		}
		b.stmtList(fn, default_.Body)
		fn.targets = fn.targets.tail
		emitJump(fn, done, s)
	}

	fn.currentBlock = entry
	for _, head := range heads {
		addEdge(entry, head)
	}
	fn.currentBlock = done
}

// selectStmt emits to fn code for the select statement s, optionally
// labelled by label.
func (b *builder) selectStmt(fn *Function, s *ast.SelectStmt, label *lblock) (noreturn bool) {
	if len(s.Body.List) == 0 {
		instr := &Select{Blocking: true}
		instr.setType(types.NewTuple(varIndex, varOk))
		fn.emit(instr, s)
		fn.emit(new(Unreachable), s)
		addEdge(fn.currentBlock, fn.Exit)
		return true
	}

	// A blocking select of a single case degenerates to a
	// simple send or receive.
	// TODO(adonovan): opt: is this optimization worth its weight?
	if len(s.Body.List) == 1 {
		clause := s.Body.List[0].(*ast.CommClause)
		if clause.Comm != nil {
			b.stmt(fn, clause.Comm)
			done := fn.newBasicBlock("select.done")
			if label != nil {
				label._break = done
			}
			fn.targets = &targets{
				tail:   fn.targets,
				_break: done,
			}
			b.stmtList(fn, clause.Body)
			fn.targets = fn.targets.tail
			emitJump(fn, done, clause)
			fn.currentBlock = done
			return false
		}
	}

	// First evaluate all channels in all cases, and find
	// the directions of each state.
	var states []*SelectState
	blocking := true
	debugInfo := fn.debugInfo()
	for _, clause := range s.Body.List {
		var st *SelectState
		switch comm := clause.(*ast.CommClause).Comm.(type) {
		case nil: // default case
			blocking = false
			continue

		case *ast.SendStmt: // ch<- i
			ch := b.expr(fn, comm.Chan)
			st = &SelectState{
				Dir:  types.SendOnly,
				Chan: ch,
				Send: emitConv(fn, b.expr(fn, comm.Value),
					typeutil.CoreType(ch.Type()).Underlying().(*types.Chan).Elem(), comm),
				Pos: comm.Arrow,
			}
			if debugInfo {
				st.DebugNode = comm
			}

		case *ast.AssignStmt: // x := <-ch
			recv := unparen(comm.Rhs[0]).(*ast.UnaryExpr)
			st = &SelectState{
				Dir:  types.RecvOnly,
				Chan: b.expr(fn, recv.X),
				Pos:  recv.OpPos,
			}
			if debugInfo {
				st.DebugNode = recv
			}

		case *ast.ExprStmt: // <-ch
			recv := unparen(comm.X).(*ast.UnaryExpr)
			st = &SelectState{
				Dir:  types.RecvOnly,
				Chan: b.expr(fn, recv.X),
				Pos:  recv.OpPos,
			}
			if debugInfo {
				st.DebugNode = recv
			}
		}
		states = append(states, st)
	}

	// We dispatch on the (fair) result of Select using a
	// switch on the returned index.
	sel := &Select{
		States:   states,
		Blocking: blocking,
	}
	sel.source = s
	var vars []*types.Var
	vars = append(vars, varIndex, varOk)
	for _, st := range states {
		if st.Dir == types.RecvOnly {
			tElem := typeutil.CoreType(st.Chan.Type()).Underlying().(*types.Chan).Elem()
			vars = append(vars, anonVar(tElem))
		}
	}
	sel.setType(types.NewTuple(vars...))
	fn.emit(sel, s)
	idx := emitExtract(fn, sel, 0, s)

	done := fn.newBasicBlock("select.done")
	if label != nil {
		label._break = done
	}

	entry := fn.currentBlock
	swtch := &ConstantSwitch{
		Tag: idx,
		// one condition per case
		Conds: make([]Value, 0, len(s.Body.List)+1),
	}
	// note that we don't need heads; a select case can only have a single condition
	var bodies []*BasicBlock

	state := 0
	r := 2 // index in 'sel' tuple of value; increments if st.Dir==RECV
	for _, cc := range s.Body.List {
		clause := cc.(*ast.CommClause)
		if clause.Comm == nil {
			body := fn.newBasicBlock("select.default")
			fn.currentBlock = body
			bodies = append(bodies, body)
			fn.targets = &targets{
				tail:   fn.targets,
				_break: done,
			}
			b.stmtList(fn, clause.Body)
			emitJump(fn, done, s)
			fn.targets = fn.targets.tail
			swtch.Conds = append(swtch.Conds, emitConst(fn, intConst(-1, nil)))
			continue
		}
		swtch.Conds = append(swtch.Conds, emitConst(fn, intConst(int64(state), nil)))
		body := fn.newBasicBlock("select.body")
		fn.currentBlock = body
		bodies = append(bodies, body)
		fn.targets = &targets{
			tail:   fn.targets,
			_break: done,
		}
		switch comm := clause.Comm.(type) {
		case *ast.ExprStmt: // <-ch
			if debugInfo {
				v := emitExtract(fn, sel, r, comm)
				emitDebugRef(fn, states[state].DebugNode.(ast.Expr), v, false)
			}
			r++

		case *ast.AssignStmt: // x := <-states[state].Chan
			if comm.Tok == token.DEFINE {
				id := comm.Lhs[0].(*ast.Ident)
				emitLocalVar(fn, identVar(fn, id), id)
			}
			x := b.addr(fn, comm.Lhs[0], false) // non-escaping
			v := emitExtract(fn, sel, r, comm)
			if debugInfo {
				emitDebugRef(fn, states[state].DebugNode.(ast.Expr), v, false)
			}
			x.store(fn, v, comm)

			if len(comm.Lhs) == 2 { // x, ok := ...
				if comm.Tok == token.DEFINE {
					id := comm.Lhs[1].(*ast.Ident)
					emitLocalVar(fn, identVar(fn, id), id)
				}
				ok := b.addr(fn, comm.Lhs[1], false) // non-escaping
				ok.store(fn, emitExtract(fn, sel, 1, comm), comm)
			}
			r++
		}
		b.stmtList(fn, clause.Body)
		fn.targets = fn.targets.tail
		emitJump(fn, done, s)
		state++
	}
	fn.currentBlock = entry
	fn.emit(swtch, s)
	for _, body := range bodies {
		addEdge(entry, body)
	}
	fn.currentBlock = done
	return false
}

// forStmt emits to fn code for the for statement s, optionally
// labelled by label.
func (b *builder) forStmt(fn *Function, s *ast.ForStmt, label *lblock) {
	// Use forStmtGo122 instead if it applies.
	if s.Init != nil {
		if assign, ok := s.Init.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
			if version.Compare(fn.goversion, "go1.22") >= 0 {
				b.forStmtGo122(fn, s, label)
				return
			}
		}
	}

	//	...init...
	//      jump loop
	// loop:
	//      if cond goto body else done
	// body:
	//      ...body...
	//      jump post
	// post:				 (target of continue)
	//      ...post...
	//      jump loop
	// done:                                 (target of break)
	if s.Init != nil {
		b.stmt(fn, s.Init)
	}
	body := fn.newBasicBlock("for.body")
	done := fn.newBasicBlock("for.done") // target of 'break'
	loop := body                         // target of back-edge
	if s.Cond != nil {
		loop = fn.newBasicBlock("for.loop")
	}
	cont := loop // target of 'continue'
	if s.Post != nil {
		cont = fn.newBasicBlock("for.post")
	}
	if label != nil {
		label._break = done
		label._continue = cont
	}
	emitJump(fn, loop, s)
	fn.currentBlock = loop
	if loop != body {
		b.cond(fn, s.Cond, body, done)
		fn.currentBlock = body
	}
	fn.targets = &targets{
		tail:      fn.targets,
		_break:    done,
		_continue: cont,
	}
	b.stmt(fn, s.Body)
	fn.targets = fn.targets.tail
	emitJump(fn, cont, s)

	if s.Post != nil {
		fn.currentBlock = cont
		b.stmt(fn, s.Post)
		emitJump(fn, loop, s) // back-edge
	}
	fn.currentBlock = done
}

// forStmtGo122 emits to fn code for the for statement s, optionally
// labelled by label. s must define its variables.
//
// This allocates once per loop iteration. This is only correct in
// GoVersions >= go1.22.
func (b *builder) forStmtGo122(fn *Function, s *ast.ForStmt, label *lblock) {
	//     i_outer = alloc[T]
	//     *i_outer = ...init...        // under objects[i] = i_outer
	//     jump loop
	// loop:
	//     i = phi [head: i_outer, loop: i_next]
	//     ...cond...                   // under objects[i] = i
	//     if cond goto body else done
	// body:
	//     ...body...                   // under objects[i] = i (same as loop)
	//     jump post
	// post:
	//     tmp = *i
	//     i_next = alloc[T]
	//     *i_next = tmp
	//     ...post...                   // under objects[i] = i_next
	//     goto loop
	// done:

	init := s.Init.(*ast.AssignStmt)
	startingBlocks := len(fn.Blocks)

	pre := fn.currentBlock               // current block before starting
	loop := fn.newBasicBlock("for.loop") // target of back-edge
	body := fn.newBasicBlock("for.body")
	post := fn.newBasicBlock("for.post") // target of 'continue'
	done := fn.newBasicBlock("for.done") // target of 'break'

	// For each of the n loop variables, we create five SSA values,
	// outer, phi, next, load, and store in pre, loop, and post.
	// There is no limit on n.
	type loopVar struct {
		obj   *types.Var
		outer *Alloc
		phi   *Phi
		load  *Load
		next  *Alloc
		store *Store
	}
	vars := make([]loopVar, len(init.Lhs))
	for i, lhs := range init.Lhs {
		v := identVar(fn, lhs.(*ast.Ident))

		fn.currentBlock = pre
		outer := emitLocal(fn, v.Type(), lhs, v.Name())

		fn.currentBlock = loop
		phi := &Phi{}
		phi.comment = v.Name()
		phi.typ = outer.Type()
		fn.emit(phi, lhs)

		fn.currentBlock = post
		// If next is local, it reuses the address and zeroes the old value so
		// load before allocating next.
		load := emitLoad(fn, phi, init)
		next := emitLocal(fn, v.Type(), lhs, v.Name())
		store := emitStore(fn, next, load, s)

		phi.Edges = []Value{outer, next} // pre edge is emitted before post edge.
		vars[i] = loopVar{v, outer, phi, load, next, store}
	}

	// ...init... under fn.objects[v] = i_outer
	fn.currentBlock = pre
	for _, v := range vars {
		fn.vars[v.obj] = v.outer
	}
	const isDef = false // assign to already-allocated outers
	b.assignStmt(fn, init.Lhs, init.Rhs, isDef, s)
	if label != nil {
		label._break = done
		label._continue = post
	}
	emitJump(fn, loop, s)

	// ...cond... under fn.objects[v] = i
	fn.currentBlock = loop
	for _, v := range vars {
		fn.vars[v.obj] = v.phi
	}
	if s.Cond != nil {
		b.cond(fn, s.Cond, body, done)
	} else {
		emitJump(fn, body, s)
	}

	// ...body... under fn.objects[v] = i
	fn.currentBlock = body
	fn.targets = &targets{
		tail:      fn.targets,
		_break:    done,
		_continue: post,
	}
	b.stmt(fn, s.Body)
	fn.targets = fn.targets.tail
	emitJump(fn, post, s)

	// ...post... under fn.objects[v] = i_next
	for _, v := range vars {
		fn.vars[v.obj] = v.next
	}
	fn.currentBlock = post
	if s.Post != nil {
		b.stmt(fn, s.Post)
	}
	emitJump(fn, loop, s) // back-edge
	fn.currentBlock = done

	// For each loop variable that does not escape,
	// (the common case), fuse its next cells into its
	// (local) outer cell as they have disjoint live ranges.
	//
	// It is sufficient to test whether i_next escapes,
	// because its Heap flag will be marked true if either
	// the cond or post expression causes i to escape
	// (because escape distributes over phi).
	var nlocals int
	for _, v := range vars {
		if !v.next.Heap {
			nlocals++
		}
	}
	if nlocals > 0 {
		replace := make(map[Value]Value, 2*nlocals)
		dead := make(map[Instruction]bool, 4*nlocals)
		for _, v := range vars {
			if !v.next.Heap {
				replace[v.next] = v.outer
				replace[v.phi] = v.outer
				dead[v.phi], dead[v.next], dead[v.load], dead[v.store] = true, true, true, true
			}
		}

		// Replace all uses of i_next and phi with i_outer.
		// Referrers have not been built for fn yet so only update Instruction operands.
		// We need only look within the blocks added by the loop.
		var operands []*Value // recycle storage
		for _, b := range fn.Blocks[startingBlocks:] {
			for _, instr := range b.Instrs {
				operands = instr.Operands(operands[:0])
				for _, ptr := range operands {
					k := *ptr
					if v := replace[k]; v != nil {
						*ptr = v
					}
				}
			}
		}

		// Remove instructions for phi, load, and store.
		// lift() will remove the unused i_next *Alloc.
		isDead := func(i Instruction) bool { return dead[i] }
		loop.Instrs = removeInstrsIf(loop.Instrs, isDead)
		post.Instrs = removeInstrsIf(post.Instrs, isDead)
	}
}

// rangeIndexed emits to fn the header for an integer-indexed loop
// over array, *array or slice value x.
// The v result is defined only if tv is non-nil.
// forPos is the position of the "for" token.
func (b *builder) rangeIndexed(fn *Function, x Value, tv types.Type, source ast.Node) (k, v Value, loop, done *BasicBlock) {
	//
	//      length = len(x)
	//      index = -1
	// loop:                                   (target of continue)
	//      index++
	// 	if index < length goto body else done
	// body:
	//      k = index
	//      v = x[index]
	//      ...body...
	// 	jump loop
	// done:                                   (target of break)

	// We store in an Alloc and load it on each iteration so that lifting produces the necessary σ nodes
	xAlloc := newVariable(fn, x.Type(), source)
	xAlloc.store(x)

	// Determine number of iterations.
	//
	// We store the length in an Alloc and load it on each iteration so that lifting produces the necessary σ nodes
	length := newVariable(fn, tInt, source)
	if arr, ok := typeutil.CoreType(deref(x.Type())).(*types.Array); ok {
		// For array or *array, the number of iterations is known statically thanks to the type. We avoid a data
		// dependence upon x, permitting later dead-code elimination if x is pure, static unrolling, etc. Ranging over a
		// nil *array may have >0 iterations. We still generate code for x, in case it has effects.
		//
		// We use the core type of x, even though the length of type parameters isn't constant as per the language
		// specification. Just because len(x) isn't constant doesn't mean we can't emit IR that takes advantage of a
		// known length.
		length.store(emitConst(fn, intConst(arr.Len(), nil)))
	} else {
		// length = len(x).
		var c Call
		c.Call.Value = makeLen(x.Type())
		c.Call.Args = []Value{x}
		c.setType(tInt)
		length.store(fn.emit(&c, source))
	}

	index := emitLocal(fn, tInt, source, "rangeindex")
	emitStore(fn, index, emitConst(fn, intConst(-1, nil)), source)

	loop = fn.newBasicBlock("rangeindex.loop")
	emitJump(fn, loop, source)
	fn.currentBlock = loop

	incr := &BinOp{
		Op: token.ADD,
		X:  emitLoad(fn, index, source),
		Y:  emitConst(fn, intConst(1, nil)),
	}
	incr.setType(tInt)
	emitStore(fn, index, fn.emit(incr, source), source)

	body := fn.newBasicBlock("rangeindex.body")
	done = fn.newBasicBlock("rangeindex.done")
	emitIf(fn, emitCompare(fn, token.LSS, incr, length.load(), source), body, done, source)
	fn.currentBlock = body

	k = emitLoad(fn, index, source)
	if tv != nil {
		x := xAlloc.load()
		switch t := typeutil.CoreType(x.Type()).Underlying().(type) {
		case *types.Array:
			instr := &Index{
				X:     x,
				Index: k,
			}
			instr.setType(t.Elem())
			v = fn.emit(instr, source)

		case *types.Pointer: // *array
			instr := &IndexAddr{
				X:     x,
				Index: k,
			}
			instr.setType(types.NewPointer(t.Elem().Underlying().(*types.Array).Elem()))
			v = emitLoad(fn, fn.emit(instr, source), source)

		case *types.Slice:
			instr := &IndexAddr{
				X:     x,
				Index: k,
			}
			instr.setType(types.NewPointer(t.Elem()))
			v = emitLoad(fn, fn.emit(instr, source), source)

		default:
			panic("rangeIndexed x:" + t.String())
		}
	}
	return
}

// rangeIter emits to fn the header for a loop using
// Range/Next/Extract to iterate over map or string value x.
// tk and tv are the types of the key/value results k and v, or nil
// if the respective component is not wanted.
func (b *builder) rangeIter(fn *Function, x Value, tk, tv types.Type, source ast.Node) (k, v Value, loop, done *BasicBlock) {
	//
	//	it = range x
	// loop:                                   (target of continue)
	//	okv = next it                      (ok, key, value)
	//  	ok = extract okv #0
	// 	if ok goto body else done
	// body:
	// 	k = extract okv #1
	// 	v = extract okv #2
	//      ...body...
	// 	jump loop
	// done:                                   (target of break)
	//

	if tk == nil {
		tk = tInvalid
	}
	if tv == nil {
		tv = tInvalid
	}

	rng := &Range{X: x}
	rng.setType(typeutil.NewIterator(types.NewTuple(
		varOk,
		newVar("k", tk),
		newVar("v", tv),
	)))
	it := newVariable(fn, rng.typ, source)
	it.store(fn.emit(rng, source))

	loop = fn.newBasicBlock("rangeiter.loop")
	emitJump(fn, loop, source)
	fn.currentBlock = loop

	// Go doesn't currently allow ranging over string|[]byte, so isString is decidable.
	_, isString := typeutil.CoreType(x.Type()).Underlying().(*types.Basic)

	okvInstr := &Next{
		Iter:     it.load(),
		IsString: isString,
	}
	okvInstr.setType(rng.typ.(*typeutil.Iterator).Elem())
	fn.emit(okvInstr, source)
	okv := newVariable(fn, okvInstr.Type(), source)
	okv.store(okvInstr)

	body := fn.newBasicBlock("rangeiter.body")
	done = fn.newBasicBlock("rangeiter.done")
	emitIf(fn, emitExtract(fn, okv.load(), 0, source), body, done, source)
	fn.currentBlock = body

	if tk != tInvalid {
		k = emitExtract(fn, okv.load(), 1, source)
	}
	if tv != tInvalid {
		v = emitExtract(fn, okv.load(), 2, source)
	}
	return
}

// rangeChan emits to fn the header for a loop that receives from
// channel x until it fails.
// tk is the channel's element type, or nil if the k result is
// not wanted
// pos is the position of the '=' or ':=' token.
func (b *builder) rangeChan(fn *Function, x Value, tk types.Type, source ast.Node) (k Value, loop, done *BasicBlock) {
	//
	// loop:                                   (target of continue)
	//      ko = <-x                           (key, ok)
	//      ok = extract ko #1
	//      if ok goto body else done
	// body:
	//      k = extract ko #0
	//      ...
	//      goto loop
	// done:                                   (target of break)

	loop = fn.newBasicBlock("rangechan.loop")
	emitJump(fn, loop, source)
	fn.currentBlock = loop

	recv := emitRecv(fn, x, true, types.NewTuple(newVar("k", typeutil.CoreType(x.Type()).Underlying().(*types.Chan).Elem()), varOk), source)
	retv := newVariable(fn, recv.Type(), source)
	retv.store(recv)

	body := fn.newBasicBlock("rangechan.body")
	done = fn.newBasicBlock("rangechan.done")
	emitIf(fn, emitExtract(fn, retv.load(), 1, source), body, done, source)
	fn.currentBlock = body
	if tk != nil {
		k = emitExtract(fn, retv.load(), 0, source)
	}
	return
}

// rangeInt emits to fn the header for a range loop with an integer operand.
// tk is the key value's type, or nil if the k result is not wanted.
// pos is the position of the "for" token.
func (b *builder) rangeInt(fn *Function, x Value, tk types.Type, source ast.Node) (k Value, loop, done *BasicBlock) {
	//
	//     iter = 0
	//     if 0 < x goto body else done
	// loop:                                   (target of continue)
	//     iter++
	//     if iter < x goto body else done
	// body:
	//     k = x
	//     ...body...
	//     jump loop
	// done:                                   (target of break)

	if b, ok := x.Type().(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
		x = emitConv(fn, x, tInt, source)
	}

	T := x.Type()
	iter := emitLocal(fn, T, source, "rangeint.iter")
	// x may be unsigned. Avoid initializing x to -1.

	body := fn.newBasicBlock("rangeint.body")
	done = fn.newBasicBlock("rangeint.done")
	emitIf(fn, emitCompare(fn, token.LSS, emitConst(fn, zeroConst(T, source)), x, source), body, done, source)

	loop = fn.newBasicBlock("rangeint.loop")
	fn.currentBlock = loop

	incr := &BinOp{
		Op: token.ADD,
		X:  emitLoad(fn, iter, source),
		Y:  emitConv(fn, emitConst(fn, intConst(1, source)), T, source),
	}
	incr.setType(T)
	emitStore(fn, iter, fn.emit(incr, source), source)
	emitIf(fn, emitCompare(fn, token.LSS, incr, x, source), body, done, source)
	fn.currentBlock = body

	if tk != nil {
		// Integer types (int, uint8, etc.) are named and
		// we know that k is assignable to x when tk != nil.
		// This implies tk and T are identical so no conversion is needed.
		k = emitLoad(fn, iter, source)
	}

	return
}

type variable struct {
	alloc  *Alloc
	fn     *Function
	source ast.Node
}

func newVariable(fn *Function, typ types.Type, source ast.Node) *variable {
	alloc := &Alloc{}
	alloc.setType(types.NewPointer(typ))
	fn.emit(alloc, source)
	fn.Locals = append(fn.Locals, alloc)
	return &variable{
		alloc:  alloc,
		fn:     fn,
		source: source,
	}
}

func (v *variable) store(sv Value) {
	emitStore(v.fn, v.alloc, sv, v.source)
}

func (v *variable) load() Value {
	return emitLoad(v.fn, v.alloc, v.source)
}

// rangeStmt emits to fn code for the range statement s, optionally
// labelled by label.
func (b *builder) rangeStmt(fn *Function, s *ast.RangeStmt, label *lblock, source ast.Node) {
	var tk, tv types.Type
	if s.Key != nil && !isBlankIdent(s.Key) {
		tk = fn.Pkg.typeOf(s.Key)
	}
	if s.Value != nil && !isBlankIdent(s.Value) {
		tv = fn.Pkg.typeOf(s.Value)
	}

	// create locals for s.Key and s.Value
	createVars := func() {
		// Unlike a short variable declaration, a RangeStmt
		// using := never redeclares an existing variable; it
		// always creates a new one.
		if tk != nil {
			id := s.Key.(*ast.Ident)
			emitLocalVar(fn, identVar(fn, id), id)
		}
		if tv != nil {
			id := s.Value.(*ast.Ident)
			emitLocalVar(fn, identVar(fn, id), id)
		}
	}

	afterGo122 := version.Compare(fn.goversion, "go1.22") >= 0

	if s.Tok == token.DEFINE && !afterGo122 {
		// pre-go1.22: If iteration variables are defined (:=), this
		// occurs once outside the loop.
		createVars()
	}

	x := b.expr(fn, s.X)

	var k, v Value
	var loop, done *BasicBlock
	switch rt := typeutil.CoreType(x.Type()).Underlying().(type) {
	case *types.Slice, *types.Array, *types.Pointer: // *array
		k, v, loop, done = b.rangeIndexed(fn, x, tv, source)

	case *types.Chan:
		k, loop, done = b.rangeChan(fn, x, tk, source)

	case *types.Map:
		k, v, loop, done = b.rangeIter(fn, x, tk, tv, source)

	case *types.Basic:
		switch {
		case rt.Info()&types.IsString != 0:
			k, v, loop, done = b.rangeIter(fn, x, tk, tv, source)

		case rt.Info()&types.IsInteger != 0:
			k, loop, done = b.rangeInt(fn, x, tk, source)

		default:
			panic("Cannot range over basic type: " + rt.String())
		}

	case *types.Signature:
		// Special case rewrite (fn.goversion >= go1.23):
		//      for x := range f { ... }
		// into
		//      f(func(x T) bool { ... })
		b.rangeFunc(fn, x, tk, tv, s, label)
		return

	default:
		panic("Cannot range over: " + rt.String())
	}

	if s.Tok == token.DEFINE && afterGo122 {
		// go1.22: If iteration variables are defined (:=), this occurs inside the loop.
		createVars()
	}

	// Evaluate both LHS expressions before we update either.
	var kl, vl lvalue
	if tk != nil {
		kl = b.addr(fn, s.Key, false) // non-escaping
	}
	if tv != nil {
		vl = b.addr(fn, s.Value, false) // non-escaping
	}
	if tk != nil {
		kl.store(fn, k, s)
	}
	if tv != nil {
		vl.store(fn, v, s)
	}

	if label != nil {
		label._break = done
		label._continue = loop
	}

	fn.targets = &targets{
		tail:      fn.targets,
		_break:    done,
		_continue: loop,
	}
	b.stmt(fn, s.Body)
	fn.targets = fn.targets.tail
	emitJump(fn, loop, source) // back-edge
	fn.currentBlock = done
}

// rangeFunc emits to fn code for the range-over-func rng.Body of the iterator
// function x, optionally labelled by label. It creates a new anonymous function
// yield for rng and builds the function.
func (b *builder) rangeFunc(fn *Function, x Value, tk, tv types.Type, rng *ast.RangeStmt, label *lblock) {
	// Consider the SSA code for the outermost range-over-func in fn:
	//
	//   func fn(...) (ret R) {
	//     ...
	//     for k, v = range x {
	//           ...
	//     }
	//     ...
	//   }
	//
	// The code emitted into fn will look something like this.
	//
	// loop:
	//     jump := READY
	//     y := make closure yield [ret, deferstack, jump, k, v]
	//     x(y)
	//     switch jump {
	//        [see resuming execution]
	//     }
	//     goto done
	// done:
	//     ...
	//
	// where yield is a new synthetic yield function:
	//
	// func yield(_k tk, _v tv) bool
	//   free variables: [ret, stack, jump, k, v]
	// {
	//    entry:
	//      if jump != READY then goto invalid else valid
	//    invalid:
	//      panic("iterator called when it is not in a ready state")
	//    valid:
	//      jump = BUSY
	//      k = _k
	//      v = _v
	//    ...
	//    cont:
	//      jump = READY
	//      return true
	// }
	//
	// Yield state:
	//
	// Each range loop has an associated jump variable that records
	// the state of the iterator. A yield function is initially
	// in a READY (0) and callable state.  If the yield function is called
	// and is not in READY state, it panics. When it is called in a callable
	// state, it becomes BUSY. When execution reaches the end of the body
	// of the loop (or a continue statement targeting the loop is executed),
	// the yield function returns true and resumes being in a READY state.
	// After the iterator function x(y) returns, then if the yield function
	// is in a READY state, the yield enters the DONE state.
	//
	// Each lowered control statement (break X, continue X, goto Z, or return)
	// that exits the loop sets the variable to a unique positive EXIT value,
	// before returning false from the yield function.
	//
	// If the yield function returns abruptly due to a panic or GoExit,
	// it remains in a BUSY state. The generated code asserts that, after
	// the iterator call x(y) returns normally, the jump variable state
	// is DONE.
	//
	// Resuming execution:
	//
	// The code generated for the range statement checks the jump
	// variable to determine how to resume execution.
	//
	//    switch jump {
	//    case BUSY:  panic("...")
	//    case DONE:  goto done
	//    case READY: state = DONE; goto done
	//    case 123:   ... // action for exit 123.
	//    case 456:   ... // action for exit 456.
	//    ...
	//    }
	//
	// Forward goto statements within a yield are jumps to labels that
	// have not yet been traversed in fn. They may be in the Body of the
	// function. What we emit for these is:
	//
	//    goto target
	//  target:
	//    ...
	//
	// We leave an unresolved exit in yield.exits to check at the end
	// of building yield if it encountered target in the body. If it
	// encountered target, no additional work is required. Otherwise,
	// the yield emits a new early exit in the basic block for target.
	// We expect that blockopt will fuse the early exit into the case
	// block later. The unresolved exit is then added to yield.parent.exits.

	loop := fn.newBasicBlock("rangefunc.loop")
	done := fn.newBasicBlock("rangefunc.done")

	// These are targets within y.
	fn.targets = &targets{
		tail:   fn.targets,
		_break: done,
		// _continue is within y.
	}
	if label != nil {
		label._break = done
		// _continue is within y
	}

	emitJump(fn, loop, nil)
	fn.currentBlock = loop

	// loop:
	//     jump := READY

	anonIdx := len(fn.AnonFuncs)

	jump := newVar(fmt.Sprintf("jump$%d", anonIdx+1), tInt)
	emitLocalVar(fn, jump, nil) // zero value is READY

	xsig := typeutil.CoreType(x.Type()).(*types.Signature)
	ysig := typeutil.CoreType(xsig.Params().At(0).Type()).(*types.Signature)

	/* synthetic yield function for body of range-over-func loop */
	y := &Function{
		name:         fmt.Sprintf("%s$%d", fn.Name(), anonIdx+1),
		Signature:    ysig,
		Synthetic:    SyntheticRangeOverFuncYield,
		parent:       fn,
		Pkg:          fn.Pkg,
		Prog:         fn.Prog,
		functionBody: new(functionBody),
	}
	y.source = rng
	y.goversion = fn.goversion
	y.jump = jump
	y.deferstack = fn.deferstack
	y.returnVars = fn.returnVars // use the parent's return variables
	y.uniq = fn.uniq             // start from parent's unique values

	// If the RangeStmt has a label, this is how it is passed to buildYieldFunc.
	if label != nil {
		y.lblocks = map[*types.Label]*lblock{label.label: nil}
	}
	fn.AnonFuncs = append(fn.AnonFuncs, y)

	// Build y immediately. It may:
	// * cause fn's locals to escape, and
	// * create new exit nodes in exits.
	// (y is not marked 'built' until the end of the enclosing FuncDecl.)
	unresolved := len(fn.exits)
	b.buildYieldFunc(y)
	fn.uniq = y.uniq // resume after y's unique values

	// Emit the call of y.
	//   c := MakeClosure y
	//   x(c)
	c := &MakeClosure{Fn: y}
	c.setType(ysig)
	c.comment = "yield"
	for _, fv := range y.FreeVars {
		c.Bindings = append(c.Bindings, fv.outer)
		fv.outer = nil
	}
	fn.emit(c, nil)
	call := Call{
		Call: CallCommon{
			Value: x,
			Args:  []Value{c},
		},
	}
	call.setType(xsig.Results())
	fn.emit(&call, nil)

	exits := fn.exits[unresolved:]
	b.buildYieldResume(fn, jump, exits, done)

	fn.currentBlock = done
	// pop the stack for the range-over-func
	fn.targets = fn.targets.tail
}

// buildYieldResume emits to fn code for how to resume execution once a call to
// the iterator function over the yield function returns x(y). It does this by building
// a switch over the value of jump for when it is READY, BUSY, or EXIT(id).
func (b *builder) buildYieldResume(fn *Function, jump *types.Var, exits []*exit, done *BasicBlock) {
	//    v := *jump
	//    switch v {
	//    case BUSY:    panic("...")
	//    case READY:   jump = DONE; goto done
	//    case EXIT(a): ...
	//    case EXIT(b): ...
	//    ...
	//    }
	v := emitLoad(fn, fn.lookup(jump, false), nil)

	entry := fn.currentBlock
	bodies := make([]*BasicBlock, 2, 2+len(exits))
	bodies[0] = fn.newBasicBlock("rangefunc.resume.busy")
	bodies[1] = fn.newBasicBlock("rangefunc.resume.ready")

	conds := make([]Value, 2, 2+len(exits))
	conds[0] = emitConst(fn, jBusy())
	conds[1] = emitConst(fn, jReady())

	fn.currentBlock = bodies[0]
	fn.emit(
		&Panic{
			X: emitConv(fn, emitConst(fn, stringConst("iterator call did not preserve panic", nil)), tEface, nil),
		},
		nil,
	)
	addEdge(fn.currentBlock, fn.Exit)

	fn.currentBlock = bodies[1]
	storeVar(fn, jump, emitConst(fn, jDone()), nil)
	emitJump(fn, done, nil)

	for _, e := range exits {
		body := fn.newBasicBlock(fmt.Sprintf("rangefunc.resume.exit.%d", e.id))
		bodies = append(bodies, body)
		id_ := intConst(e.id, nil)
		id_.comment = fmt.Sprintf("rangefunc.exit.%d", e.id)
		id := emitConst(fn, id_)
		conds = append(conds, id)

		fn.currentBlock = body
		switch {
		case e.label != nil: // forward goto?
			// case EXIT(id): goto lb // label
			lb := fn.lblockOf(e.label)
			// Do not mark lb as resolved.
			// If fn does not contain label, lb remains unresolved and
			// fn must itself be a range-over-func function. lb will be:
			//   lb:
			//     fn.jump = id
			//     return false
			emitJump(fn, lb._goto, e.source)

		case e.to != fn: // e jumps to an ancestor of fn?
			// case EXIT(id): { fn.jump = id; return false }
			// fn is a range-over-func function.

			storeVar(fn, fn.jump, id, e.source)
			vFalse := emitConst(fn, NewConst(constant.MakeBool(false), tBool, e.source))
			emitReturn(fn, []Value{vFalse}, e.source)

		case e.block == nil && e.label == nil: // return from fn?
			// case EXIT(id): { return ... }

			// The results have already been stored to variables in fn.results, so
			// emitReturn doesn't have to do it again.
			emitReturn(fn, nil, e.source)

		case e.block != nil:
			// case EXIT(id): goto block
			emitJump(fn, e.block, e.source)

		default:
			panic("unreachable")
		}

	}

	fn.currentBlock = entry
	// Note that this switch does not have an implicit default case. This wouldn't be
	// valid for a user-provided switch statement, but for range-over-func we know all
	// possible values and we can avoid the impossible branch.
	swtch := &ConstantSwitch{
		Tag:   v,
		Conds: conds,
	}
	fn.emit(swtch, nil)
	for _, body := range bodies {
		addEdge(entry, body)
	}
}

// stmt lowers statement s to IR form, emitting code to fn.
func (b *builder) stmt(fn *Function, _s ast.Stmt) {
	// The label of the current statement.  If non-nil, its _goto
	// target is always set; its _break and _continue are set only
	// within the body of switch/typeswitch/select/for/range.
	// It is effectively an additional default-nil parameter of stmt().
	var label *lblock
start:
	switch s := _s.(type) {
	case *ast.EmptyStmt:
		// ignore.  (Usually removed by gofmt.)

	case *ast.DeclStmt: // Con, Var or Typ
		d := s.Decl.(*ast.GenDecl)
		if d.Tok == token.VAR {
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					b.localValueSpec(fn, vs)
				}
			}
		}

	case *ast.LabeledStmt:
		if s.Label.Name == "_" {
			// Blank labels can't be the target of a goto, break,
			// or continue statement, so we don't need a new block.
			_s = s.Stmt
			goto start
		}
		label = fn.lblockOf(fn.label(s.Label))
		label.resolved = true
		emitJump(fn, label._goto, s)
		fn.currentBlock = label._goto
		_s = s.Stmt
		goto start // effectively: tailcall stmt(fn, s.Stmt, label)

	case *ast.ExprStmt:
		b.expr(fn, s.X)

	case *ast.SendStmt:
		instr := &Send{
			Chan: b.expr(fn, s.Chan),
			X: emitConv(fn, b.expr(fn, s.Value),
				typeutil.CoreType(fn.Pkg.typeOf(s.Chan)).Underlying().(*types.Chan).Elem(), s),
		}
		fn.emit(instr, s)

	case *ast.IncDecStmt:
		op := token.ADD
		if s.Tok == token.DEC {
			op = token.SUB
		}
		loc := b.addr(fn, s.X, false)
		b.assignOp(fn, loc, emitConst(fn, NewConst(constant.MakeInt64(1), loc.typ(), s)), op, s)

	case *ast.AssignStmt:
		switch s.Tok {
		case token.ASSIGN, token.DEFINE:
			b.assignStmt(fn, s.Lhs, s.Rhs, s.Tok == token.DEFINE, _s)

		default: // +=, etc.
			op := s.Tok + token.ADD - token.ADD_ASSIGN
			b.assignOp(fn, b.addr(fn, s.Lhs[0], false), b.expr(fn, s.Rhs[0]), op, s)
		}

	case *ast.GoStmt:
		// The "intrinsics" new/make/len/cap are forbidden here.
		// panic is treated like an ordinary function call.
		v := Go{}
		b.setCall(fn, s.Call, &v.Call)
		fn.emit(&v, s)

	case *ast.DeferStmt:
		// The "intrinsics" new/make/len/cap are forbidden here.
		// panic is treated like an ordinary function call.
		deferstack := emitLoad(fn, fn.lookup(fn.deferstack, false), s)
		v := Defer{_DeferStack: deferstack}
		b.setCall(fn, s.Call, &v.Call)
		fn.hasDefer = true
		fn.emit(&v, s)

	case *ast.ReturnStmt:
		b.returnStmt(fn, s)

	case *ast.BranchStmt:
		b.branchStmt(fn, s)

	case *ast.BlockStmt:
		b.stmtList(fn, s.List)

	case *ast.IfStmt:
		if s.Init != nil {
			b.stmt(fn, s.Init)
		}
		then := fn.newBasicBlock("if.then")
		done := fn.newBasicBlock("if.done")
		els := done
		if s.Else != nil {
			els = fn.newBasicBlock("if.else")
		}
		instr := b.cond(fn, s.Cond, then, els)
		instr.source = s
		fn.currentBlock = then
		b.stmt(fn, s.Body)
		emitJump(fn, done, s)

		if s.Else != nil {
			fn.currentBlock = els
			b.stmt(fn, s.Else)
			emitJump(fn, done, s)
		}

		fn.currentBlock = done

	case *ast.SwitchStmt:
		b.switchStmt(fn, s, label)

	case *ast.TypeSwitchStmt:
		b.typeSwitchStmt(fn, s, label)

	case *ast.SelectStmt:
		if b.selectStmt(fn, s, label) {
			// the select has no cases, it blocks forever
			fn.currentBlock = fn.newBasicBlock("unreachable")
		}

	case *ast.ForStmt:
		b.forStmt(fn, s, label)

	case *ast.RangeStmt:
		b.rangeStmt(fn, s, label, s)

	default:
		panic(fmt.Sprintf("unexpected statement kind: %T", s))
	}
}

func (b *builder) branchStmt(fn *Function, s *ast.BranchStmt) {
	var block *BasicBlock
	if s.Label == nil {
		block = targetedBlock(fn, s.Tok)
	} else {
		target := fn.label(s.Label)
		block = labelledBlock(fn, target, s.Tok)
		if block == nil { // forward goto
			lb := fn.lblockOf(target)
			block = lb._goto // jump to lb._goto
			if fn.jump != nil {
				// fn is a range-over-func and the goto may exit fn.
				// Create an exit and resolve it at the end of
				// builder.buildYieldFunc.
				labelExit(fn, target, s)
			}
		}
	}
	to := block.parent

	if to == fn {
		emitJump(fn, block, s)
	} else { // break outside of fn.
		// fn must be a range-over-func
		e := blockExit(fn, block, s)
		id_ := intConst(e.id, s)
		id_.comment = fmt.Sprintf("rangefunc.exit.%d", e.id)
		id := emitConst(fn, id_)
		storeVar(fn, fn.jump, id, s)
		vFalse := emitConst(fn, NewConst(constant.MakeBool(false), tBool, s))
		emitReturn(fn, []Value{vFalse}, s)
	}
	fn.currentBlock = fn.newBasicBlock("unreachable")
}

func (b *builder) returnStmt(fn *Function, s *ast.ReturnStmt) {
	// TODO(dh): we could emit tighter position information by
	// using the ith returned expression

	var results []Value

	sig := fn.sourceFn.Signature // signature of the enclosing source function

	// Convert return operands to result type.
	if len(s.Results) == 1 && sig.Results().Len() > 1 {
		// Return of one expression in a multi-valued function.
		tuple := b.exprN(fn, s.Results[0])
		ttuple := tuple.Type().(*types.Tuple)
		for i, n := 0, ttuple.Len(); i < n; i++ {
			results = append(results,
				emitConv(fn, emitExtract(fn, tuple, i, s),
					sig.Results().At(i).Type(), s))
		}
	} else {
		// 1:1 return, or no-arg return in non-void function.
		for i, r := range s.Results {
			v := emitConv(fn, b.expr(fn, r), sig.Results().At(i).Type(), s)
			results = append(results, v)
		}
	}

	// Store the results.
	for i, r := range results {
		var result Value // fn.sourceFn.result[i] conceptually
		if fn == fn.sourceFn {
			result = fn.results[i]
		} else { // lookup needed?
			result = fn.lookup(fn.returnVars[i], false)
		}
		emitStore(fn, result, r, s)
	}

	if fn.jump != nil {
		// Return from body of a range-over-func.
		// The return statement is syntactically within the loop,
		// but the generated code is in the 'switch jump {...}' after it.
		e := returnExit(fn, s)
		id_ := intConst(e.id, e.source)
		id_.comment = fmt.Sprintf("rangefunc.exit.%d", e.id)
		id := emitConst(fn, id_)
		storeVar(fn, fn.jump, id, e.source)
		vFalse := emitConst(fn, NewConst(constant.MakeBool(false), tBool, e.source))
		emitReturn(fn, []Value{vFalse}, e.source)
		return
	}

	// The results have already been stored to variables in fn.results, so
	// emitReturn doesn't have to do it again.
	emitReturn(fn, nil, s)
}

func emitReturn(fn *Function, results []Value, source ast.Node) {
	for i, r := range results {
		emitStore(fn, fn.results[i], r, source)
	}

	emitJump(fn, fn.Exit, source)
	fn.currentBlock = fn.newBasicBlock("unreachable")
}

// buildFunction builds IR code for the body of function fn.  Idempotent.
func (b *builder) buildFunction(fn *Function) {
	if fn.Blocks != nil {
		return // building already started
	}

	var recvField *ast.FieldList
	var body *ast.BlockStmt
	var functype *ast.FuncType
	switch n := fn.source.(type) {
	case nil:
		return // not a Go source function.  (Synthetic, or from object file.)
	case *ast.FuncDecl:
		functype = n.Type
		recvField = n.Recv
		body = n.Body
	case *ast.FuncLit:
		functype = n.Type
		body = n.Body
	default:
		panic(n)
	}

	if body == nil {
		// External function.
		if fn.Params == nil {
			// This condition ensures we add a non-empty
			// params list once only, but we may attempt
			// the degenerate empty case repeatedly.
			// TODO(adonovan): opt: don't do that.

			// We set Function.Params even though there is no body
			// code to reference them.  This simplifies clients.
			if recv := fn.Signature.Recv(); recv != nil {
				// XXX synthesize an ast.Node
				fn.addParamVar(recv, nil)
			}
			params := fn.Signature.Params()
			for i, n := 0, params.Len(); i < n; i++ {
				// XXX synthesize an ast.Node
				fn.addParamVar(params.At(i), nil)
			}
		}
		return
	}
	if fn.Prog.mode&LogSource != 0 {
		defer logStack("build function %s @ %s", fn, fn.Prog.Fset.Position(fn.Pos()))()
	}
	fn.blocksets = b.blocksets
	fn.Blocks = make([]*BasicBlock, 0, avgBlocks)
	fn.sourceFn = fn
	fn.startBody()
	fn.createSyntacticParams(recvField, functype)
	fn.createDeferStack()
	fn.exitBlock()
	b.stmt(fn, body)
	if cb := fn.currentBlock; cb != nil && (cb == fn.Blocks[0] || cb.Preds != nil) {
		// Control fell off the end of the function's body block.
		//
		// Block optimizations eliminate the current block, if
		// unreachable.  It is a builder invariant that
		// if this no-arg return is ill-typed for
		// fn.Signature.Results, this block must be
		// unreachable.  The sanity checker checks this.
		// fn.emit(new(RunDefers))
		// fn.emit(new(Return))
		emitJump(fn, fn.Exit, nil)
	}
	optimizeBlocks(fn)
	buildFakeExits(fn)
	fn.finishBody()
	b.blocksets = fn.blocksets
	fn.functionBody = nil
}

// buildYieldFunc builds the body of the yield function created
// from a range-over-func *ast.RangeStmt.
func (b *builder) buildYieldFunc(fn *Function) {
	// See builder.rangeFunc for detailed documentation on how fn is set up.
	//
	// In pseudo-Go this roughly builds:
	// func yield(_k tk, _v tv) bool {
	//         if jump != READY { panic("yield function called after range loop exit") }
	//     jump = BUSY
	//     k, v = _k, _v // assign the iterator variable (if needed)
	//     ... // rng.Body
	//   continue:
	//     jump = READY
	//     return true
	// }
	s := fn.source.(*ast.RangeStmt)
	fn.sourceFn = fn.parent.sourceFn
	fn.startBody()
	params := fn.Signature.Params()
	for v := range params.Variables() {
		fn.addParamVar(v, nil)
	}
	fn.addResultVar(fn.Signature.Results().At(0), nil)
	fn.exitBlock()

	// Initial targets
	ycont := fn.newBasicBlock("yield-continue")
	// lblocks is either {} or is {label: nil} where label is the label of syntax.
	for label := range fn.lblocks {
		fn.lblocks[label] = &lblock{
			label:     label,
			resolved:  true,
			_goto:     ycont,
			_continue: ycont,
			// `break label` statement targets fn.parent.targets._break
		}
	}
	fn.targets = &targets{
		tail:      fn.targets,
		_continue: ycont,
		// `break` statement targets fn.parent.targets._break.
	}

	// continue:
	//   jump = READY
	//   return true
	saved := fn.currentBlock
	fn.currentBlock = ycont
	storeVar(fn, fn.jump, emitConst(fn, jReady()), s.Body)
	vTrue := emitConst(fn, NewConst(constant.MakeBool(true), tBool, nil))
	emitReturn(fn, []Value{vTrue}, nil)

	// Emit header:
	//
	//   if jump != READY { panic("yield iterator accessed after exit") }
	//   jump = BUSY
	//   k, v = _k, _v
	fn.currentBlock = saved
	yloop := fn.newBasicBlock("yield-loop")
	invalid := fn.newBasicBlock("yield-invalid")

	jumpVal := emitLoad(fn, fn.lookup(fn.jump, true), nil)
	emitIf(fn, emitCompare(fn, token.EQL, jumpVal, emitConst(fn, jReady()), nil), yloop, invalid, nil)
	fn.currentBlock = invalid
	fn.emit(
		&Panic{
			X: emitConv(fn, emitConst(fn, stringConst("yield function called after range loop exit", nil)), tEface, nil),
		},
		nil,
	)
	addEdge(fn.currentBlock, fn.Exit)

	fn.currentBlock = yloop
	storeVar(fn, fn.jump, emitConst(fn, jBusy()), s.Body)

	// Initialize k and v from params.
	var tk, tv types.Type
	if s.Key != nil && !isBlankIdent(s.Key) {
		tk = fn.Pkg.typeOf(s.Key) // fn.parent.typeOf is identical
	}
	if s.Value != nil && !isBlankIdent(s.Value) {
		tv = fn.Pkg.typeOf(s.Value)
	}
	if s.Tok == token.DEFINE {
		if tk != nil {
			emitLocalVar(fn, identVar(fn, s.Key.(*ast.Ident)), s.Key)
		}
		if tv != nil {
			emitLocalVar(fn, identVar(fn, s.Value.(*ast.Ident)), s.Value)
		}
	}
	var k, v Value
	if len(fn.Params) > 0 {
		k = fn.Params[0]
	}
	if len(fn.Params) > 1 {
		v = fn.Params[1]
	}
	var kl, vl lvalue
	if tk != nil {
		kl = b.addr(fn, s.Key, false) // non-escaping
	}
	if tv != nil {
		vl = b.addr(fn, s.Value, false) // non-escaping
	}
	if tk != nil {
		kl.store(fn, k, s.Key)
	}
	if tv != nil {
		vl.store(fn, v, s.Value)
	}

	// Build the body of the range loop.
	b.stmt(fn, s.Body)
	if cb := fn.currentBlock; cb != nil && (cb == fn.Blocks[0] || cb.Preds != nil) {
		// Control fell off the end of the function's body block.
		// Block optimizations eliminate the current block, if
		// unreachable.
		emitJump(fn, ycont, nil)
	}
	fn.targets = fn.targets.tail

	// Clean up exits and promote any unresolved exits to fn.parent.
	for _, e := range fn.exits {
		if e.label != nil {
			lb := fn.lblocks[e.label]
			if lb.resolved {
				// label was resolved. Do not turn lb into an exit.
				// e does not need to be handled by the parent.
				continue
			}

			// _goto becomes an exit.
			//   _goto:
			//     jump = id
			//     return false
			fn.currentBlock = lb._goto
			id_ := intConst(e.id, e.source)
			id_.comment = fmt.Sprintf("rangefunc.exit.%d", e.id)
			id := emitConst(fn, id_)
			storeVar(fn, fn.jump, id, e.source)
			vFalse := emitConst(fn, NewConst(constant.MakeBool(false), tBool, e.source))
			emitReturn(fn, []Value{vFalse}, e.source)
		}

		if e.to != fn { // e needs to be handled by the parent too.
			fn.parent.exits = append(fn.parent.exits, e)
		}
	}

	fn.finishBody()
}

// buildFuncDecl builds IR code for the function or method declared
// by decl in package pkg.
func (b *builder) buildFuncDecl(pkg *Package, decl *ast.FuncDecl) {
	id := decl.Name
	fn := pkg.values[pkg.info.Defs[id]].(*Function)
	if decl.Recv == nil && id.Name == "init" {
		var v Call
		v.Call.Value = fn
		v.setType(types.NewTuple())
		pkg.init.emit(&v, decl)
	}
	fn.source = decl
	b.buildFunction(fn)
}

// Build calls Package.Build for each package in prog.
//
// Build is intended for whole-program analysis; a typical compiler
// need only build a single package.
//
// Build is idempotent and thread-safe.
func (prog *Program) Build() {
	for _, p := range prog.packages {
		p.Build()
	}
}

// Build builds IR code for all functions and vars in package p.
//
// Precondition: CreatePackage must have been called for all of p's
// direct imports (and hence its direct imports must have been
// error-free).
//
// Build is idempotent and thread-safe.
func (p *Package) Build() { p.buildOnce.Do(p.build) }

func (p *Package) build() {
	if p.info == nil {
		return // synthetic package, e.g. "testmain"
	}

	// Ensure we have runtime type info for all exported members.
	// TODO(adonovan): ideally belongs in memberFromObject, but
	// that would require package creation in topological order.
	for name, mem := range p.Members {
		if ast.IsExported(name) {
			p.Prog.needMethodsOf(mem.Type())
		}
	}
	if p.Prog.mode&LogSource != 0 {
		defer logStack("build %s", p)()
	}
	init := p.init
	init.startBody()
	init.exitBlock()

	var done *BasicBlock

	// Make init() skip if package is already initialized.
	initguard := p.Var("init$guard")
	doinit := init.newBasicBlock("init.start")
	done = init.Exit
	emitIf(init, emitLoad(init, initguard, nil), done, doinit, nil)
	init.currentBlock = doinit
	emitStore(init, initguard, emitConst(init, NewConst(constant.MakeBool(true), tBool, nil)), nil)

	// Call the init() function of each package we import.
	for _, pkg := range p.Pkg.Imports() {
		prereq := p.Prog.packages[pkg]
		if prereq == nil {
			panic(fmt.Sprintf("Package(%q).Build(): unsatisfied import: Program.CreatePackage(%q) was not called", p.Pkg.Path(), pkg.Path()))
		}
		var v Call
		v.Call.Value = prereq.init
		v.setType(types.NewTuple())
		init.emit(&v, nil)
	}

	b := builder{
		printFunc: p.printFunc,
	}

	// Initialize package-level vars in correct order.
	for _, varinit := range p.info.InitOrder {
		if init.Prog.mode&LogSource != 0 {
			fmt.Fprintf(os.Stderr, "build global initializer %v @ %s\n",
				varinit.Lhs, p.Prog.Fset.Position(varinit.Rhs.Pos()))
		}
		// Initializers for global vars are evaluated in dependency
		// order, but may come from arbitrary files of the package
		// with different versions, so we transiently update
		// init.goversion for each one. (Since init is a synthetic
		// function it has no syntax of its own that needs a version.)
		init.goversion = p.initVersion[varinit.Rhs]
		if len(varinit.Lhs) == 1 {
			// 1:1 initialization: var x, y = a(), b()
			var lval lvalue
			if v := varinit.Lhs[0]; v.Name() != "_" {
				lval = &address{addr: p.values[v].(*Global)}
			} else {
				lval = blank{}
			}
			// TODO(dh): do emit position information
			b.assign(init, lval, varinit.Rhs, true, nil, nil)
		} else {
			// n:1 initialization: var x, y :=  f()
			tuple := b.exprN(init, varinit.Rhs)
			for i, v := range varinit.Lhs {
				if v.Name() == "_" {
					continue
				}
				emitStore(init, p.values[v].(*Global), emitExtract(init, tuple, i, nil), nil)
			}
		}
	}
	init.goversion = "" // The rest of the init function is synthetic. No syntax => no goversion.

	// Build all package-level functions, init functions
	// and methods, including unreachable/blank ones.
	// We build them in source order, but it's not significant.
	for _, file := range p.files {
		for _, decl := range file.Decls {
			if decl, ok := decl.(*ast.FuncDecl); ok {
				b.buildFuncDecl(p, decl)
			}
		}
	}

	// Finish up init().
	emitJump(init, done, nil)
	init.finishBody()

	// We no longer need ASTs or go/types deductions.
	p.info = nil
	p.initVersion = nil

	if p.Prog.mode&SanityCheckFunctions != 0 {
		sanityCheckPackage(p)
	}
}

// Like ObjectOf, but panics instead of returning nil.
// Only valid during p's create and build phases.
func (p *Package) objectOf(id *ast.Ident) types.Object {
	if o := p.info.ObjectOf(id); o != nil {
		return o
	}
	panic(fmt.Sprintf("no types.Object for ast.Ident %s @ %s",
		id.Name, p.Prog.Fset.Position(id.Pos())))
}

// Like TypeOf, but panics instead of returning nil.
// Only valid during p's create and build phases.
func (p *Package) typeOf(e ast.Expr) types.Type {
	if T := p.info.TypeOf(e); T != nil {
		return T
	}
	panic(fmt.Sprintf("no type for %T @ %s",
		e, p.Prog.Fset.Position(e.Pos())))
}
