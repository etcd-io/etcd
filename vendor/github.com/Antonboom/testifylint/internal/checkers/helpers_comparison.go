package checkers

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

func isComparisonWithFloat(p *analysis.Pass, e ast.Expr, op token.Token) bool {
	be, ok := e.(*ast.BinaryExpr)
	if !ok {
		return false
	}
	return be.Op == op && (isFloat(p, be.X) || isFloat(p, be.Y))
}

func isComparisonWithTrue(pass *analysis.Pass, e ast.Expr, op token.Token) (ast.Expr, bool) {
	return isComparisonWith(pass, e, isUntypedTrue, op)
}

func isComparisonWithFalse(pass *analysis.Pass, e ast.Expr, op token.Token) (ast.Expr, bool) {
	return isComparisonWith(pass, e, isUntypedFalse, op)
}

type predicate func(pass *analysis.Pass, e ast.Expr) bool

func isComparisonWith(
	pass *analysis.Pass,
	e ast.Expr,
	predicate predicate,
	op token.Token,
) (ast.Expr, bool) {
	be, ok := e.(*ast.BinaryExpr)
	if !ok {
		return nil, false
	}
	if be.Op != op {
		return nil, false
	}

	t1, t2 := predicate(pass, be.X), predicate(pass, be.Y)
	if xor(t1, t2) {
		if t1 {
			return be.Y, true
		}
		return be.X, true
	}
	return nil, false
}

func isStrictComparisonWith(
	pass *analysis.Pass,
	e ast.Expr,
	lhs predicate,
	op token.Token,
	rhs predicate,
) (leftOperand ast.Expr, rightOperand ast.Expr, fact bool) {
	be, ok := e.(*ast.BinaryExpr)
	if !ok {
		return nil, nil, false
	}

	if be.Op == op && lhs(pass, be.X) && rhs(pass, be.Y) {
		return be.X, be.Y, true
	}
	return nil, nil, false
}
