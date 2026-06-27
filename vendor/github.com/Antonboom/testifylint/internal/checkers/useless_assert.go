package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// UselessAssert detects useless asserts like
//
//	assert.Contains(t, tt.value, tt.value)
//	assert.ElementsMatch(t, tt.value, tt.value)
//	assert.Equal(t, tt.value, tt.value)
//	assert.EqualExportedValues(t, tt.value, tt.value)
//	...
//
//	assert.True(t, num > num)
//	assert.True(t, num < num)
//	assert.True(t, num >= num)
//	assert.True(t, num <= num)
//	assert.True(t, num == num)
//	assert.True(t, num != num)
//
//	assert.False(t, num > num)
//	assert.False(t, num < num)
//	assert.False(t, num >= num)
//	assert.False(t, num <= num)
//	assert.False(t, num == num)
//	assert.False(t, num != num)
//
//	assert.Empty(t, "value") // Any string literal.
//	assert.Error(t, nil)
//	assert.False(t, false) // Any bool literal.
//	assert.Implements(t, (*any)(nil), new(Conn))
//	assert.Negative(t, 42) // Any int literal.
//	assert.Nil(t, nil)
//	assert.NoError(t, nil)
//	assert.NotEmpty(t, "value") // Any string literal.
//	assert.NotImplements(t, (*any)(nil), new(Conn))
//	assert.NotNil(t, nil)
//	assert.NotZero(t, 42)      // Any int literal.
//	assert.NotZero(t, "value") // Any string literal.
//	assert.NotZero(t, nil)
//	assert.NotZero(t, false) // Any bool literal.
//	assert.Positive(t, 42)   // Any int literal.
//	assert.True(t, true)     // Any bool literal.
//	assert.Zero(t, 42)       // Any int literal.
//	assert.Zero(t, "value")  // Any string literal.
//	assert.Zero(t, nil)
//	assert.Zero(t, false) // Any bool literal.
//
//	assert.Negative(len(x))
//	assert.Less(len(x), 0)
//	assert.Greater(0, len(x))
//	assert.GreaterOrEqual(len(x), 0)
//	assert.LessOrEqual(0, len(x))
//
//	assert.Negative(uintVal)
//	assert.Less(uintVal, 0)
//	assert.Greater(0, uintVal)
//	assert.GreaterOrEqual(uintVal, 0)
//	assert.LessOrEqual(0, uintVal)
type UselessAssert struct{}

// NewUselessAssert constructs UselessAssert checker.
func NewUselessAssert() UselessAssert { return UselessAssert{} }
func (UselessAssert) Name() string    { return "useless-assert" }

func (checker UselessAssert) Check(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	if d := checker.checkSameVars(pass, call); d != nil {
		return d
	}

	var isMeaningless bool
	switch call.Fn.NameFTrimmed {
	case "False", "True":
		isMeaningless = (len(call.Args) >= 1) && isUntypedBool(pass, call.Args[0])

	case "GreaterOrEqual", "Less":
		isMeaningless = (len(call.Args) >= 2) && isAnyZero(call.Args[1]) && canNotBeNegative(pass, call.Args[0])

	case "Implements", "NotImplements":
		if len(call.Args) < 2 {
			return nil
		}

		elem, ok := isPointer(pass, call.Args[0])
		isMeaningless = ok && isEmptyInterfaceType(elem)

	case "LessOrEqual", "Greater":
		isMeaningless = (len(call.Args) >= 2) && isAnyZero(call.Args[0]) && canNotBeNegative(pass, call.Args[1])

	case "Positive":
		if len(call.Args) < 1 {
			return nil
		}
		_, isMeaningless = isIntBasicLit(call.Args[0])

	case "Negative":
		if len(call.Args) < 1 {
			return nil
		}
		_, isInt := isIntBasicLit(call.Args[0])
		isMeaningless = isInt || canNotBeNegative(pass, call.Args[0])

	case "Error", "Nil", "NoError", "NotNil":
		isMeaningless = (len(call.Args) >= 1) && isNil(call.Args[0])

	case "Empty", "NotEmpty":
		isMeaningless = (len(call.Args) >= 1) && isStringLit(call.Args[0])

	case "NotZero", "Zero":
		if len(call.Args) < 1 {
			return nil
		}
		_, isInt := isIntBasicLit(call.Args[0])
		isMeaningless = isInt || isStringLit(call.Args[0]) || isNil(call.Args[0]) || isUntypedBool(pass, call.Args[0])
	}

	if isMeaningless {
		return newDiagnostic(checker.Name(), call, "meaningless assertion")
	}
	return nil
}

func (checker UselessAssert) checkSameVars(pass *analysis.Pass, call *CallMeta) *analysis.Diagnostic {
	var first, second ast.Node

	switch call.Fn.NameFTrimmed {
	case
		"Contains",
		"ElementsMatch",
		"Equal",
		"EqualExportedValues",
		"EqualValues",
		"ErrorAs",
		"ErrorIs",
		"Exactly",
		"Greater",
		"GreaterOrEqual",
		"Implements",
		"InDelta",
		"InDeltaMapValues",
		"InDeltaSlice",
		"InEpsilon",
		"InEpsilonSlice",
		"IsNotType",
		"IsType",
		"JSONEq",
		"Less",
		"LessOrEqual",
		"NotElementsMatch",
		"NotEqual",
		"NotEqualValues",
		"NotErrorAs",
		"NotErrorIs",
		"NotRegexp",
		"NotSame",
		"NotSubset",
		"Regexp",
		"Same",
		"Subset",
		"WithinDuration",
		"YAMLEq":
		if len(call.Args) < 2 {
			return nil
		}
		first, second = call.Args[0], call.Args[1]

	case "True", "False":
		if len(call.Args) < 1 {
			return nil
		}

		be, ok := call.Args[0].(*ast.BinaryExpr)
		if !ok {
			return nil
		}
		first, second = be.X, be.Y

	default:
		return nil
	}

	if analysisutil.NodeString(pass.Fset, first) == analysisutil.NodeString(pass.Fset, second) {
		return newDiagnostic(checker.Name(), call, "asserting of the same variable")
	}
	return nil
}

func canNotBeNegative(pass *analysis.Pass, e ast.Expr) bool {
	_, isLen := isBuiltinLenCall(pass, e)
	return isLen || isUnsigned(pass, e)
}
