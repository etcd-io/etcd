package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

// HaveOccurredRule checks for correct usage of the HaveOccurred matcher.
// It covers two main cases:
// 1. It suggests using the Succeed matcher for error-returning functions, instead of HaveOccurred.
// 2. It warns when the HaveOccurred matcher is used with a non-error type.
//
// Examples:
//
//	func errFn() error {
//		return errors.New("error")
//	}
//
//	// Bad:
//	Expect(errFn()).To(HaveOccurred())
//
//	// Good:
//	Expect(errFn()).ToNot(Succeed())
//
//	var x int = 5
//
//	// Bad:
//	Expect(x).To(HaveOccurred()) // x is not an error type
type HaveOccurredRule struct{}

func (r HaveOccurredRule) isApplied(gexp *expression.GomegaExpression) bool {
	return gexp.MatcherTypeIs(matcher.HaveOccurredMatcherType)
}

func (r HaveOccurredRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	if !gexp.ActualArgTypeIs(actual.ErrorTypeArgType) {
		reportBuilder.AddIssue(false, "asserting a non-error type with HaveOccurred matcher")
		return true
	}

	if config.ForceSucceedForFuncs && gexp.GetActualArg().(*actual.ErrPayload).IsFunc() {
		gexp.ReverseAssertionFuncLogic()
		gexp.SetMatcherSucceed()
		reportBuilder.AddIssue(true, "prefer using the Succeed matcher for error function, instead of HaveOccurred")
		return true
	}

	return false
}
