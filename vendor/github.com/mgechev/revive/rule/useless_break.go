package rule

import (
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/lint"
)

// UselessBreak lint rule.
type UselessBreak struct{}

// Apply applies the rule to given file.
func (*UselessBreak) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	astFile := file.AST
	w := &lintUselessBreak{onFailure, false}
	ast.Walk(w, astFile)
	return failures
}

// Name returns the rule name.
func (*UselessBreak) Name() string {
	return "useless-break"
}

type lintUselessBreak struct {
	onFailure  func(lint.Failure)
	inLoopBody bool
}

func (w *lintUselessBreak) Visit(node ast.Node) ast.Visitor {
	switch v := node.(type) {
	case *ast.ForStmt:
		w.inLoopBody = true
		ast.Walk(w, v.Body)
		w.inLoopBody = false
		return nil
	case *ast.RangeStmt:
		w.inLoopBody = true
		ast.Walk(w, v.Body)
		w.inLoopBody = false
		return nil
	case *ast.CommClause:
		w.inspectCaseStatement(v.Body)
		return nil
	case *ast.CaseClause:
		w.inspectCaseStatement(v.Body)
		return nil
	}
	return w
}

func (w *lintUselessBreak) inspectCaseStatement(body []ast.Stmt) {
	l := len(body)
	if l == 0 {
		return // empty body, nothing to do
	}

	s := body[l-1] // pick the last statement
	if !isUnlabelledBreak(s) {
		return
	}

	msg := "useless break in case clause"
	if w.inLoopBody {
		msg += " (WARN: this break statement affects this switch or select statement and not the loop enclosing it)"
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       s,
		Failure:    msg,
	})
}

func isUnlabelledBreak(stmt ast.Stmt) bool {
	s, ok := stmt.(*ast.BranchStmt)
	return ok && s.Tok == token.BREAK && s.Label == nil
}
