package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

func xor(a, b bool) bool {
	return a != b
}

// anyVal returns the first value[i] for which bools[i] is true.
func anyVal[T any](bools []bool, vals ...T) (T, bool) {
	if len(bools) != len(vals) {
		panic("inconsistent usage of valOr") //nolint:forbidigo // Does not depend on the code being analyzed.
	}

	for i, b := range bools {
		if b {
			return vals[i], true
		}
	}

	var _default T
	return _default, false
}

func anyCondSatisfaction(pass *analysis.Pass, p predicate, vals ...ast.Expr) bool {
	for _, v := range vals {
		if p(pass, v) {
			return true
		}
	}
	return false
}

// p transforms simple is-function in a predicate.
func p(fn func(e ast.Expr) bool) predicate {
	return func(_ *analysis.Pass, e ast.Expr) bool {
		return fn(e)
	}
}
