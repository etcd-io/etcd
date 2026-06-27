package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const assertionDescriptionWarning = "missing assertion description"

// AssertionDescriptionRule checks for missing assertion descriptions in Gomega assertions.
// It suggests adding a description to improve the clarity of the assertion.
//
// Example:
//
//	// Bad:
//	Expect(x).To(Equal(5))
//
//	// Good:
//	Expect(x).To(Equal(5), "x should be equal to 5")
type AssertionDescriptionRule struct{}

// Apply applies the assertion description rule to the given gomega expression
func (r *AssertionDescriptionRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	reportBuilder.AddIssue(false, assertionDescriptionWarning)
	return true
}

func (r *AssertionDescriptionRule) isApplied(gexp *expression.GomegaExpression, config config.Config) bool {
	if !config.ForceAssertionDescription {
		return false
	}

	return !gexp.HasDescription()
}
