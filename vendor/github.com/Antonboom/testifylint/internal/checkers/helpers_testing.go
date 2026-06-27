package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

func isSubTestRun(pass *analysis.Pass, ce *ast.CallExpr) bool {
	se, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok || se.Sel == nil {
		return false
	}
	return (implementsTestingT(pass, se.X) || implementsTestifySuite(pass, se.X)) && se.Sel.Name == "Run"
}

func isTestingFuncOrMethod(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	return hasTestingTParam(pass, fd.Type) || isSuiteMethod(pass, fd)
}

func isTestingAnonymousFunc(pass *analysis.Pass, ft *ast.FuncType) bool {
	return hasTestingTParam(pass, ft)
}

func hasTestingTParam(pass *analysis.Pass, ft *ast.FuncType) bool {
	if ft == nil || ft.Params == nil {
		return false
	}

	for _, param := range ft.Params.List {
		if implementsTestingT(pass, param.Type) {
			return true
		}
	}
	return false
}
