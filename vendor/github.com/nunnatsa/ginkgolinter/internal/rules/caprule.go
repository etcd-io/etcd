package rules

import (
	"go/token"

	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const wrongCapWarningTemplate = "wrong cap assertion"

// CapRule does not allow using the cap() function in actual with numeric comparison.
// it suggests to use the HaveLen matcher, instead.
//
// Example:
//
//	// Bad:
//	Expect(cap(x)).To(Equal(5))
//
//	// Good:
//	Expect(x).To(HaveCap(5))
type CapRule struct{}

func (r *CapRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	if r.fixExpression(gexp) {
		reportBuilder.AddIssue(true, wrongCapWarningTemplate)
		return true
	}
	return false
}

func (r *CapRule) isApplied(gexp *expression.GomegaExpression, config config.Config) bool {
	if config.SuppressLen {
		return false
	}

	//matcherType := gexp.matcher.GetMatcherInfo().Type()
	if gexp.ActualArgTypeIs(actual.CapFuncActualArgType) {
		if gexp.MatcherTypeIs(matcher.EqualMatcherType | matcher.BeZeroMatcherType) {
			return true
		}

		if gexp.MatcherTypeIs(matcher.BeNumericallyMatcherType) {
			mtchr := gexp.GetMatcherInfo().(*matcher.BeNumericallyMatcher)
			return mtchr.GetOp() == token.EQL || mtchr.GetOp() == token.NEQ || gexp.MatcherTypeIs(matcher.EqualZero|matcher.GreaterThanZero)
		}
	}

	if gexp.ActualArgTypeIs(actual.CapComparisonActualArgType) && gexp.MatcherTypeIs(matcher.BeTrueMatcherType|matcher.BeFalseMatcherType|matcher.EqualBoolValueMatcherType) {
		return true
	}

	return false
}

func (r *CapRule) fixExpression(gexp *expression.GomegaExpression) bool {
	if gexp.ActualArgTypeIs(actual.CapFuncActualArgType) {
		return r.fixEqual(gexp)
	}

	if gexp.ActualArgTypeIs(actual.CapComparisonActualArgType) {
		return r.fixComparison(gexp)
	}

	return false
}

func (r *CapRule) fixEqual(gexp *expression.GomegaExpression) bool {
	matcherInfo := gexp.GetMatcherInfo()
	switch mtchr := matcherInfo.(type) {
	case *matcher.EqualMatcher:
		gexp.SetMatcherCap(mtchr.GetValueExpr())

	case *matcher.BeZeroMatcher:
		gexp.SetMatcherCapZero()

	case *matcher.BeNumericallyMatcher:
		if !r.handleBeNumerically(gexp, mtchr) {
			return false
		}

	default:
		return false
	}

	gexp.ReplaceActualWithItsFirstArg()

	return true
}

func (r *CapRule) fixComparison(gexp *expression.GomegaExpression) bool {
	actl := gexp.GetActualArg().(*actual.FuncComparisonPayload)
	if op := actl.GetOp(); op == token.NEQ {
		gexp.ReverseAssertionFuncLogic()
	} else if op != token.EQL {
		return false
	}

	gexp.SetMatcherCap(actl.GetValueExpr())
	gexp.ReplaceActual(actl.GetFuncArg())

	if gexp.MatcherTypeIs(matcher.BoolValueFalse) {
		gexp.ReverseAssertionFuncLogic()
	}

	return true
}

func (r *CapRule) handleBeNumerically(gexp *expression.GomegaExpression, mtchr *matcher.BeNumericallyMatcher) bool {
	op := mtchr.GetOp()

	if op == token.EQL {
		gexp.SetMatcherCap(mtchr.GetValueExpr())
	} else if op == token.NEQ {
		gexp.ReverseAssertionFuncLogic()
		gexp.SetMatcherCap(mtchr.GetValueExpr())
	} else if gexp.MatcherTypeIs(matcher.GreaterThanZero) {
		gexp.ReverseAssertionFuncLogic()
		gexp.SetMatcherCapZero()
	} else {
		return false
	}

	return true
}
