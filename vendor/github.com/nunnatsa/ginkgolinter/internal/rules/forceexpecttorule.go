package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const forceExpectToTemplate = "must not use %s with %s"

// ForceExpectToRule checks for correct usage of Expect and ExpectWithOffset assertions.
// It suggests using the To and ToNot assertion methods instead of the Should and ShouldNot.
//
// Example:
//
//	// Bad:
//	Expect(x).Should(Equal(5))
//
//	// Good:
//	Expect(x).To(Equal(5))
type ForceExpectToRule struct{}

func (ForceExpectToRule) isApplied(gexp *expression.GomegaExpression, config config.Config) bool {
	if !config.ForceExpectTo {
		return false
	}

	actlName := gexp.GetActualFuncName()
	return actlName == "Expect" || actlName == "ExpectWithOffset"
}

func (r ForceExpectToRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}

	var newName string

	switch gexp.GetAssertFuncName() {
	case "Should":
		newName = "To"
	case "ShouldNot":
		newName = "ToNot"
	default:
		return false
	}

	gexp.ReplaceAssertionMethod(newName)
	reportBuilder.AddIssue(true, forceExpectToTemplate, gexp.GetActualFuncName(), gexp.GetOrigAssertFuncName())

	// always return false, to keep checking another rules.
	return false
}
