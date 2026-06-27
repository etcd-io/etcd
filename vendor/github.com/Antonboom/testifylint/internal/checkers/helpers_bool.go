package checkers

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

var (
	falseObj = types.Universe.Lookup("false")
	trueObj  = types.Universe.Lookup("true")
)

func isUntypedBool(pass *analysis.Pass, e ast.Expr) bool {
	return isUntypedTrue(pass, e) || isUntypedFalse(pass, e)
}

func isUntypedTrue(pass *analysis.Pass, e ast.Expr) bool {
	return analysisutil.IsObj(pass.TypesInfo, e, trueObj)
}

func isUntypedFalse(pass *analysis.Pass, e ast.Expr) bool {
	return analysisutil.IsObj(pass.TypesInfo, e, falseObj)
}

func hasBoolType(pass *analysis.Pass, e ast.Expr) bool {
	basicType, ok := pass.TypesInfo.TypeOf(e).(*types.Basic)
	return ok && basicType.Kind() == types.Bool
}

func isBoolOverride(pass *analysis.Pass, e ast.Expr) bool {
	namedType, ok := pass.TypesInfo.TypeOf(e).(*types.Named)
	return ok && namedType.Obj().Name() == "bool"
}
