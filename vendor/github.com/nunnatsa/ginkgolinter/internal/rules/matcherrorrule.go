package rules

import (
	"go/ast"

	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const (
	matchErrorArgWrongType       = "the MatchError matcher used to assert a non error type (%s)"
	matchErrorWrongTypeAssertion = "MatchError first parameter (%s) must be error, string, GomegaMatcher or func(error)bool are allowed"
	matchErrorMissingDescription = "missing function description as second parameter of MatchError"
	matchErrorRedundantArg       = "redundant MatchError arguments; consider removing them"
	matchErrorNoFuncDescription  = "The second parameter of MatchError must be the function description (string)"
)

// MatchErrorRule validates the usage of the MatchError matcher.
//
// First, it checks that the actual value is actually an error
// Then, it checks the matcher itself: this matcher can be used in 3 different ways:
//  1. With error type variable
//  2. With a gomega matcher, to check the actual err.Error() value
//  3. With function with a signature of func(error) bool. In this case, additional description
//     string variable is required.
type MatchErrorRule struct{}

func (r MatchErrorRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.MatcherTypeIs(matcher.MatchErrorMatcherType | matcher.MultipleMatcherMatherType)
}

func (r MatchErrorRule) Apply(gexp *expression.GomegaExpression, _ config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	return checkMatchError(gexp, reportBuilder)
}

func checkMatchError(gexp *expression.GomegaExpression, reportBuilder *reports.Builder) bool {
	mtchr := gexp.GetMatcherInfo()
	switch m := mtchr.(type) {
	case matcher.MatchErrorMatcher:
		return checkMatchErrorMatcher(gexp, gexp.GetMatcher(), m, reportBuilder)

	case *matcher.MultipleMatchersMatcher:
		res := false
		for i := range m.Len() {
			nested := m.At(i)
			if specific, ok := nested.GetMatcherInfo().(matcher.MatchErrorMatcher); ok {
				if valid := checkMatchErrorMatcher(gexp, gexp.GetMatcher(), specific, reportBuilder); valid {
					res = true
				}
			}
		}
		return res
	default:
		return false
	}
}

func checkMatchErrorMatcher(gexp *expression.GomegaExpression, mtchr *matcher.Matcher, mtchrInfo matcher.MatchErrorMatcher, reportBuilder *reports.Builder) bool {
	if !gexp.ActualArgTypeIs(actual.ErrorTypeArgType) {
		reportBuilder.AddIssue(false, matchErrorArgWrongType, reportBuilder.FormatExpr(gexp.GetActualArgExpr()))
	}

	switch m := mtchrInfo.(type) {
	case *matcher.InvalidMatchErrorMatcher:
		reportBuilder.AddIssue(false, matchErrorWrongTypeAssertion, reportBuilder.FormatExpr(mtchr.Clone.Args[0]))

	case *matcher.MatchErrorMatcherWithErrFunc:
		if m.NumArgs() == m.AllowedNumArgs() {
			if !m.IsSecondArgString() {
				reportBuilder.AddIssue(false, matchErrorNoFuncDescription)
			}
			return true
		}

		if m.NumArgs() == 1 {
			reportBuilder.AddIssue(false, matchErrorMissingDescription)
			return true
		}

	case *matcher.MatchErrorMatcherWithErr,
		*matcher.MatchErrorMatcherWithMatcher,
		*matcher.MatchErrorMatcherWithString:
		// continue
	default:
		return false
	}

	if mtchrInfo.NumArgs() == mtchrInfo.AllowedNumArgs() {
		return true
	}

	if mtchrInfo.NumArgs() > mtchrInfo.AllowedNumArgs() {
		var newArgsSuggestion []ast.Expr
		for i := 0; i < mtchrInfo.AllowedNumArgs(); i++ {
			newArgsSuggestion = append(newArgsSuggestion, mtchr.Clone.Args[i])
		}
		mtchr.Clone.Args = newArgsSuggestion
		reportBuilder.AddIssue(false, matchErrorRedundantArg)
		return true
	}
	return false
}
