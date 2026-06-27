package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const missingAssertionMessage = `%q: missing assertion method. Expected %s`

// MissingAssertionRule checks if the assertion method is missing. In this case, the test does not make any assertion.
// This is mostly relevant for the async actual methods, that tend to be longer, and so harder to spot the missing assertion
// by just reading the test code.
//
// Examples:
//
//		// Bad:
//		Expect(x)
//		Eventually(func() error {
//			return nil
//	 	})
//
//		// Good:
//		Expect(x).To(Equal(42))
//		Eventually(func() error {
//			return nil
//		}).Should(Succeed())
type MissingAssertionRule struct{}

func (r MissingAssertionRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.IsMissingAssertion()
}

func (r MissingAssertionRule) Apply(gexp *expression.GomegaExpression, _ config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	actualMethodName := gexp.GetActualFuncName()
	reportBuilder.AddIssue(false, missingAssertionMessage, actualMethodName, gomegainfo.GetAllowedAssertionMethods(actualMethodName))

	return true
}
