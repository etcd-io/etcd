package matcher

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/gomegahandler"
)

const ( // gomega matchers
	beEmpty        = "BeEmpty"
	beEquivalentTo = "BeEquivalentTo"
	beFalse        = "BeFalse"
	beIdenticalTo  = "BeIdenticalTo"
	beNil          = "BeNil"
	beNumerically  = "BeNumerically"
	beTrue         = "BeTrue"
	beZero         = "BeZero"
	equal          = "Equal"
	haveLen        = "HaveLen"
	haveValue      = "HaveValue"
	and            = "And"
	or             = "Or"
	withTransform  = "WithTransform"
	matchError     = "MatchError"
	haveOccurred   = "HaveOccurred"
	succeed        = "Succeed"
)

type Matcher struct {
	funcName      string
	Orig          *ast.CallExpr
	Clone         *ast.CallExpr
	info          Info
	reverseLogic  bool
	handler       *gomegahandler.Handler
	hasNotMatcher bool // true if the matcher is wrapped with a "Not" matcher
}

func New(origMatcher, matcherClone *ast.CallExpr, pass *analysis.Pass, handler *gomegahandler.Handler) *Matcher {
	reverse := false
	hasNotMatcher := false

	var assertFuncName string
	for {
		info := handler.GetGomegaBasicInfo(origMatcher)
		if info == nil {
			return nil
		}

		if info.MethodName != "Not" {
			assertFuncName = info.MethodName
			break
		}

		hasNotMatcher = true
		reverse = !reverse
		var ok bool
		origMatcher, ok = origMatcher.Args[0].(*ast.CallExpr)
		if !ok {
			return nil
		}
		matcherClone = matcherClone.Args[0].(*ast.CallExpr)
	}

	return &Matcher{
		funcName:      assertFuncName,
		Orig:          origMatcher,
		Clone:         matcherClone,
		info:          getMatcherInfo(origMatcher, matcherClone, assertFuncName, pass, handler),
		reverseLogic:  reverse,
		hasNotMatcher: hasNotMatcher,
		handler:       handler,
	}
}

func (m *Matcher) ShouldReverseLogic() bool {
	return m.reverseLogic
}

func (m *Matcher) HasNotMatcher() bool {
	return m.hasNotMatcher
}

func (m *Matcher) GetMatcherInfo() Info {
	return m.info
}

func (m *Matcher) ReplaceMatcherFuncName(name string) {
	m.handler.ReplaceFunction(m.Clone, ast.NewIdent(name))
}

func (m *Matcher) ReplaceMatcherArgs(newArgs []ast.Expr) {
	m.Clone.Args = newArgs
}
