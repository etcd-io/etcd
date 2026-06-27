package checkers

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

type funcID struct {
	pos    token.Pos
	posStr string
	name   string
	meta   funcMeta
}

type funcMeta struct {
	isTestCleanup bool
	isGoroutine   bool
	isHTTPHandler bool
}

func (id funcID) String() string {
	return fmt.Sprintf("%s at %s", id.name, id.posStr)
}

func findSurroundingFunc(pass *analysis.Pass, stack []ast.Node) *funcID {
	for i := len(stack) - 2; i >= 0; i-- {
		var fType *ast.FuncType
		var fName string
		var isTestCleanup bool
		var isGoroutine bool
		var isHTTPHandler bool

		switch fd := stack[i].(type) {
		case *ast.FuncDecl:
			fType, fName = fd.Type, fd.Name.Name

			if isSuiteMethod(pass, fd) {
				if ident := fd.Name; ident != nil && isSuiteAfterTestMethod(ident.Name) {
					isTestCleanup = true
				}
			}

			if mimicHTTPHandler(pass, fd.Type) {
				isHTTPHandler = true
			}

		case *ast.FuncLit:
			fType, fName = fd.Type, "anonymous"

			if mimicHTTPHandler(pass, fType) {
				isHTTPHandler = true
			}

			if i >= 2 { //nolint:nestif // Already clear code.
				if ce, ok := stack[i-1].(*ast.CallExpr); ok {
					if se, ok := ce.Fun.(*ast.SelectorExpr); ok {
						isTestCleanup = implementsTestingT(pass, se.X) && se.Sel != nil && (se.Sel.Name == "Cleanup")
					}

					if _, ok := stack[i-2].(*ast.GoStmt); ok {
						isGoroutine = true
					}
				}
			}

		default:
			continue
		}

		return &funcID{
			pos:    fType.Pos(),
			posStr: pass.Fset.Position(fType.Pos()).String(),
			name:   fName,
			meta: funcMeta{
				isTestCleanup: isTestCleanup,
				isGoroutine:   isGoroutine,
				isHTTPHandler: isHTTPHandler,
			},
		}
	}
	return nil
}

func findNearestNode[T ast.Node](stack []ast.Node) (v T) {
	v, _ = findNearestNodeWithIdx[T](stack)
	return
}

func findNearestNodeWithIdx[T ast.Node](stack []ast.Node) (v T, index int) {
	for i := len(stack) - 2; i >= 0; i-- {
		if n, ok := stack[i].(T); ok {
			return n, i
		}
	}
	return
}

func fnContainsAssertions(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	for _, s := range fn.Body.List {
		if isAssertionStmt(pass, s) {
			return true
		}
	}
	return false
}

func isAssertionStmt(pass *analysis.Pass, stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}

	ce, ok := expr.X.(*ast.CallExpr)
	if !ok {
		return false
	}

	return NewCallMeta(pass, ce) != nil
}
