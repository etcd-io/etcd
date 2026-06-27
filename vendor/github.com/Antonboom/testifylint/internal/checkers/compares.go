package checkers

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Compares detects situations like
//
//	assert.True(t, a == b)
//	assert.True(t, a != b)
//	assert.True(t, a > b)
//	assert.True(t, a >= b)
//	assert.True(t, a < b)
//	assert.True(t, a <= b)
//	assert.False(t, a == b)
//	...
//
// and requires
//
//	assert.Equal(t, a, b)
//	assert.NotEqual(t, a, b)
//	assert.Greater(t, a, b)
//	assert.GreaterOrEqual(t, a, b)
//	assert.Less(t, a, b)
//	assert.LessOrEqual(t, a, b)
//
// If `a` and `b` are pointers then `assert.Same`/`NotSame` is required instead,
// due to the inappropriate recursive nature of `assert.Equal` (based on `reflect.DeepEqual`).
type Compares struct{}

// NewCompares constructs Compares checker.
func NewCompares() Compares   { return Compares{} }
func (Compares) Name() string { return "compares" }

func (checker Compares) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if len(call.Args) < 1 {
		return nil
	}

	be, ok := call.Args[0].(*ast.BinaryExpr)
	if !ok {
		return nil
	}

	var tokenToProposedFn map[token.Token]string

	switch call.Fn.NameFTrimmed {
	case "True":
		tokenToProposedFn = tokenToProposedFnInsteadOfTrue
	case "False":
		tokenToProposedFn = tokenToProposedFnInsteadOfFalse
	default:
		return nil
	}

	proposedFn, ok := tokenToProposedFn[be.Op]
	if !ok {
		return nil
	}

	_, xp := isPointer(pass, be.X)
	_, yp := isPointer(pass, be.Y)
	if xp && yp {
		switch proposedFn {
		case "Equal":
			proposedFn = "Same"
		case "NotEqual":
			proposedFn = "NotSame"
		}
	}

	a, b := be.X, be.Y
	return newUseFunctionDiagnostic(checker.Name(), call, proposedFn,
		analysis.TextEdit{
			Pos:     be.X.Pos(),
			End:     be.Y.End(),
			NewText: formatAsCallArgs(pass, a, b),
		})
}

var tokenToProposedFnInsteadOfTrue = map[token.Token]string{
	token.EQL: "Equal",
	token.NEQ: "NotEqual",
	token.GTR: "Greater",
	token.GEQ: "GreaterOrEqual",
	token.LSS: "Less",
	token.LEQ: "LessOrEqual",
}

var tokenToProposedFnInsteadOfFalse = map[token.Token]string{
	token.EQL: "NotEqual",
	token.NEQ: "Equal",
	token.GTR: "LessOrEqual",
	token.GEQ: "Less",
	token.LSS: "GreaterOrEqual",
	token.LEQ: "Greater",
}
