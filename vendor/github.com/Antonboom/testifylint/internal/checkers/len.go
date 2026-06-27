package checkers

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Len detects situations like
//
//	assert.Equal(t, 42, len(arr))
//	assert.Equal(t, len(arr), 42)
//	assert.EqualValues(t, 42, len(arr))
//	assert.EqualValues(t, len(arr), 42)
//	assert.Exactly(t, 42, len(arr))
//	assert.Exactly(t, len(arr), 42)
//	assert.True(t, 42 == len(arr))
//	assert.True(t, len(arr) == 42)
//
//	assert.Equal(t, value, len(arr))
//	assert.EqualValues(t, value, len(arr))
//	assert.Exactly(t, value, len(arr))
//	assert.True(t, len(arr) == value)
//
//	assert.Equal(t, len(expArr), len(arr))
//	assert.EqualValues(t, len(expArr), len(arr))
//	assert.Exactly(t, len(expArr), len(arr))
//	assert.True(t, len(arr) == len(expArr))
//
// and requires
//
//	assert.Len(t, arr, 42)
//	assert.Len(t, arr, value)
//	assert.Len(t, arr, len(expArr))
//
// The checker ignores assertions in which length checking is not a priority, e.g
//
//	assert.Equal(t, len(arr), value)
type Len struct{}

// NewLen constructs Len checker.
func NewLen() Len        { return Len{} }
func (Len) Name() string { return "len" }

func (checker Len) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	switch call.Fn.NameFTrimmed {
	case "Equal", "EqualValues", "Exactly":
		if len(call.Args) < 2 {
			return nil
		}

		a, b := call.Args[0], call.Args[1]
		return checker.checkArgs(call, pass, a, b, false)

	case "True":
		if len(call.Args) < 1 {
			return nil
		}

		be, ok := call.Args[0].(*ast.BinaryExpr)
		if !ok {
			return nil
		}
		if be.Op != token.EQL {
			return nil
		}
		return checker.checkArgs(call, pass, be.Y, be.X, true) // In True, the actual value is usually first.
	}
	return nil
}

func (checker Len) checkArgs(call *CallMeta, pass *analysis.Pass, a, b ast.Expr, inverted bool) *analysis.Diagnostic {
	newUseLenDiagnostic := func(lenArg, expectedLen ast.Expr) *analysis.Diagnostic {
		const proposedFn = "Len"
		start, end := a.Pos(), b.End()
		if inverted {
			start, end = b.Pos(), a.End()
		}
		return newUseFunctionDiagnostic(checker.Name(), call, proposedFn,
			analysis.TextEdit{
				Pos:     start,
				End:     end,
				NewText: formatAsCallArgs(pass, lenArg, expectedLen),
			})
	}

	arg1, firstIsLen := isBuiltinLenCall(pass, a)
	arg2, secondIsLen := isBuiltinLenCall(pass, b)

	switch {
	case firstIsLen && secondIsLen:
		return newUseLenDiagnostic(arg2, a)

	case firstIsLen:
		if _, secondIsNum := isIntBasicLit(b); secondIsNum {
			return newUseLenDiagnostic(arg1, b)
		}

	case secondIsLen:
		return newUseLenDiagnostic(arg2, a)
	}

	return nil
}
