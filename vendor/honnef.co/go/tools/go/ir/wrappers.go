// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file defines synthesis of Functions that delegate to declared
// methods; they come in three kinds:
//
// (1) wrappers: methods that wrap declared methods, performing
//     implicit pointer indirections and embedded field selections.
//
// (2) thunks: funcs that wrap declared methods.  Like wrappers,
//     thunks perform indirections and field selections. The thunk's
//     first parameter is used as the receiver for the method call.
//
// (3) bounds: funcs that wrap declared methods.  The bound's sole
//     free variable, supplied by a closure, is used as the receiver
//     for the method call.  No indirections or field selections are
//     performed since they can be done before the call.

import (
	"fmt"
	"go/types"
)

// -- wrappers -----------------------------------------------------------

// makeWrapper returns a synthetic method that delegates to the
// declared method denoted by meth.Obj(), first performing any
// necessary pointer indirections or field selections implied by meth.
//
// The resulting method's receiver type is meth.Recv().
//
// This function is versatile but quite subtle!  Consider the
// following axes of variation when making changes:
//   - optional receiver indirection
//   - optional implicit field selections
//   - meth.Obj() may denote a concrete or an interface method
//   - the result may be a thunk or a wrapper.
//
// EXCLUSIVE_LOCKS_REQUIRED(prog.methodsMu)
func makeWrapper(prog *Program, sel *types.Selection) *Function {
	obj := sel.Obj().(*types.Func)       // the declared function
	sig := sel.Type().(*types.Signature) // type of this wrapper

	var recv *types.Var // wrapper's receiver or thunk's params[0]
	name := obj.Name()
	var description Synthetic
	var start int // first regular param
	if sel.Kind() == types.MethodExpr {
		name += "$thunk"
		description = SyntheticThunk
		recv = sig.Params().At(0)
		start = 1
	} else {
		description = SyntheticWrapper
		recv = sig.Recv()
	}

	if prog.mode&LogSource != 0 {
		defer logStack("make %s to (%s)", description, recv.Type())()
	}
	fn := &Function{
		name:         name,
		method:       sel,
		object:       obj,
		Signature:    sig,
		Synthetic:    description,
		Prog:         prog,
		functionBody: new(functionBody),
	}
	fn.initHTML(prog.PrintFunc)
	fn.startBody()
	fn.addSpilledParam(recv, nil)
	createParams(fn, start)

	indices := sel.Index()

	var v Value = fn.Locals[0] // spilled receiver
	if isPointer(sel.Recv()) {
		v = emitLoad(fn, v, nil)

		// For simple indirection wrappers, perform an informative nil-check:
		// "value method (T).f called using nil *T pointer"
		if len(indices) == 1 && !isPointer(recvType(obj)) {
			var c Call
			c.Call.Value = &Builtin{
				name: "ir:wrapnilchk",
				sig: types.NewSignatureType(nil, nil, nil,
					types.NewTuple(anonVar(sel.Recv()), anonVar(tString), anonVar(tString)),
					types.NewTuple(anonVar(sel.Recv())), false),
			}
			c.Call.Args = []Value{
				v,
				emitConst(fn, stringConst(deref(sel.Recv()).String(), nil)),
				emitConst(fn, stringConst(sel.Obj().Name(), nil)),
			}
			c.setType(v.Type())
			v = fn.emit(&c, nil)
		}
	}

	// Invariant: v is a pointer, either
	//   value of *A receiver param, or
	// address of  A spilled receiver.

	// We use pointer arithmetic (FieldAddr possibly followed by
	// Load) in preference to value extraction (Field possibly
	// preceded by Load).

	v = emitImplicitSelections(fn, v, indices[:len(indices)-1], nil)

	// Invariant: v is a pointer, either
	//   value of implicit *C field, or
	// address of implicit  C field.

	var c Call
	if r := recvType(obj); !types.IsInterface(r) { // concrete method
		if !isPointer(r) {
			v = emitLoad(fn, v, nil)
		}
		c.Call.Value = prog.declaredFunc(obj)
		c.Call.Args = append(c.Call.Args, v)
	} else {
		c.Call.Method = obj
		c.Call.Value = emitLoad(fn, v, nil)
	}
	for _, arg := range fn.Params[1:] {
		c.Call.Args = append(c.Call.Args, arg)
	}
	emitTailCall(fn, &c, nil)
	fn.finishBody()
	return fn
}

// createParams creates parameters for wrapper method fn based on its
// Signature.Params, which do not include the receiver.
// start is the index of the first regular parameter to use.
func createParams(fn *Function, start int) {
	tparams := fn.Signature.Params()
	for i, n := start, tparams.Len(); i < n; i++ {
		fn.addParamVar(tparams.At(i), nil)
	}
}

// -- bounds -----------------------------------------------------------

// makeBound returns a bound method wrapper (or "bound"), a synthetic
// function that delegates to a concrete or interface method denoted
// by obj.  The resulting function has no receiver, but has one free
// variable which will be used as the method's receiver in the
// tail-call.
//
// Use MakeClosure with such a wrapper to construct a bound method
// closure.  e.g.:
//
//	type T int          or:  type T interface { meth() }
//	func (t T) meth()
//	var t T
//	f := t.meth
//	f() // calls t.meth()
//
// f is a closure of a synthetic wrapper defined as if by:
//
//	f := func() { return t.meth() }
//
// Unlike makeWrapper, makeBound need perform no indirection or field
// selections because that can be done before the closure is
// constructed.
//
// EXCLUSIVE_LOCKS_ACQUIRED(meth.Prog.methodsMu)
func makeBound(prog *Program, obj *types.Func) *Function {
	prog.methodsMu.Lock()
	defer prog.methodsMu.Unlock()
	if prog.mode&LogSource != 0 {
		defer logStack("%s", SyntheticBound)()
	}
	fn := &Function{
		name:         obj.Name() + "$bound",
		object:       obj,
		Signature:    changeRecv(obj.Type().(*types.Signature), nil), // drop receiver
		Synthetic:    SyntheticBound,
		Prog:         prog,
		functionBody: new(functionBody),
	}
	fn.initHTML(prog.PrintFunc)

	fv := &FreeVar{name: "recv", typ: recvType(obj), parent: fn}
	fn.FreeVars = []*FreeVar{fv}
	fn.startBody()
	createParams(fn, 0)
	var c Call

	if !types.IsInterface(recvType(obj)) { // concrete
		c.Call.Value = prog.declaredFunc(obj)
		c.Call.Args = []Value{fv}
	} else {
		c.Call.Value = fv
		c.Call.Method = obj
	}
	for _, arg := range fn.Params {
		c.Call.Args = append(c.Call.Args, arg)
	}
	emitTailCall(fn, &c, nil)
	fn.finishBody()
	return fn
}

// -- thunks -----------------------------------------------------------

// makeThunk returns a thunk, a synthetic function that delegates to a
// concrete or interface method denoted by sel.Obj().  The resulting
// function has no receiver, but has an additional (first) regular
// parameter.
//
// Precondition: sel.Kind() == types.MethodExpr.
//
//	type T int          or:  type T interface { meth() }
//	func (t T) meth()
//	f := T.meth
//	var t T
//	f(t) // calls t.meth()
//
// f is a synthetic wrapper defined as if by:
//
//	f := func(t T) { return t.meth() }
//
// EXCLUSIVE_LOCKS_ACQUIRED(meth.Prog.methodsMu)
func makeThunk(prog *Program, sel *types.Selection) *Function {
	if sel.Kind() != types.MethodExpr {
		panic(sel)
	}

	prog.methodsMu.Lock()
	defer prog.methodsMu.Unlock()

	fn := makeWrapper(prog, sel)
	if fn.Signature.Recv() != nil {
		panic(fn) // unexpected receiver
	}
	return fn
}

func changeRecv(s *types.Signature, recv *types.Var) *types.Signature {
	return types.NewSignatureType(recv, nil, nil, s.Params(), s.Results(), s.Variadic())
}

// makeInstance creates a wrapper function with signature sig that calls the generic function fn.
// If targs is not nil, fn is a function and targs describes the concrete type arguments.
// If targs is nil, fn is a method and the type arguments are derived from the receiver.
func makeInstance(prog *Program, fn *Function, sig *types.Signature, targs *types.TypeList) *Function {
	if sig.Recv() != nil {
		assert(targs == nil)
		// Methods don't have their own type parameters, but the receiver does
		targs = types.Unalias(deref(sig.Recv().Type())).(*types.Named).TypeArgs()
	} else {
		assert(targs != nil)
	}

	wrapper := fn.generics.At(targs)
	if wrapper != nil {
		return wrapper
	}

	var name string
	if sig.Recv() != nil {
		name = fn.name
	} else {
		name = fmt.Sprintf("%s$generic#%d", fn.name, fn.generics.Len())
	}
	w := &Function{
		name:         name,
		object:       fn.object,
		Signature:    sig,
		Synthetic:    SyntheticGeneric,
		Prog:         prog,
		functionBody: new(functionBody),
	}
	w.initHTML(prog.PrintFunc)
	w.startBody()
	if sig.Recv() != nil {
		w.addParamVar(sig.Recv(), nil)
	}
	createParams(w, 0)
	var c Call
	c.Call.Value = fn
	tresults := fn.Signature.Results()
	if tresults.Len() == 1 {
		c.typ = tresults.At(0).Type()
	} else {
		c.typ = tresults
	}

	changeType := func(v Value, typ types.Type) Value {
		if types.Identical(v.Type(), typ) {
			return v
		}
		var c ChangeType
		c.X = v
		c.typ = typ
		return w.emit(&c, nil)
	}

	for i, arg := range w.Params {
		if sig.Recv() != nil {
			if i == 0 {
				c.Call.Args = append(c.Call.Args, changeType(w.Params[0], fn.Signature.Recv().Type()))
			} else {
				c.Call.Args = append(c.Call.Args, changeType(arg, fn.Signature.Params().At(i-1).Type()))
			}
		} else {
			c.Call.Args = append(c.Call.Args, changeType(arg, fn.Signature.Params().At(i).Type()))
		}
	}
	for arg := range targs.Types() {
		c.Call.TypeArgs = append(c.Call.TypeArgs, arg)
	}
	results := w.emit(&c, nil)
	var ret Return
	switch tresults.Len() {
	case 0:
	case 1:
		ret.Results = []Value{changeType(results, sig.Results().At(0).Type())}
	default:
		for i := 0; i < tresults.Len(); i++ {
			v := emitExtract(w, results, i, nil)
			ret.Results = append(ret.Results, changeType(v, sig.Results().At(i).Type()))
		}
	}

	w.Exit = w.newBasicBlock("exit")
	emitJump(w, w.Exit, nil)
	w.currentBlock = w.Exit
	w.emit(&ret, nil)
	w.currentBlock = nil

	w.finishBody()

	fn.generics.Set(targs, w)
	return w
}
