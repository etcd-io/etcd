package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// IdenticalSwitchBranchesRule warns on identical switch branches.
type IdenticalSwitchBranchesRule struct{}

// Apply applies the rule to given file.
func (*IdenticalSwitchBranchesRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	getStmtLine := func(s ast.Stmt) int {
		return file.ToPosition(s.Pos()).Line
	}

	w := &lintIdenticalSwitchBranches{getStmtLine: getStmtLine, onFailure: onFailure}
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
func (*IdenticalSwitchBranchesRule) Name() string {
	return "identical-switch-branches"
}

type lintIdenticalSwitchBranches struct {
	getStmtLine func(ast.Stmt) int
	onFailure   func(lint.Failure)
}

func (w *lintIdenticalSwitchBranches) Visit(node ast.Node) ast.Visitor {
	switchStmt, ok := node.(*ast.SwitchStmt)
	if !ok {
		return w
	}

	if switchStmt.Tag == nil {
		return w // do not lint untagged switches (order of case evaluation might be important)
	}

	doesFallthrough := func(stmts []ast.Stmt) bool {
		if len(stmts) == 0 {
			return false
		}

		ft, ok := stmts[len(stmts)-1].(*ast.BranchStmt)
		return ok && ft.Tok == token.FALLTHROUGH
	}

	hashes := map[string]int{} // map hash(branch code) -> branch line
	for _, cc := range switchStmt.Body.List {
		caseClause := cc.(*ast.CaseClause)
		if doesFallthrough(caseClause.Body) {
			continue // skip fallthrough branches
		}
		branch := &ast.BlockStmt{
			List: caseClause.Body,
		}
		hash := astutils.NodeHash(branch)
		branchLine := w.getStmtLine(caseClause)
		if matchLine, ok := hashes[hash]; ok {
			w.onFailure(lint.Failure{
				Confidence: 1.0,
				Node:       node,
				Category:   lint.FailureCategoryLogic,
				Failure:    fmt.Sprintf(`"switch" with identical branches (lines %d and %d)`, matchLine, branchLine),
			})
		}

		hashes[hash] = branchLine
		ast.Walk(w, branch)
	}

	return nil // switch branches already analyzed
}
