package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/actual"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const comparePointerToValue = "comparing a pointer to a value will always fail"

// ComparePointerRule checks for comparisons between a pointer and a value using matchers like Equal, BeEquivalentTo, or BeIdenticalTo.
// Such comparisons will always fail, so this rule suggests using the HaveValue matcher instead.
// It applies when the actual argument is a pointer and the matcher is not comparing to another pointer, interface, or nil.
//
// Example:
//
//	a := 5
//	x := &a
//
//	// Bad:
//	Expect(x).To(Equal(5))
//
//	// Good:
//	Expect(x).To(HaveValue(5))
type ComparePointerRule struct{}

func (r ComparePointerRule) isApplied(gexp *expression.GomegaExpression) bool {
	actl, ok := gexp.GetActualArg().(*actual.RegularArgPayload)
	if !ok {
		return false
	}

	return actl.IsPointer()
}

func (r ComparePointerRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp) {
		return false
	}

	switch mtchr := gexp.GetMatcherInfo().(type) {
	case *matcher.EqualMatcher:
		if mtchr.IsPointer() || mtchr.IsInterface() {
			return false
		}

	case *matcher.BeEquivalentToMatcher:
		if mtchr.IsPointer() || mtchr.IsInterface() || mtchr.IsNil() {
			return false
		}

	case *matcher.BeIdenticalToMatcher:
		if mtchr.IsPointer() || mtchr.IsInterface() || mtchr.IsNil() {
			return false
		}

	case *matcher.EqualNilMatcher:
		return false

	case *matcher.BeTrueMatcher,
		*matcher.BeFalseMatcher,
		*matcher.BeNumericallyMatcher,
		*matcher.EqualTrueMatcher,
		*matcher.EqualFalseMatcher:

	default:
		return false
	}

	getMatcherOnlyRules().Apply(gexp, config, reportBuilder)

	gexp.SetMatcherHaveValue()
	reportBuilder.AddIssue(true, comparePointerToValue)

	return true
}
