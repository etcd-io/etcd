package rule

import (
	"go/ast"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// IdenticalBranchesRule warns on if...else statements with both branches being the same.
type IdenticalBranchesRule struct{}

// Apply applies the rule to given file.
func (*IdenticalBranchesRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := &lintIdenticalBranches{onFailure: onFailure}
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		ast.Walk(w, fn.Body)
	}

	return failures
}

// Name returns the rule name.
func (*IdenticalBranchesRule) Name() string {
	return "identical-branches"
}

type lintIdenticalBranches struct {
	onFailure func(lint.Failure)
}

func (w *lintIdenticalBranches) Visit(node ast.Node) ast.Visitor {
	ifStmt, ok := node.(*ast.IfStmt)
	if !ok {
		return w
	}

	if ifStmt.Else == nil {
		return w // if without else
	}

	elseBranch, ok := ifStmt.Else.(*ast.BlockStmt)
	if !ok { // if-else-if construction, the rule only copes with single if...else statements
		return w
	}

	if w.identicalBranches(ifStmt.Body, elseBranch) {
		w.onFailure(lint.Failure{
			Confidence: 1.0,
			Node:       ifStmt,
			Category:   lint.FailureCategoryLogic,
			Failure:    "both branches of the if are identical",
		})
	}

	ast.Walk(w, ifStmt.Body)
	ast.Walk(w, ifStmt.Else)
	return nil
}

func (*lintIdenticalBranches) identicalBranches(body, elseBranch *ast.BlockStmt) bool {
	if len(body.List) != len(elseBranch.List) {
		return false // branches don't have the same number of statements
	}

	bodyStr := astutils.GoFmt(body)
	elseStr := astutils.GoFmt(elseBranch)

	return bodyStr == elseStr
}
