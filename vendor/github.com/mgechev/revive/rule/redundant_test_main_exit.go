package rule

import (
	"fmt"
	"go/ast"

	"github.com/mgechev/revive/lint"
)

// RedundantTestMainExitRule suggests removing redundant [os.Exit] or [syscall.Exit] calls in TestMain function.
type RedundantTestMainExitRule struct{}

// Apply applies the rule to given file.
func (*RedundantTestMainExitRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	if !file.IsTest() || !file.Pkg.IsAtLeastGoVersion(lint.Go115) {
		// skip analysis for non-test files or for Go versions before 1.15
		return failures
	}

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := &lintRedundantTestMainExit{onFailure: onFailure}
	ast.Walk(w, file.AST)
	return failures
}

// Name returns the rule name.
func (*RedundantTestMainExitRule) Name() string {
	return "redundant-test-main-exit"
}

type lintRedundantTestMainExit struct {
	onFailure func(lint.Failure)
}

func (w *lintRedundantTestMainExit) Visit(node ast.Node) ast.Visitor {
	if fd, ok := node.(*ast.FuncDecl); ok {
		if fd.Name.Name != "TestMain" {
			return nil // skip analysis for other functions than TestMain
		}

		return w
	}

	se, ok := node.(*ast.ExprStmt)
	if !ok {
		return w
	}
	ce, ok := se.X.(*ast.CallExpr)
	if !ok {
		return w
	}

	fc, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok {
		return w
	}
	id, ok := fc.X.(*ast.Ident)
	if !ok {
		return w
	}

	pkg := id.Name
	// skip flag calls because they are commonly used in TestMain
	if pkg == "flag" {
		return w
	}

	fn := fc.Sel.Name
	if isCallToExitFunction(pkg, fn, ce.Args) {
		w.onFailure(lint.Failure{
			Confidence: 1,
			Node:       ce,
			Category:   lint.FailureCategoryStyle,
			Failure:    fmt.Sprintf("redundant call to %s.%s in TestMain function, the test runner will handle it automatically as of Go 1.15", pkg, fn),
		})
	}

	return w
}
