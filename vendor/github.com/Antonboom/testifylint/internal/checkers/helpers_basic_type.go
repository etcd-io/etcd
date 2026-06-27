package checkers

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

func isZero(e ast.Expr) bool { return isIntNumber(e, 0) }

func isOne(e ast.Expr) bool { return isIntNumber(e, 1) }

func isAnyZero(e ast.Expr) bool {
	return isIntNumber(e, 0) || isTypedSignedIntNumber(e, 0) || isTypedUnsignedIntNumber(e, 0)
}

func isNotAnyZero(e ast.Expr) bool {
	return !isAnyZero(e)
}

func isZeroOrSignedZero(e ast.Expr) bool {
	return isIntNumber(e, 0) || isTypedSignedIntNumber(e, 0)
}

func isSignedNotZero(pass *analysis.Pass, e ast.Expr) bool {
	return !isUnsigned(pass, e) && !isZeroOrSignedZero(e)
}

func isTypedSignedIntNumber(e ast.Expr, v int) bool {
	return isTypedIntNumber(e, v, "int", "int8", "int16", "int32", "int64")
}

func isTypedUnsignedIntNumber(e ast.Expr, v int) bool {
	return isTypedIntNumber(e, v, "uint", "uint8", "uint16", "uint32", "uint64")
}

func isTypedIntNumber(e ast.Expr, v int, goTypes ...string) bool {
	ce, ok := e.(*ast.CallExpr)
	if !ok || len(ce.Args) != 1 {
		return false
	}

	fn, ok := ce.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	for _, t := range goTypes {
		if fn.Name == t {
			return isIntNumber(ce.Args[0], v)
		}
	}
	return false
}

func isIntNumber(e ast.Expr, rhs int) bool {
	lhs, ok := isIntBasicLit(e)
	return ok && (lhs == rhs)
}

func isStringLit(e ast.Expr) bool {
	bl, ok := e.(*ast.BasicLit)
	return ok && bl.Kind == token.STRING
}

func isEmptyStringLit(e ast.Expr) bool {
	bl, ok := e.(*ast.BasicLit)
	return ok && bl.Kind == token.STRING && (bl.Value == `""` || bl.Value == "``")
}

func isBasicLit(e ast.Expr) bool {
	if un, ok := e.(*ast.UnaryExpr); ok {
		if un.Op == token.SUB {
			return isBasicLit(un.X)
		}
	}

	_, ok := e.(*ast.BasicLit)
	return ok
}

func isIntBasicLit(e ast.Expr) (int, bool) {
	if un, ok := e.(*ast.UnaryExpr); ok {
		if un.Op == token.SUB {
			v, ok := isIntBasicLit(un.X)
			return -1 * v, ok
		}
	}

	bl, ok := e.(*ast.BasicLit)
	if !ok {
		return 0, false
	}
	if bl.Kind != token.INT {
		return 0, false
	}

	v, err := strconv.Atoi(bl.Value)
	if err != nil {
		return 0, false
	}
	return v, true
}

func isUntypedConst(pass *analysis.Pass, e ast.Expr) bool {
	return isUnderlying(pass, e, types.IsUntyped)
}

func isTypedConst(pass *analysis.Pass, e ast.Expr) bool {
	tt, ok := pass.TypesInfo.Types[e]
	return ok && tt.IsValue() && tt.Value != nil
}

func isFloat(pass *analysis.Pass, e ast.Expr) bool {
	return isUnderlying(pass, e, types.IsFloat)
}

func isUnsigned(pass *analysis.Pass, e ast.Expr) bool {
	return isUnderlying(pass, e, types.IsUnsigned)
}

func isUnderlying(pass *analysis.Pass, e ast.Expr, flag types.BasicInfo) bool {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return false
	}

	bt, ok := t.Underlying().(*types.Basic)
	return ok && (bt.Info()&flag > 0)
}

func isPointer(pass *analysis.Pass, e ast.Expr) (types.Type, bool) {
	ptr, ok := pass.TypesInfo.TypeOf(e).(*types.Pointer)
	if !ok {
		return nil, false
	}
	return ptr.Elem(), true
}

func isFunc(pass *analysis.Pass, e ast.Expr) bool {
	_, ok := pass.TypesInfo.TypeOf(e).(*types.Signature)
	return ok
}

// isByteArray returns true if expression is `[]byte` itself.
func isByteArray(e ast.Expr) bool {
	at, ok := e.(*ast.ArrayType)
	return ok && isIdentWithName("byte", at.Elt)
}

// hasBytesType returns true if the expression is of `[]byte` type.
func hasBytesType(pass *analysis.Pass, e ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(e)
	if t == nil {
		return false
	}

	sl, ok := t.(*types.Slice)
	if !ok {
		return false
	}

	el, ok := sl.Elem().(*types.Basic)
	return ok && el.Kind() == types.Uint8
}

// hasStringType returns true if the expression is of `string` type.
func hasStringType(pass *analysis.Pass, e ast.Expr) bool {
	basicType, ok := pass.TypesInfo.TypeOf(e).(*types.Basic)
	return ok && basicType.Kind() == types.String
}
