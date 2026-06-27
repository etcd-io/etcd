package expression

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"

	"github.com/nunnatsa/ginkgolinter/internal/formatter"

	"github.com/go-toolsmith/astcopy"
	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
	"github.com/nunnatsa/ginkgolinter/internal/gomegahandler"
	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"
	"github.com/nunnatsa/ginkgolinter/internal/reverseassertion"
)

type GomegaExpression struct {
	orig  *ast.CallExpr
	clone *ast.CallExpr

	assertionFuncName     string
	origAssertionFuncName string
	actualFuncName        string

	isAsync bool

	actual  *actual.Actual
	matcher *matcher.Matcher

	handler *gomegahandler.Handler
}

func New(origExpr *ast.CallExpr, pass *analysis.Pass, handler *gomegahandler.Handler, timePkg string) (gexp *GomegaExpression) {
	info := handler.GetGomegaBasicInfo(origExpr)
	if info == nil {
		return nil
	}

	switch info.RootCallType {
	case gomegahandler.AsyncAssertionCall, gomegahandler.SyncAssertionCall:
		// Okay, that's what we want here.
	default:
		// Cannot handle anything else.
		return nil
	}

	origSel, ok := origExpr.Fun.(*ast.SelectorExpr)
	if !ok || !gomegainfo.IsAssertionFunc(origSel.Sel.Name) {
		return &GomegaExpression{
			orig:           origExpr,
			actualFuncName: info.MethodName,
		}
	}

	exprClone := astcopy.CallExpr(origExpr)
	selClone := exprClone.Fun.(*ast.SelectorExpr)

	origActual := info.RootCall
	if origActual == nil {
		return nil
	}

	actualClone := handler.GetActualExprClone(origActual, origSel, selClone)
	if actualClone == nil {
		return nil
	}

	actl, ok := actual.New(origExpr, exprClone, actualClone, pass, timePkg, info)
	if !ok {
		return nil
	}

	origMatcher, ok := origExpr.Args[0].(*ast.CallExpr)
	if !ok {
		return nil
	}

	matcherClone := exprClone.Args[0].(*ast.CallExpr)

	mtchr := matcher.New(origMatcher, matcherClone, pass, handler)
	if mtchr == nil {
		return nil
	}

	exprClone.Args[0] = mtchr.Clone

	gexp = &GomegaExpression{
		orig:  origExpr,
		clone: exprClone,

		assertionFuncName:     origSel.Sel.Name,
		origAssertionFuncName: origSel.Sel.Name,
		actualFuncName:        info.MethodName,

		isAsync: actl.IsAsync(),

		actual:  actl,
		matcher: mtchr,

		handler: handler,
	}

	if mtchr.ShouldReverseLogic() {
		gexp.ReverseAssertionFuncLogic()
	}

	return gexp
}

func (e *GomegaExpression) IsMissingAssertion() bool {
	return e.matcher == nil
}

func (e *GomegaExpression) GetActualFuncName() string {
	if e == nil {
		return ""
	}
	return e.actualFuncName
}

func (e *GomegaExpression) GetAssertFuncName() string {
	if e == nil {
		return ""
	}
	return e.assertionFuncName
}

func (e *GomegaExpression) GetOrigAssertFuncName() string {
	if e == nil {
		return ""
	}
	return e.origAssertionFuncName
}

func (e *GomegaExpression) IsAsync() bool {
	return e.isAsync
}

func (e *GomegaExpression) ReverseAssertionFuncLogic() {
	assertionFunc := e.clone.Fun.(*ast.SelectorExpr).Sel
	newName := reverseassertion.ChangeAssertionLogic(assertionFunc.Name)
	assertionFunc.Name = newName
	e.assertionFuncName = newName
}

func (e *GomegaExpression) ReplaceAssertionMethod(name string) {
	e.clone.Fun.(*ast.SelectorExpr).Sel.Name = name
}

func (e *GomegaExpression) ReplaceMatcherFuncName(name string) {
	e.matcher.ReplaceMatcherFuncName(name)
}

func (e *GomegaExpression) ReplaceMatcherArgs(newArgs []ast.Expr) {
	e.matcher.ReplaceMatcherArgs(newArgs)
}

func (e *GomegaExpression) RemoveMatcherArgs() {
	e.matcher.ReplaceMatcherArgs(nil)
}

func (e *GomegaExpression) ReplaceActual(newArg ast.Expr) {
	e.actual.ReplaceActual(newArg)
}

func (e *GomegaExpression) ReplaceActualWithItsFirstArg() {
	e.actual.ReplaceActualWithItsFirstArg()
}

func (e *GomegaExpression) replaceMathcerFuncNoArgs(name string) {
	e.matcher.ReplaceMatcherFuncName(name)
	e.RemoveMatcherArgs()
}

func (e *GomegaExpression) SetMatcherBeZero() {
	e.replaceMathcerFuncNoArgs("BeZero")
}

func (e *GomegaExpression) SetMatcherBeEmpty() {
	e.replaceMathcerFuncNoArgs("BeEmpty")
}

func (e *GomegaExpression) SetLenNumericMatcher() {
	if m, ok := e.matcher.GetMatcherInfo().(value.Valuer); ok && m.IsValueZero() {
		e.SetMatcherBeEmpty()
	} else {
		e.ReplaceMatcherFuncName("HaveLen")
		e.ReplaceMatcherArgs([]ast.Expr{m.GetValueExpr()})
	}
}

func (e *GomegaExpression) SetLenNumericActual() {
	if m, ok := e.matcher.GetMatcherInfo().(value.Valuer); ok && m.IsValueZero() {
		e.SetMatcherBeEmpty()
	} else {
		e.ReplaceMatcherFuncName("HaveLen")
		e.ReplaceMatcherArgs([]ast.Expr{m.GetValueExpr()})
	}
}

func (e *GomegaExpression) SetMatcherLen(arg ast.Expr) {
	e.ReplaceMatcherFuncName("HaveLen")
	e.ReplaceMatcherArgs([]ast.Expr{arg})
}

func (e *GomegaExpression) SetMatcherCap(arg ast.Expr) {
	e.ReplaceMatcherFuncName("HaveCap")
	e.ReplaceMatcherArgs([]ast.Expr{arg})
}

func (e *GomegaExpression) SetMatcherCapZero() {
	e.ReplaceMatcherFuncName("HaveCap")
	e.ReplaceMatcherArgs([]ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "0"}})
}

func (e *GomegaExpression) SetMatcherSucceed() {
	e.replaceMathcerFuncNoArgs("Succeed")
}

func (e *GomegaExpression) SetMatcherHaveOccurred() {
	e.replaceMathcerFuncNoArgs("HaveOccurred")
}

func (e *GomegaExpression) SetMatcherBeNil() {
	e.replaceMathcerFuncNoArgs("BeNil")
}

func (e *GomegaExpression) SetMatcherBeTrue() {
	e.replaceMathcerFuncNoArgs("BeTrue")
}

func (e *GomegaExpression) SetMatcherBeFalse() {
	e.replaceMathcerFuncNoArgs("BeFalse")
}

func (e *GomegaExpression) SetMatcherHaveValue() {
	newMatcherExp := e.handler.GetNewWrapperMatcher("HaveValue", e.matcher.Clone)
	e.clone.Args[0] = newMatcherExp
	e.matcher.Clone = newMatcherExp
}

func (e *GomegaExpression) SetMatcherEqual(arg ast.Expr) {
	e.ReplaceMatcherFuncName("Equal")
	e.ReplaceMatcherArgs([]ast.Expr{arg})
}

func (e *GomegaExpression) SetMatcherBeIdenticalTo(arg ast.Expr) {
	e.ReplaceMatcherFuncName("BeIdenticalTo")
	e.ReplaceMatcherArgs([]ast.Expr{arg})
}

func (e *GomegaExpression) SetMatcherBeNumerically(op token.Token, arg ast.Expr) {
	e.ReplaceMatcherFuncName("BeNumerically")
	e.ReplaceMatcherArgs([]ast.Expr{
		&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", op.String())},
		arg,
	})
}

func (e *GomegaExpression) IsNegativeAssertion() bool {
	return reverseassertion.IsNegativeLogic(e.assertionFuncName)
}

func (e *GomegaExpression) GetClone() *ast.CallExpr {
	return e.clone
}

// Actual proxies:

func (e *GomegaExpression) GetActualClone() *ast.CallExpr {
	return e.actual.Clone
}

func (e *GomegaExpression) AppendWithArgsToActual() {
	e.actual.AppendWithArgsMethod()
}

func (e *GomegaExpression) GetAsyncActualArg() *actual.AsyncArg {
	return e.actual.GetAsyncArg()
}

func (e *GomegaExpression) GetActualArg() actual.ArgPayload {
	return e.actual.Arg
}

func (e *GomegaExpression) GetActualArgExpr() ast.Expr {
	return e.actual.GetActualArg()
}

func (e *GomegaExpression) GetActualArgGOType() gotypes.Type {
	return e.actual.ArgGOType()
}

func (e *GomegaExpression) ActualArgTypeIs(other actual.ArgType) bool {
	return e.actual.Arg.ArgType().Is(other)
}

func (e *GomegaExpression) IsActualTuple() bool {
	return e.actual.IsTuple()
}

// Matcher proxies

func (e *GomegaExpression) GetMatcher() *matcher.Matcher {
	return e.matcher
}

func (e *GomegaExpression) GetMatcherInfo() matcher.Info {
	return e.matcher.GetMatcherInfo()
}

func (e *GomegaExpression) MatcherTypeIs(other matcher.Type) bool {
	return e.matcher.GetMatcherInfo().Type().Is(other)
}

func (e *GomegaExpression) FormatOrig(frm *formatter.GoFmtFormatter) string {
	return frm.Format(e.orig)
}

func (e *GomegaExpression) HasDescription() bool {
	if e == nil || e.orig == nil {
		return false
	}
	return len(e.orig.Args) > 1
}
