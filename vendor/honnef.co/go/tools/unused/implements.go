package unused

import (
	"go/types"
)

// lookupMethod returns the index of and method with matching package and name, or (-1, nil).
func lookupMethod(T *types.Interface, pkg *types.Package, name string) (int, *types.Func) {
	if name != "_" {
		for i := 0; i < T.NumMethods(); i++ {
			m := T.Method(i)
			if sameId(m, pkg, name) {
				return i, m
			}
		}
	}
	return -1, nil
}

func sameId(obj types.Object, pkg *types.Package, name string) bool {
	// spec:
	// "Two identifiers are different if they are spelled differently,
	// or if they appear in different packages and are not exported.
	// Otherwise, they are the same."
	if name != obj.Name() {
		return false
	}
	// obj.Name == name
	if obj.Exported() {
		return true
	}
	// not exported, so packages must be the same (pkg == nil for
	// fields in Universe scope; this can only happen for types
	// introduced via Eval)
	if pkg == nil || obj.Pkg() == nil {
		return pkg == obj.Pkg()
	}
	// pkg != nil && obj.pkg != nil
	return pkg.Path() == obj.Pkg().Path()
}

func implements(V types.Type, T *types.Interface, msV *types.MethodSet) ([]*types.Selection, bool) {
	// fast path for common case
	if T.Empty() {
		return nil, true
	}

	if ityp, _ := V.Underlying().(*types.Interface); ityp != nil {
		// TODO(dh): is this code reachable?
		for m := range T.Methods() {
			_, obj := lookupMethod(ityp, m.Pkg(), m.Name())
			switch {
			case obj == nil:
				return nil, false
			case !types.Identical(obj.Type(), m.Type()):
				return nil, false
			}
		}
		return nil, true
	}

	// A concrete type implements T if it implements all methods of T.
	var sels []*types.Selection
	var c methodsChecker
	for m := range T.Methods() {
		sel := msV.Lookup(m.Pkg(), m.Name())
		if sel == nil {
			return nil, false
		}

		f, _ := sel.Obj().(*types.Func)
		if f == nil {
			return nil, false
		}

		if !c.methodIsCompatible(f, m) {
			return nil, false
		}

		sels = append(sels, sel)
	}
	return sels, true
}

type methodsChecker struct {
	typeParams map[*types.TypeParam]types.Type
}

// Currently, this doesn't support methods like `foo(x []T)`.
func (c *methodsChecker) methodIsCompatible(implFunc *types.Func, interfaceFunc *types.Func) bool {
	if types.Identical(implFunc.Type(), interfaceFunc.Type()) {
		return true
	}
	implSig, implOk := implFunc.Type().(*types.Signature)
	interfaceSig, interfaceOk := interfaceFunc.Type().(*types.Signature)
	if !implOk || !interfaceOk {
		// probably not reachable. handle conservatively.
		return false
	}

	if !c.typesAreCompatible(implSig.Params(), interfaceSig.Params()) {
		return false
	}

	if !c.typesAreCompatible(implSig.Results(), interfaceSig.Results()) {
		return false
	}

	return true
}

func (c *methodsChecker) typesAreCompatible(implTypes, interfaceTypes *types.Tuple) bool {
	if implTypes.Len() != interfaceTypes.Len() {
		return false
	}
	for i := 0; i < implTypes.Len(); i++ {
		if !c.typeIsCompatible(implTypes.At(i).Type(), interfaceTypes.At(i).Type()) {
			return false
		}
	}
	return true
}

func (c *methodsChecker) typeIsCompatible(implType, interfaceType types.Type) bool {
	if types.Identical(implType, interfaceType) {
		return true
	}
	// We only support trivial use of type parameters. This isn't fully compatible with compiler type checking yet.
	tp, ok := interfaceType.(*types.TypeParam)
	if !ok {
		return false
	}
	if c.typeParams == nil {
		c.typeParams = make(map[*types.TypeParam]types.Type)
	}
	if c.typeParams[tp] == nil {
		if !satisfiesConstraint(implType, tp) {
			return false
		}
		c.typeParams[tp] = implType
		return true
	}
	return types.Identical(c.typeParams[tp], implType)
}

func satisfiesConstraint(t types.Type, tp *types.TypeParam) bool {
	bound := tp.Constraint().Underlying().(*types.Interface)
	return types.Satisfies(t, bound)
}
