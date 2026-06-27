package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// IdenticalSwitchConditionsRule warns on switch case clauses with identical conditions.
type IdenticalSwitchConditionsRule struct{}

// Apply applies the rule to given file.
func (*IdenticalSwitchConditionsRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	w := &lintIdenticalSwitchConditions{toPosition: file.ToPosition, onFailure: onFailure}
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
func (*IdenticalSwitchConditionsRule) Name() string {
	return "identical-switch-conditions"
}

type lintIdenticalSwitchConditions struct {
	toPosition func(token.Pos) token.Position
	onFailure  func(lint.Failure)
}

func (w *lintIdenticalSwitchConditions) Visit(node ast.Node) ast.Visitor {
	switchStmt, ok := node.(*ast.SwitchStmt)
	if !ok { // not a switch statement, keep walking the AST
		return w
	}

	if switchStmt.Tag != nil {
		return w // Not interested in tagged switches
	}

	hashes := map[string]int{} // map hash(condition code) -> condition line
	for _, cc := range switchStmt.Body.List {
		caseClause := cc.(*ast.CaseClause)
		caseClauseLine := w.toPosition(caseClause.Pos()).Line
		for _, expr := range caseClause.List {
			hash := astutils.NodeHash(expr)
			if matchLine, ok := hashes[hash]; ok {
				w.onFailure(lint.Failure{
					Confidence: 1.0,
					Node:       caseClause,
					Category:   lint.FailureCategoryLogic,
					Failure:    fmt.Sprintf(`case clause at line %d has the same condition`, matchLine),
				})
			}

			hashes[hash] = caseClauseLine
		}

		ast.Walk(w, caseClause)
	}

	return nil // switch branches already analyzed
}
