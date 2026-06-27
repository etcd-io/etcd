package matcher

import (
	"go/ast"
	"go/constant"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
)

func newEqualMatcher(orig, clone ast.Expr, pass *analysis.Pass) Info {
	t := pass.TypesInfo.Types[orig]

	if t.Value != nil {
		if t.Value.Kind() == constant.Bool {
			if t.Value.String() == "true" {
				return &EqualTrueMatcher{}
			}
			return &EqualFalseMatcher{}
		}
	}

	if value.IsNil(orig, pass) {
		return &EqualNilMatcher{
			gotype: pass.TypesInfo.TypeOf(orig),
		}
	}

	val := value.GetValuer(orig, clone, pass)

	return &EqualMatcher{
		val: val,
	}
}

type EqualMatcher struct {
	val value.Valuer
}

func (EqualMatcher) Type() Type {
	return EqualMatcherType
}

func (EqualMatcher) MatcherName() string {
	return equal
}

func (m EqualMatcher) GetValue() constant.Value {
	return m.val.GetValue()
}

func (m EqualMatcher) GetType() gotypes.Type {
	return m.val.GetType()
}

func (m EqualMatcher) GetValueExpr() ast.Expr {
	return m.val.GetValueExpr()
}

func (m EqualMatcher) IsValueZero() bool {
	return m.val.IsValueZero()
}

func (m EqualMatcher) IsValueInt() bool {
	return m.val.IsValueInt()
}

func (m EqualMatcher) IsValueNumeric() bool {
	return m.val.IsValueNumeric()
}

func (m EqualMatcher) IsError() bool {
	return m.val.IsError()
}

func (m EqualMatcher) IsFunc() bool {
	return m.val.IsFunc()
}

func (m EqualMatcher) IsInterface() bool {
	return m.val.IsInterface()
}

func (m EqualMatcher) IsPointer() bool {
	return m.val.IsPointer()
}

type EqualNilMatcher struct {
	gotype gotypes.Type
}

func (EqualNilMatcher) Type() Type {
	return EqualNilMatcherType | EqualMatcherType | EqualValueMatcherType
}

func (EqualNilMatcher) MatcherName() string {
	return equal
}

func (n EqualNilMatcher) GetType() gotypes.Type {
	return n.gotype
}

type EqualTrueMatcher struct{}

func (EqualTrueMatcher) Type() Type {
	return EqualMatcherType | EqualBoolValueMatcherType | BoolValueTrue
}

func (EqualTrueMatcher) MatcherName() string {
	return equal
}

type EqualFalseMatcher struct{}

func (EqualFalseMatcher) Type() Type {
	return EqualMatcherType | EqualBoolValueMatcherType | BoolValueFalse
}

func (EqualFalseMatcher) MatcherName() string {
	return equal
}
