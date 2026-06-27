package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

// EqualNilRule checks for correct usage of nil comparisons.
// It suggests using the BeNil() matcher instead of Equal(nil) for better readability.
//
// Example:
//
//	// Bad:
//	Expect(x).To(Equal(nil))
//
//	// Good:
//	Expect(x).To(BeNil())
type EqualNilRule struct{}

func (r EqualNilRule) isApplied(gexp *expression.GomegaExpression, config config.Config) bool {
	return !config.SuppressNil &&
		gexp.MatcherTypeIs(matcher.EqualValueMatcherType)
}

func (r EqualNilRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	gexp.SetMatcherBeNil()

	reportBuilder.AddIssue(true, wrongNilWarningTemplate)

	return true
}
