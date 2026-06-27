package rule

import (
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// ConstantLogicalExprRule warns on constant logical expressions.
type ConstantLogicalExprRule struct{}

// Apply applies the rule to given file.
func (*ConstantLogicalExprRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	astFile := file.AST
	w := &lintConstantLogicalExpr{astFile, onFailure}
	ast.Walk(w, astFile)
	return failures
}

// Name returns the rule name.
func (*ConstantLogicalExprRule) Name() string {
	return "constant-logical-expr"
}

type lintConstantLogicalExpr struct {
	file      *ast.File
	onFailure func(lint.Failure)
}

func (w *lintConstantLogicalExpr) Visit(node ast.Node) ast.Visitor {
	if n, ok := node.(*ast.BinaryExpr); ok {
		if !w.isOperatorWithLogicalResult(n.Op) {
			return w
		}

		subExpressionsAreNotEqual := astutils.GoFmt(n.X) != astutils.GoFmt(n.Y)
		if subExpressionsAreNotEqual {
			return w // nothing to say
		}

		// Handles cases like: a <= a, a == a, a >= a
		if w.isEqualityOperator(n.Op) {
			w.newFailure(n, "expression always evaluates to true")
			return w
		}

		// Handles cases like: a < a, a > a, a != a
		if w.isInequalityOperator(n.Op) {
			w.newFailure(n, "expression always evaluates to false")
			return w
		}

		w.newFailure(n, "left and right hand-side sub-expressions are the same")
	}

	return w
}

func (*lintConstantLogicalExpr) isOperatorWithLogicalResult(t token.Token) bool {
	switch t {
	case token.LAND, token.LOR, token.EQL, token.LSS, token.GTR, token.NEQ, token.LEQ, token.GEQ:
		return true
	}

	return false
}

func (*lintConstantLogicalExpr) isEqualityOperator(t token.Token) bool {
	switch t {
	case token.EQL, token.LEQ, token.GEQ:
		return true
	}

	return false
}

func (*lintConstantLogicalExpr) isInequalityOperator(t token.Token) bool {
	switch t {
	case token.LSS, token.GTR, token.NEQ:
		return true
	}

	return false
}

func (w *lintConstantLogicalExpr) newFailure(node ast.Node, msg string) {
	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       node,
		Category:   lint.FailureCategoryLogic,
		Failure:    msg,
	})
}
