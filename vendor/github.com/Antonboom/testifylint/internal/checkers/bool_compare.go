package checkers

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// BoolCompare detects situations like
//
//	assert.Equal(t, false, result)
//	assert.EqualValues(t, false, result)
//	assert.Exactly(t, false, result)
//	assert.NotEqual(t, true, result)
//	assert.NotEqualValues(t, true, result)
//	assert.False(t, !result)
//	assert.True(t, result == true)
//	...
//
// and requires
//
//	assert.False(t, result)
//	assert.True(t, result)
type BoolCompare struct {
	ignoreCustomTypes bool
}

// NewBoolCompare constructs BoolCompare checker.
func NewBoolCompare() *BoolCompare { return new(BoolCompare) }
func (BoolCompare) Name() string   { return "bool-compare" }

func (checker *BoolCompare) SetIgnoreCustomTypes(v bool) *BoolCompare {
	checker.ignoreCustomTypes = v
	return checker
}

func (checker BoolCompare) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	newBoolCast := func(e ast.Expr) ast.Expr {
		return &ast.CallExpr{Fun: &ast.Ident{Name: "bool"}, Args: []ast.Expr{e}}
	}

	newUseFnDiagnostic := func(proposed string, survivingArg ast.Expr, replaceStart, replaceEnd token.Pos) *analysis.Diagnostic {
		if !hasBoolType(pass, survivingArg) {
			if checker.ignoreCustomTypes {
				return nil
			}
			survivingArg = newBoolCast(survivingArg)
		}
		return newUseFunctionDiagnostic(checker.Name(), call, proposed, analysis.TextEdit{
			Pos:     replaceStart,
			End:     replaceEnd,
			NewText: analysisutil.NodeBytes(pass.Fset, survivingArg),
		})
	}

	newUseTrueDiagnostic := func(survivingArg ast.Expr, replaceStart, replaceEnd token.Pos) *analysis.Diagnostic {
		return newUseFnDiagnostic("True", survivingArg, replaceStart, replaceEnd)
	}

	newUseFalseDiagnostic := func(survivingArg ast.Expr, replaceStart, replaceEnd token.Pos) *analysis.Diagnostic {
		return newUseFnDiagnostic("False", survivingArg, replaceStart, replaceEnd)
	}

	newNeedSimplifyDiagnostic := func(survivingArg ast.Expr, replaceStart, replaceEnd token.Pos) *analysis.Diagnostic {
		if !hasBoolType(pass, survivingArg) {
			if checker.ignoreCustomTypes {
				return nil
			}
			survivingArg = newBoolCast(survivingArg)
		}
		return newDiagnostic(checker.Name(), call, "need to simplify the assertion",
			analysis.SuggestedFix{
				Message: "Simplify the assertion",
				TextEdits: []analysis.TextEdit{{
					Pos:     replaceStart,
					End:     replaceEnd,
					NewText: analysisutil.NodeBytes(pass.Fset, survivingArg),
				}},
			},
		)
	}

	switch call.Fn.NameFTrimmed {
	case "Equal", "EqualValues", "Exactly":
		if len(call.Args) < 2 {
			return nil
		}

		arg1, arg2 := call.Args[0], call.Args[1]
		if anyCondSatisfaction(pass, isEmptyInterface, arg1, arg2) {
			return nil
		}
		if anyCondSatisfaction(pass, isBoolOverride, arg1, arg2) {
			return nil
		}

		t1, t2 := isUntypedTrue(pass, arg1), isUntypedTrue(pass, arg2)
		f1, f2 := isUntypedFalse(pass, arg1), isUntypedFalse(pass, arg2)

		switch {
		case xor(t1, t2):
			survivingArg, _ := anyVal([]bool{t1, t2}, arg2, arg1)
			if call.Fn.NameFTrimmed == "Exactly" && !hasBoolType(pass, survivingArg) {
				// NOTE(a.telyshev): `Exactly` assumes no type conversion.
				return nil
			}
			return newUseTrueDiagnostic(survivingArg, arg1.Pos(), arg2.End())

		case xor(f1, f2):
			survivingArg, _ := anyVal([]bool{f1, f2}, arg2, arg1)
			if call.Fn.NameFTrimmed == "Exactly" && !hasBoolType(pass, survivingArg) {
				// NOTE(a.telyshev): `Exactly` assumes no type conversion.
				return nil
			}
			return newUseFalseDiagnostic(survivingArg, arg1.Pos(), arg2.End())
		}

	case "NotEqual", "NotEqualValues":
		if len(call.Args) < 2 {
			return nil
		}

		arg1, arg2 := call.Args[0], call.Args[1]
		if anyCondSatisfaction(pass, isEmptyInterface, arg1, arg2) {
			return nil
		}
		if anyCondSatisfaction(pass, isBoolOverride, arg1, arg2) {
			return nil
		}

		t1, t2 := isUntypedTrue(pass, arg1), isUntypedTrue(pass, arg2)
		f1, f2 := isUntypedFalse(pass, arg1), isUntypedFalse(pass, arg2)

		switch {
		case xor(t1, t2):
			survivingArg, _ := anyVal([]bool{t1, t2}, arg2, arg1)
			return newUseFalseDiagnostic(survivingArg, arg1.Pos(), arg2.End())

		case xor(f1, f2):
			survivingArg, _ := anyVal([]bool{f1, f2}, arg2, arg1)
			return newUseTrueDiagnostic(survivingArg, arg1.Pos(), arg2.End())
		}

	case "True":
		if len(call.Args) < 1 {
			return nil
		}
		expr := call.Args[0]

		{
			arg1, ok1 := isComparisonWithTrue(pass, expr, token.EQL)
			arg2, ok2 := isComparisonWithFalse(pass, expr, token.NEQ)

			survivingArg, ok := anyVal([]bool{ok1, ok2}, arg1, arg2)
			if ok && !isEmptyInterface(pass, survivingArg) {
				return newNeedSimplifyDiagnostic(survivingArg, expr.Pos(), expr.End())
			}
		}

		{
			arg1, ok1 := isComparisonWithTrue(pass, expr, token.NEQ)
			arg2, ok2 := isComparisonWithFalse(pass, expr, token.EQL)
			arg3, ok3 := isNegation(expr)

			survivingArg, ok := anyVal([]bool{ok1, ok2, ok3}, arg1, arg2, arg3)
			if ok && !isEmptyInterface(pass, survivingArg) {
				return newUseFalseDiagnostic(survivingArg, expr.Pos(), expr.End())
			}
		}

	case "False":
		if len(call.Args) < 1 {
			return nil
		}
		expr := call.Args[0]

		{
			arg1, ok1 := isComparisonWithTrue(pass, expr, token.EQL)
			arg2, ok2 := isComparisonWithFalse(pass, expr, token.NEQ)

			survivingArg, ok := anyVal([]bool{ok1, ok2}, arg1, arg2)
			if ok && !isEmptyInterface(pass, survivingArg) {
				return newNeedSimplifyDiagnostic(survivingArg, expr.Pos(), expr.End())
			}
		}

		{
			arg1, ok1 := isComparisonWithTrue(pass, expr, token.NEQ)
			arg2, ok2 := isComparisonWithFalse(pass, expr, token.EQL)
			arg3, ok3 := isNegation(expr)

			survivingArg, ok := anyVal([]bool{ok1, ok2, ok3}, arg1, arg2, arg3)
			if ok && !isEmptyInterface(pass, survivingArg) {
				return newUseTrueDiagnostic(survivingArg, expr.Pos(), expr.End())
			}
		}
	}
	return nil
}

func isNegation(e ast.Expr) (ast.Expr, bool) {
	ue, ok := e.(*ast.UnaryExpr)
	if !ok {
		return nil, false
	}
	return ue.X, ue.Op == token.NOT
}
