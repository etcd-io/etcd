package matcher

import (
	"go/ast"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
	"github.com/nunnatsa/ginkgolinter/internal/typecheck"
)

type HaveOccurredMatcher struct{}

func (m *HaveOccurredMatcher) Type() Type {
	return HaveOccurredMatcherType
}
func (m *HaveOccurredMatcher) MatcherName() string {
	return haveOccurred
}

type SucceedMatcher struct{}

func (m *SucceedMatcher) Type() Type {
	return SucceedMatcherType
}
func (m *SucceedMatcher) MatcherName() string {
	return succeed
}

type MatchErrorMatcher interface {
	Info
	AllowedNumArgs() int
	NumArgs() int
}

type InvalidMatchErrorMatcher struct {
	firstAgr ast.Expr
	numArgs  int
}

func (m *InvalidMatchErrorMatcher) Type() Type {
	return MatchErrorMatcherType
}

func (m *InvalidMatchErrorMatcher) MatcherName() string {
	return matchError
}

func (m *InvalidMatchErrorMatcher) AllowedNumArgs() int {
	return 1
}

func (m *InvalidMatchErrorMatcher) NumArgs() int {
	return m.numArgs
}

func (m *InvalidMatchErrorMatcher) GetValueExpr() ast.Expr {
	return m.firstAgr
}

type MatchErrorMatcherWithErr struct {
	numArgs int
}

func (m *MatchErrorMatcherWithErr) Type() Type {
	return MatchErrorMatcherType | ErrMatchWithErr
}

func (m *MatchErrorMatcherWithErr) MatcherName() string {
	return matchError
}

func (m *MatchErrorMatcherWithErr) AllowedNumArgs() int {
	return 1
}

func (m *MatchErrorMatcherWithErr) NumArgs() int {
	return m.numArgs
}

type MatchErrorMatcherWithErrFunc struct {
	numArgs           int
	secondArgIsString bool
}

func (m *MatchErrorMatcherWithErrFunc) Type() Type {
	return MatchErrorMatcherType | ErrMatchWithErrFunc
}

func (m *MatchErrorMatcherWithErrFunc) MatcherName() string {
	return matchError
}

func (m *MatchErrorMatcherWithErrFunc) AllowedNumArgs() int {
	return 2
}

func (m *MatchErrorMatcherWithErrFunc) NumArgs() int {
	return m.numArgs
}

func (m *MatchErrorMatcherWithErrFunc) IsSecondArgString() bool {
	return m.secondArgIsString
}

type MatchErrorMatcherWithString struct {
	numArgs int
}

func (m *MatchErrorMatcherWithString) Type() Type {
	return MatchErrorMatcherType | ErrMatchWithString
}

func (m *MatchErrorMatcherWithString) MatcherName() string {
	return matchError
}

func (m *MatchErrorMatcherWithString) AllowedNumArgs() int {
	return 1
}

func (m *MatchErrorMatcherWithString) NumArgs() int {
	return m.numArgs
}

type MatchErrorMatcherWithMatcher struct {
	numArgs int
}

func (m *MatchErrorMatcherWithMatcher) Type() Type {
	return MatchErrorMatcherType | ErrMatchWithMatcher
}

func (m *MatchErrorMatcherWithMatcher) MatcherName() string {
	return matchError
}

func (m *MatchErrorMatcherWithMatcher) AllowedNumArgs() int {
	return 1
}

func (m *MatchErrorMatcherWithMatcher) NumArgs() int {
	return m.numArgs
}

func newMatchErrorMatcher(args []ast.Expr, pass *analysis.Pass) MatchErrorMatcher {
	numArgs := len(args)
	if value.IsExprError(pass, args[0]) {
		return &MatchErrorMatcherWithErr{numArgs: numArgs}
	}

	t := pass.TypesInfo.TypeOf(args[0])
	if isString(args[0], pass) {
		return &MatchErrorMatcherWithString{numArgs: numArgs}
	}

	if typecheck.ImplementsGomegaMatcher(t) {
		return &MatchErrorMatcherWithMatcher{numArgs: numArgs}
	}

	if isFuncErrBool(t) {
		isString := false
		if numArgs > 1 {
			t2 := pass.TypesInfo.TypeOf(args[1])
			isString = gotypes.Identical(t2, gotypes.Typ[gotypes.String])
		}
		return &MatchErrorMatcherWithErrFunc{numArgs: numArgs, secondArgIsString: isString}
	}

	return &InvalidMatchErrorMatcher{numArgs: numArgs}
}

func isString(exp ast.Expr, pass *analysis.Pass) bool {
	t := pass.TypesInfo.TypeOf(exp)
	return gotypes.Identical(t, gotypes.Typ[gotypes.String])
}

// isFuncErrBool checks if a function is with the signature `func(error) bool`
func isFuncErrBool(t gotypes.Type) bool {
	sig, ok := t.(*gotypes.Signature)
	if !ok {
		return false
	}
	if sig.Params().Len() != 1 || sig.Results().Len() != 1 {
		return false
	}

	if !typecheck.ImplementsError(sig.Params().At(0).Type()) {
		return false
	}

	b, ok := sig.Results().At(0).Type().(*gotypes.Basic)
	if ok && b.Name() == "bool" && b.Info() == gotypes.IsBoolean && b.Kind() == gotypes.Bool {
		return true
	}

	return false
}
