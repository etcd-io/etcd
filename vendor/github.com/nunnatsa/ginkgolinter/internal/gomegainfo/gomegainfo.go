package gomegainfo

import (
	"go/ast"
	gotypes "go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const ( // gomega actual method names
	expect                 = "Expect"
	expectWithOffset       = "ExpectWithOffset"
	omega                  = "Ω"
	eventually             = "Eventually"
	eventuallyWithOffset   = "EventuallyWithOffset"
	consistently           = "Consistently"
	consistentlyWithOffset = "ConsistentlyWithOffset"
)

const ( // assertion methods
	to        = "To"
	toNot     = "ToNot"
	notTo     = "NotTo"
	should    = "Should"
	shouldNot = "ShouldNot"
)

var funcOffsetMap = map[string]int{
	expect:                 0,
	expectWithOffset:       1,
	omega:                  0,
	eventually:             0,
	eventuallyWithOffset:   1,
	consistently:           0,
	consistentlyWithOffset: 1,
}

func ActualArgOffset(methodName string) int {
	funcOffset, ok := funcOffsetMap[methodName]
	if !ok {
		// Assume first argument for unknown methods.
		return 0
	}
	return funcOffset
}

func GetAllowedAssertionMethods(actualMethodName string) string {
	switch actualMethodName {
	case expect, expectWithOffset:
		return `"To()", "ToNot()" or "NotTo()"`

	case eventually, eventuallyWithOffset, consistently, consistentlyWithOffset:
		return `"Should()" or "ShouldNot()"`

	case omega:
		return `"Should()", "To()", "ShouldNot()", "ToNot()" or "NotTo()"`

	default:
		// Unknown wrapper or missing method name, mention all options.
		return `one of "To/NotTo/ToNot" (for Expect assertions) or "Should/ShouldNot" (for Eventually/Consistently assertions)`
	}
}

func IsAssertionFunc(name string) bool {
	switch name {
	case to, toNot, notTo, should, shouldNot:
		return true
	}
	return false
}

func IsGomegaVar(x ast.Expr, pass *analysis.Pass) bool {
	if _, isIdent := x.(*ast.Ident); !isIdent {
		return false
	}

	tx, ok := pass.TypesInfo.Types[x]
	if !ok {
		return false
	}

	return IsGomegaType(tx.Type)
}

const (
	gomegaStructType = "github.com/onsi/gomega/internal.Gomega"
	gomegaInterface  = "github.com/onsi/gomega/types.Gomega"
)

func IsGomegaType(t gotypes.Type) bool {
	switch ttx := gotypes.Unalias(t).(type) {
	case *gotypes.Pointer:
		return IsGomegaType(ttx.Elem())

	case *gotypes.Named:
		name := ttx.String()
		return strings.HasSuffix(name, gomegaStructType) || strings.HasSuffix(name, gomegaInterface)
	}

	return false
}
