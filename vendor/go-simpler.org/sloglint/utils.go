package sloglint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/types/typeutil"
)

func typeName(info *types.Info, expr ast.Expr) string {
	if typ := info.TypeOf(expr); typ != nil {
		return typ.String()
	}
	return ""
}

func funcName(info *types.Info, call *ast.CallExpr) string {
	if fn := typeutil.StaticCallee(info, call); fn != nil {
		return fn.FullName()
	}
	return ""
}

func keyName(key ast.Expr) (string, bool) {
	if ident, ok := key.(*ast.Ident); ok {
		if ident.Obj == nil || ident.Obj.Decl == nil || ident.Obj.Kind != ast.Con {
			return "", false
		}
		if spec, ok := ident.Obj.Decl.(*ast.ValueSpec); ok && len(spec.Values) > 0 {
			key = spec.Values[0] // TODO: Support len(spec.Values) > 1; e.g. const foo, bar = 1, 2.
		}
	}

	lit, ok := key.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		panic("unreachable") // String literals are always quoted.
	}

	return name, true
}
