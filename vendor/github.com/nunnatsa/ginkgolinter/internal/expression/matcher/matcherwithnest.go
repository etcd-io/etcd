package matcher

import (
	"go/ast"
	gotypes "go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/gomegahandler"
)

type HaveValueMatcher struct {
	nested *Matcher
}

func (m *HaveValueMatcher) Type() Type {
	return HaveValueMatherType
}
func (m *HaveValueMatcher) MatcherName() string {
	return haveValue
}

func (m *HaveValueMatcher) GetNested() *Matcher {
	return m.nested
}

type WithTransformMatcher struct {
	funcType gotypes.Type
	nested   *Matcher
}

func (m *WithTransformMatcher) Type() Type {
	return WithTransformMatherType
}
func (m *WithTransformMatcher) MatcherName() string {
	return withTransform
}

func (m *WithTransformMatcher) GetNested() *Matcher {
	return m.nested
}

func (m *WithTransformMatcher) GetFuncType() gotypes.Type {
	return m.funcType
}

func getNestedMatcher(orig, clone *ast.CallExpr, offset int, pass *analysis.Pass, handler *gomegahandler.Handler) *Matcher {
	if origNested, ok := orig.Args[offset].(*ast.CallExpr); ok {
		cloneNested := clone.Args[offset].(*ast.CallExpr)

		return New(origNested, cloneNested, pass, handler)
	}

	return nil
}

func newWithTransformMatcher(fun ast.Expr, nested *Matcher, pass *analysis.Pass) *WithTransformMatcher {
	funcType := pass.TypesInfo.TypeOf(fun)
	if sig, ok := funcType.(*gotypes.Signature); ok && sig.Results().Len() > 0 {
		funcType = sig.Results().At(0).Type()
	}
	return &WithTransformMatcher{
		funcType: funcType,
		nested:   nested,
	}
}
