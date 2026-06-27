package checkers

import (
	"go/ast"
	"go/token"
	"regexp"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// DefaultExpectedVarPattern matches variables with "expected" or "wanted" prefix or suffix in the name.
var DefaultExpectedVarPattern = regexp.MustCompile(
	`(^(exp(ected)?|want(ed)?)([A-Z]\w*)?$)|(^(\w*[a-z])?(Exp(ected)?|Want(ed)?)$)`)

// ExpectedActual detects situations like
//
//	assert.Equal(t, result, expected)
//	assert.Equal(t, result, len(expected))
//	assert.Equal(t, len(resultFields), len(expectedFields))
//	assert.EqualExportedValues(t, resultObj, User{Name: "Anton"})
//	assert.EqualValues(t, result, 42)
//	assert.Exactly(t, result, int64(42))
//	assert.JSONEq(t, result, `{"version": 3}`)
//	assert.InDelta(t, result, 42.42, 1.0)
//	assert.InDeltaMapValues(t, result, map[string]float64{"score": 0.99}, 1.0)
//	assert.InDeltaSlice(t, result, []float64{0.98, 0.99}, 1.0)
//	assert.InEpsilon(t, result, 42.42, 0.0001)
//	assert.InEpsilonSlice(t, result, []float64{0.9801, 0.9902}, 0.0001)
//	assert.IsType(t, result, (*User)(nil))
//	assert.NotEqual(t, result, "expected")
//	assert.NotEqualValues(t, result, "expected")
//	assert.NotSame(t, resultPtr, &value)
//	assert.Same(t, resultPtr, &value)
//	assert.WithinDuration(t, resultTime, time.Date(2023, 01, 12, 11, 46, 33, 0, nil), time.Second)
//	assert.YAMLEq(t, result, "version: '3'")
//
// and requires
//
//	assert.Equal(t, expected, result)
//	assert.Equal(t, len(expected), result)
//	assert.Equal(t, len(expectedFields), len(resultFields))
//	assert.EqualExportedValues(t, User{Name: "Anton"}, resultObj)
//	assert.EqualValues(t, 42, result)
//	...
type ExpectedActual struct {
	expVarPattern *regexp.Regexp
}

// NewExpectedActual constructs ExpectedActual checker using DefaultExpectedVarPattern.
func NewExpectedActual() *ExpectedActual {
	return &ExpectedActual{expVarPattern: DefaultExpectedVarPattern}
}

func (ExpectedActual) Name() string { return "expected-actual" }

func (checker *ExpectedActual) SetExpVarPattern(p *regexp.Regexp) *ExpectedActual {
	if p != nil {
		checker.expVarPattern = p
	}
	return checker
}

func (checker ExpectedActual) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	switch call.Fn.NameFTrimmed {
	case "Equal",
		"EqualExportedValues",
		"EqualValues",
		"Exactly",
		"InDelta",
		"InDeltaMapValues",
		"InDeltaSlice",
		"InEpsilon",
		"InEpsilonSlice",
		"IsNotType",
		"IsType",
		"JSONEq",
		"NotEqual",
		"NotEqualValues",
		"NotSame",
		"Same",
		"WithinDuration",
		"YAMLEq":
	default:
		return nil
	}

	if len(call.Args) < 2 {
		return nil
	}
	first, second := call.Args[0], call.Args[1]

	if checker.isWrongExpectedActualOrder(pass, first, second) {
		return newDiagnostic(checker.Name(), call, "need to reverse actual and expected values", analysis.SuggestedFix{
			Message: "Reverse actual and expected values",
			TextEdits: []analysis.TextEdit{
				{
					Pos:     first.Pos(),
					End:     second.End(),
					NewText: formatAsCallArgs(pass, second, first),
				},
			},
		})
	}
	return nil
}

func (checker ExpectedActual) isWrongExpectedActualOrder(pass *analysis.Pass, first, second ast.Expr) bool {
	leftIsCandidate := checker.isExpectedValueCandidate(pass, first)
	rightIsCandidate := checker.isExpectedValueCandidate(pass, second)
	return rightIsCandidate && !leftIsCandidate
}

func (checker ExpectedActual) isExpectedValueCandidate(pass *analysis.Pass, expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.ParenExpr:
		return checker.isExpectedValueCandidate(pass, v.X)

	case *ast.StarExpr: // *value
		return checker.isExpectedValueCandidate(pass, v.X)

	case *ast.UnaryExpr:
		if v.Op == token.AND || v.Op == token.SUB { // &value, -value
			return checker.isExpectedValueCandidate(pass, v.X)
		}

	case *ast.CompositeLit:
		return true

	case *ast.CallExpr:
		if lv, ok := isBuiltinLenCall(pass, expr); ok {
			return isIdentNamedAfterPattern(checker.expVarPattern, lv)
		}
		return isParenExpr(v) ||
			isCastedBasicLitOrExpectedValue(v, checker.expVarPattern) ||
			isExpectedValueFactory(pass, v, checker.expVarPattern)
	}

	return isBasicLit(expr) ||
		isUntypedConst(pass, expr) ||
		isTypedConst(pass, expr) ||
		isIdentNamedAfterPattern(checker.expVarPattern, expr) ||
		isStructVarNamedAfterPattern(checker.expVarPattern, expr) ||
		isStructFieldNamedAfterPattern(checker.expVarPattern, expr)
}

func isParenExpr(ce *ast.CallExpr) bool {
	_, ok := ce.Fun.(*ast.ParenExpr)
	return ok
}

func isCastedBasicLitOrExpectedValue(ce *ast.CallExpr, pattern *regexp.Regexp) bool {
	if len(ce.Args) != 1 {
		return false
	}

	fn, ok := ce.Fun.(*ast.Ident)
	if !ok {
		return false
	}

	switch fn.Name {
	case "complex64", "complex128":
		return true

	case "uint", "uint8", "uint16", "uint32", "uint64",
		"int", "int8", "int16", "int32", "int64",
		"float32", "float64",
		"rune", "string":
		return isBasicLit(ce.Args[0]) || isIdentNamedAfterPattern(pattern, ce.Args[0])
	}
	return false
}

func isExpectedValueFactory(pass *analysis.Pass, ce *ast.CallExpr, pattern *regexp.Regexp) bool {
	switch fn := ce.Fun.(type) {
	case *ast.Ident:
		return pattern.MatchString(fn.Name)

	case *ast.SelectorExpr:
		timeDateFn := analysisutil.ObjectOf(pass.Pkg, "time", "Date")
		if timeDateFn != nil && analysisutil.IsObj(pass.TypesInfo, fn.Sel, timeDateFn) {
			return true
		}
		return pattern.MatchString(fn.Sel.Name)
	}
	return false
}
