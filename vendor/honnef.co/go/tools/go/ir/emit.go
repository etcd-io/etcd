// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// Helpers for emitting IR instructions.

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/exp/typeparams"
)

// emitAlloc emits to f a new Alloc instruction allocating a variable
// of type typ.
//
// The caller must set Alloc.Heap=true (for a heap-allocated variable)
// or add the Alloc to f.Locals (for a frame-allocated variable).
//
// During building, a variable in f.Locals may have its Heap flag
// set when it is discovered that its address is taken.
// These Allocs are removed from f.Locals at the end.
//
// The builder should generally call one of the emit{New,Local,LocalVar} wrappers instead.
func emitAlloc(f *Function, typ types.Type, source ast.Node, comment string) *Alloc {
	v := &Alloc{}
	v.comment = comment
	v.setType(types.NewPointer(typ))
	f.emit(v, source)
	return v
}

// emitNew emits to f a new Alloc instruction heap-allocating a
// variable of type typ.
func emitNew(f *Function, typ types.Type, source ast.Node, comment string) *Alloc {
	alloc := emitAlloc(f, typ, source, comment)
	alloc.Heap = true
	return alloc
}

// emitLocal creates a local var for (t, source, comment) and
// emits an Alloc instruction for it.
//
// (Use this function or emitNew for synthetic variables;
// for source-level variables, use emitLocalVar.)
func emitLocal(f *Function, t types.Type, source ast.Node, comment string) *Alloc {
	local := emitAlloc(f, t, source, comment)
	f.Locals = append(f.Locals, local)
	return local
}

// emitLocalVar creates a local var for v and emits an Alloc instruction for it.
// Subsequent calls to f.lookup(v) return it.
func emitLocalVar(f *Function, v *types.Var, source ast.Node) *Alloc {
	alloc := emitLocal(f, v.Type(), source, v.Name())
	f.vars[v] = alloc
	return alloc
}

// emitLoad emits to f an instruction to load the address addr into a
// new temporary, and returns the value so defined.
func emitLoad(f *Function, addr Value, source ast.Node) *Load {
	v := &Load{X: addr}
	v.setType(deref(addr.Type()))
	f.emit(v, source)
	return v
}

func emitRecv(f *Function, ch Value, commaOk bool, typ types.Type, source ast.Node) Value {
	recv := &Recv{
		Chan:    ch,
		CommaOk: commaOk,
	}
	recv.setType(typ)
	return f.emit(recv, source)
}

// emitDebugRef emits to f a DebugRef pseudo-instruction associating
// expression e with value v.
func emitDebugRef(f *Function, e ast.Expr, v Value, isAddr bool) {
	ref := makeDebugRef(f, e, v, isAddr)
	if ref == nil {
		return
	}
	f.emit(ref, nil)
}

func makeDebugRef(f *Function, e ast.Expr, v Value, isAddr bool) *DebugRef {
	if !f.debugInfo() {
		return nil // debugging not enabled
	}
	if v == nil || e == nil {
		panic("nil")
	}
	var obj types.Object
	e = unparen(e)
	if id, ok := e.(*ast.Ident); ok {
		if isBlankIdent(id) {
			return nil
		}
		obj = f.Pkg.objectOf(id)
		switch obj.(type) {
		case *types.Nil, *types.Const, *types.Builtin:
			return nil
		}
	}
	return &DebugRef{
		X:      v,
		Expr:   e,
		IsAddr: isAddr,
		object: obj,
	}
}

// emitArith emits to f code to compute the binary operation op(x, y)
// where op is an eager shift, logical or arithmetic operation.
// (Use emitCompare() for comparisons and Builder.logicalBinop() for
// non-eager operations.)
func emitArith(f *Function, op token.Token, x, y Value, t types.Type, source ast.Node) Value {
	switch op {
	case token.SHL, token.SHR:
		x = emitConv(f, x, t, source)
		// y may be signed or an 'untyped' constant.
		// There is a runtime panic if y is signed and <0. Instead of inserting a check for y<0
		// and converting to an unsigned value (like the compiler) leave y as is.
		if b, ok := y.Type().Underlying().(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
			// Untyped conversion:
			// Spec https://go.dev/ref/spec#Operators:
			// The right operand in a shift expression must have integer type or be an untyped constant
			// representable by a value of type uint.
			y = emitConv(f, y, types.Typ[types.Uint], source)
		}

	case token.ADD, token.SUB, token.MUL, token.QUO, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT:
		x = emitConv(f, x, t, source)
		y = emitConv(f, y, t, source)

	default:
		panic("illegal op in emitArith: " + op.String())

	}
	v := &BinOp{
		Op: op,
		X:  x,
		Y:  y,
	}
	v.setType(t)
	return f.emit(v, source)
}

// emitCompare emits to f code compute the boolean result of
// comparison 'x op y'.
func emitCompare(f *Function, op token.Token, x, y Value, source ast.Node) Value {
	xt := x.Type().Underlying()
	yt := y.Type().Underlying()

	// Special case to optimise a tagless SwitchStmt so that
	// these are equivalent
	//   switch { case e: ...}
	//   switch true { case e: ... }
	//   if e==true { ... }
	// even in the case when e's type is an interface.
	// TODO(adonovan): opt: generalise to x==true, false!=y, etc.
	if x, ok := x.(*Const); ok && op == token.EQL && x.Value != nil && x.Value.Kind() == constant.Bool && constant.BoolVal(x.Value) {
		if yt, ok := yt.(*types.Basic); ok && yt.Info()&types.IsBoolean != 0 {
			return y
		}
	}

	if types.Identical(xt, yt) {
		// no conversion necessary
	} else if _, ok := xt.(*types.Interface); ok && !typeparams.IsTypeParam(x.Type()) {
		y = emitConv(f, y, x.Type(), source)
	} else if _, ok := yt.(*types.Interface); ok && !typeparams.IsTypeParam(y.Type()) {
		x = emitConv(f, x, y.Type(), source)
	} else if _, ok := x.(*Const); ok {
		x = emitConv(f, x, y.Type(), source)
	} else if _, ok := y.(*Const); ok {
		y = emitConv(f, y, x.Type(), source)
		//lint:ignore SA9003 no-op
	} else {
		// other cases, e.g. channels.  No-op.
	}

	v := &BinOp{
		Op: op,
		X:  x,
		Y:  y,
	}
	v.setType(tBool)
	return f.emit(v, source)
}

// isValuePreserving returns true if a conversion from ut_src to
// ut_dst is value-preserving, i.e. just a change of type.
// Precondition: neither argument is a named type.
func isValuePreserving(ut_src, ut_dst types.Type) bool {
	// Identical underlying types?
	if types.IdenticalIgnoreTags(ut_dst, ut_src) {
		return true
	}

	switch ut_dst.(type) {
	case *types.Chan:
		// Conversion between channel types?
		_, ok := ut_src.(*types.Chan)
		return ok

	case *types.Pointer:
		// Conversion between pointers with identical base types?
		_, ok := ut_src.(*types.Pointer)
		return ok
	}
	return false
}

// emitConv emits to f code to convert Value val to exactly type typ,
// and returns the converted value.  Implicit conversions are required
// by language assignability rules in assignments, parameter passing,
// etc.
func emitConv(f *Function, val Value, t_dst types.Type, source ast.Node) Value {
	t_src := val.Type()

	// Identical types?  Conversion is a no-op.
	if types.Identical(t_src, t_dst) {
		return val
	}

	ut_dst := t_dst.Underlying()
	ut_src := t_src.Underlying()

	// Conversion to, or construction of a value of, an interface type?
	if isNonTypeParamInterface(t_dst) {
		// Interface name change?
		if isValuePreserving(ut_src, ut_dst) {
			c := &ChangeType{X: val}
			c.setType(t_dst)
			return f.emit(c, source)
		}

		// Assignment from one interface type to another?
		if isNonTypeParamInterface(t_src) {
			c := &ChangeInterface{X: val}
			c.setType(t_dst)
			return f.emit(c, source)
		}

		// Untyped nil constant?  Return interface-typed nil constant.
		if ut_src == tUntypedNil {
			return emitConst(f, zeroConst(t_dst, source))
		}

		// Convert (non-nil) "untyped" literals to their default type.
		if t, ok := ut_src.(*types.Basic); ok && t.Info()&types.IsUntyped != 0 {
			val = emitConv(f, val, types.Default(ut_src), source)
		}

		f.Pkg.Prog.needMethodsOf(val.Type())
		mi := &MakeInterface{X: val}
		mi.setType(t_dst)
		return f.emit(mi, source)
	}

	// In the common case, the typesets of src and dst are singletons
	// and we emit an appropriate conversion. But if either contains
	// a type parameter, the conversion may represent a cross product,
	// in which case which we emit a MultiConvert.
	tset_dst := typeutil.NewTypeSet(ut_dst)
	tset_src := typeutil.NewTypeSet(ut_src)

	// conversionCase describes an instruction pattern that may be emitted to
	// model d <- s for d in dst_terms and s in src_terms.
	// Multiple conversions can match the same pattern.
	type conversionCase uint8
	const (
		changeType conversionCase = 1 << iota
		sliceToArray
		sliceToArrayPtr
		sliceTo0Array
		sliceTo0ArrayPtr
		convert
	)

	classify := func(s, d types.Type) conversionCase {
		// Just a change of type, but not value or representation?
		if isValuePreserving(s, d) {
			return changeType
		}

		// Conversion from slice to array or slice to array pointer?
		if slice, ok := s.(*types.Slice); ok {
			var arr *types.Array
			var ptr bool
			// Conversion from slice to array pointer?
			switch d := d.(type) {
			case *types.Array:
				arr = d
			case *types.Pointer:
				arr, _ = d.Elem().Underlying().(*types.Array)
				ptr = true
			}
			if arr != nil && types.Identical(slice.Elem(), arr.Elem()) {
				if arr.Len() == 0 {
					if ptr {
						return sliceTo0ArrayPtr
					} else {
						return sliceTo0Array
					}
				}
				if ptr {
					return sliceToArrayPtr
				} else {
					return sliceToArray
				}
			}
		}

		// The only remaining case in well-typed code is a representation-
		// changing conversion of basic types (possibly with []byte/[]rune).
		if !isBasic(s) && !isBasic(d) {
			panic(fmt.Sprintf("in %s: cannot convert term %s (%s [within %s]) to type %s [within %s]", f, val, val.Type(), s, t_dst, d))
		}
		return convert
	}

	var classifications conversionCase
	for _, s := range tset_src.Terms {
		us := s.Type().Underlying()
		for _, d := range tset_dst.Terms {
			ud := d.Type().Underlying()
			classifications |= classify(us, ud)
		}
	}
	if classifications == 0 {
		panic(fmt.Sprintf("in %s: cannot convert %s (%s) to %s", f, val, val.Type(), t_dst))
	}

	// Conversion of a compile-time constant value?
	if c, ok := val.(*Const); ok {
		// Conversion to a basic type?
		if isBasic(ut_dst) {
			// Conversion of a compile-time constant to
			// another constant type results in a new
			// constant of the destination type and
			// (initially) the same abstract value.
			// We don't truncate the value yet.
			return emitConst(f, NewConst(c.Value, t_dst, source))
		}
		// Can we always convert from zero value without panicking?
		const mayPanic = sliceToArray | sliceToArrayPtr
		if c.Value == nil && classifications&mayPanic == 0 {
			return emitConst(f, NewConst(nil, t_dst, source))
		}

		// We're converting from constant to non-constant type,
		// e.g. string -> []byte/[]rune.
	}

	switch classifications {
	case changeType: // representation-preserving change
		c := &ChangeType{X: val}
		c.setType(t_dst)
		return f.emit(c, source)

	case sliceToArrayPtr, sliceTo0ArrayPtr: // slice to array pointer
		c := &SliceToArrayPointer{X: val}
		c.setType(t_dst)
		return f.emit(c, source)

	case sliceToArray: // slice to arrays (not zero-length)
		p := &SliceToArray{X: val}
		p.setType(t_dst)
		return f.emit(p, source)

	case sliceTo0Array: // slice to zero-length arrays (constant)
		return emitConst(f, zeroConst(t_dst, source))

	case convert: // representation-changing conversion
		c := &Convert{X: val}
		c.setType(t_dst)
		return f.emit(c, source)

	default: // multiple conversion
		c := &MultiConvert{X: val, from: tset_src, to: tset_dst}
		c.setType(t_dst)
		return f.emit(c, source)
	}
}

// emitStore emits to f an instruction to store value val at location
// addr, applying implicit conversions as required by assignability rules.
func emitStore(f *Function, addr, val Value, source ast.Node) *Store {
	s := &Store{
		Addr: addr,
		Val:  emitConv(f, val, deref(addr.Type()), source),
	}
	f.emit(s, source)
	return s
}

// emitJump emits to f a jump to target, and updates the control-flow graph.
// Postcondition: f.currentBlock is nil.
func emitJump(f *Function, target *BasicBlock, source ast.Node) *Jump {
	b := f.currentBlock
	j := new(Jump)
	b.emit(j, source)
	addEdge(b, target)
	f.currentBlock = nil
	return j
}

// emitIf emits to f a conditional jump to tblock or fblock based on
// cond, and updates the control-flow graph.
// Postcondition: f.currentBlock is nil.
func emitIf(f *Function, cond Value, tblock, fblock *BasicBlock, source ast.Node) *If {
	b := f.currentBlock
	stmt := &If{Cond: cond}
	b.emit(stmt, source)
	addEdge(b, tblock)
	addEdge(b, fblock)
	f.currentBlock = nil
	return stmt
}

// emitExtract emits to f an instruction to extract the index'th
// component of tuple.  It returns the extracted value.
func emitExtract(f *Function, tuple Value, index int, source ast.Node) Value {
	e := &Extract{Tuple: tuple, Index: index}
	e.setType(tuple.Type().(*types.Tuple).At(index).Type())
	return f.emit(e, source)
}

// emitTypeAssert emits to f a type assertion value := x.(t) and
// returns the value.  x.Type() must be an interface.
func emitTypeAssert(f *Function, x Value, t types.Type, source ast.Node) Value {
	a := &TypeAssert{X: x, AssertedType: t}
	a.setType(t)
	return f.emit(a, source)
}

// emitTypeTest emits to f a type test value,ok := x.(t) and returns
// a (value, ok) tuple.  x.Type() must be an interface.
func emitTypeTest(f *Function, x Value, t types.Type, source ast.Node) Value {
	a := &TypeAssert{
		X:            x,
		AssertedType: t,
		CommaOk:      true,
	}
	a.setType(types.NewTuple(
		newVar("value", t),
		varOk,
	))
	return f.emit(a, source)
}

// emitTailCall emits to f a function call in tail position.  The
// caller is responsible for all fields of 'call' except its type.
// Intended for wrapper methods.
// Precondition: f does/will not use deferred procedure calls.
// Postcondition: f.currentBlock is nil.
func emitTailCall(f *Function, call *Call, source ast.Node) {
	tresults := f.Signature.Results()
	nr := tresults.Len()
	if nr == 1 {
		call.typ = tresults.At(0).Type()
	} else {
		call.typ = tresults
	}
	tuple := f.emit(call, source)
	var ret Return
	switch nr {
	case 0:
		// no-op
	case 1:
		ret.Results = []Value{tuple}
	default:
		for i := range nr {
			v := emitExtract(f, tuple, i, source)
			// TODO(adonovan): in principle, this is required:
			//   v = emitConv(f, o.Type, f.Signature.Results[i].Type)
			// but in practice emitTailCall is only used when
			// the types exactly match.
			ret.Results = append(ret.Results, v)
		}
	}

	f.Exit = f.newBasicBlock("exit")
	emitJump(f, f.Exit, source)
	f.currentBlock = f.Exit
	f.emit(&ret, source)
	f.currentBlock = nil
}

func emitCall(fn *Function, call *Call, source ast.Node) Value {
	res := fn.emit(call, source)

	callee := call.Call.StaticCallee()
	if callee != nil &&
		callee.object != nil &&
		fn.Prog.noReturn != nil &&
		fn.Prog.noReturn(callee.object) {
		// Call doesn't return normally. Either it doesn't return at all
		// (infinitely blocked or exitting the process), or it unwinds the stack
		// (panic, runtime.Goexit). In case it unwinds, jump to the exit block.
		fn.emit(new(Jump), source)
		addEdge(fn.currentBlock, fn.Exit)
		fn.currentBlock = fn.newBasicBlock("unreachable")
	}

	return res
}

// emitImplicitSelections emits to f code to apply the sequence of
// implicit field selections specified by indices to base value v, and
// returns the selected value.
//
// If v is the address of a struct, the result will be the address of
// a field; if it is the value of a struct, the result will be the
// value of a field.
func emitImplicitSelections(f *Function, v Value, indices []int, source ast.Node) Value {
	for _, index := range indices {
		// We may have a generic type containing a pointer, or a pointer to a generic type containing a struct. A
		// pointer to a generic containing a pointer to a struct shouldn't be possible because the outer pointer gets
		// dereferenced implicitly before we get here.
		fld := typeutil.CoreType(deref(v.Type())).Underlying().(*types.Struct).Field(index)

		if isPointer(v.Type()) {
			instr := &FieldAddr{
				X:     v,
				Field: index,
			}
			instr.setType(types.NewPointer(fld.Type()))
			v = f.emit(instr, source)
			// Load the field's value iff indirectly embedded.
			if isPointer(fld.Type()) {
				v = emitLoad(f, v, source)
			}
		} else {
			instr := &Field{
				X:     v,
				Field: index,
			}
			instr.setType(fld.Type())
			v = f.emit(instr, source)
		}
	}
	return v
}

// emitFieldSelection emits to f code to select the index'th field of v.
//
// If wantAddr, the input must be a pointer-to-struct and the result
// will be the field's address; otherwise the result will be the
// field's value.
// Ident id is used for position and debug info.
func emitFieldSelection(f *Function, v Value, index int, wantAddr bool, id *ast.Ident) Value {
	// We may have a generic type containing a pointer, or a pointer to a generic type containing a struct. A
	// pointer to a generic containing a pointer to a struct shouldn't be possible because the outer pointer gets
	// dereferenced implicitly before we get here.
	vut := typeutil.CoreType(deref(v.Type())).Underlying().(*types.Struct)
	fld := vut.Field(index)
	if isPointer(v.Type()) {
		instr := &FieldAddr{
			X:     v,
			Field: index,
		}
		instr.setSource(id)
		instr.setType(types.NewPointer(fld.Type()))
		v = f.emit(instr, id)
		// Load the field's value iff we don't want its address.
		if !wantAddr {
			v = emitLoad(f, v, id)
		}
	} else {
		instr := &Field{
			X:     v,
			Field: index,
		}
		instr.setSource(id)
		instr.setType(fld.Type())
		v = f.emit(instr, id)
	}
	emitDebugRef(f, id, v, wantAddr)
	return v
}

// zeroValue emits to f code to produce a zero value of type t,
// and returns it.
func zeroValue(f *Function, t types.Type, source ast.Node) Value {
	return emitConst(f, zeroConst(t, source))
}

type constKey struct {
	typ    types.Type
	value  constant.Value
	source ast.Node
}

func emitConst(f *Function, c Constant) Constant {
	if f.consts == nil {
		f.consts = map[constKey]constValue{}
	}

	typ := c.Type()
	var val constant.Value
	switch c := c.(type) {
	case *Const:
		val = c.Value
	case *ArrayConst, *GenericConst:
		// These can only represent zero values, so all we need is the type
	case *AggregateConst:
		candidates, _ := f.aggregateConsts.At(c.typ)
		for _, candidate := range candidates {
			if c.equal(candidate) {
				return candidate
			}
		}

		for i := range c.Values {
			c.Values[i] = emitConst(f, c.Values[i].(Constant))
		}

		c.setBlock(f.Blocks[0])
		rands := c.Operands(nil)
		updateOperandsReferrers(c, rands)
		candidates = append(candidates, c)
		f.aggregateConsts.Set(c.typ, candidates)
		return c

	default:
		panic(fmt.Sprintf("unexpected type %T", c))
	}
	k := constKey{
		typ:    typ,
		value:  val,
		source: c.Source(),
	}
	dup, ok := f.consts[k]
	if ok {
		return dup.c
	} else {
		c.setBlock(f.Blocks[0])
		f.consts[k] = constValue{
			c:   c,
			idx: len(f.consts),
		}
		rands := c.Operands(nil)
		updateOperandsReferrers(c, rands)
		return c
	}
}
