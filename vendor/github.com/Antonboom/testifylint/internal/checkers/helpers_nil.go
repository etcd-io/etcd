package checkers

import "go/ast"

func xorNil(first, second ast.Expr) (ast.Expr, bool) {
	a, b := isNil(first), isNil(second)
	if xor(a, b) {
		if a {
			return second, true
		}
		return first, true
	}
	return nil, false
}

func isNil(expr ast.Expr) bool {
	return isIdentWithName("nil", expr)
}
