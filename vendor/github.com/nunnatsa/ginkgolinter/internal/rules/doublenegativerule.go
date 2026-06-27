package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const doubleNegativeWarningTemplate = "avoid double negative assertion"

// DoubleNegativeRule checks for double negative assertions, such as using `Not(BeFalse())`.
// It suggests replacing `Not(BeFalse())` with `BeTrue()` for better readability.
//
// Example:
//
//	// Bad:
//	Expect(x).NotTo(BeFalse())
//
//	// Good:
//	Expect(x).To(BeTrue())
type DoubleNegativeRule struct{}

func (DoubleNegativeRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.MatcherTypeIs(matcher.BeFalseMatcherType) &&
		gexp.IsNegativeAssertion()
}

func (r DoubleNegativeRule) Apply(gexp *expression.GomegaExpression, _ config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	gexp.ReverseAssertionFuncLogic()
	gexp.SetMatcherBeTrue()

	reportBuilder.AddIssue(true, doubleNegativeWarningTemplate)

	return true
}
