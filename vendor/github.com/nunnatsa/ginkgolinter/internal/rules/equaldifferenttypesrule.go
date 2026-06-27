package rules

import (
	gotypes "go/types"

	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/expression/matcher"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

const compareDifferentTypes = "use %[1]s with different types: Comparing %[2]s with %[3]s; either change the expected value type if possible, or use the BeEquivalentTo() matcher, instead of %[1]s()"

// EqualDifferentTypesRule checks for correct usage of matchers with different types.
// It suggests using the BeEquivalentTo() matcher instead of Equal() when comparing different types.
//
// Example:
//
//	x := float64(5)
//
//	// Bad: (compares int with float64)
//	Expect(x).To(Equal(5))
//
//	// Good:
//	Expect(x).To(BeEquivalentTo(5))
type EqualDifferentTypesRule struct{}

func (r EqualDifferentTypesRule) isApplied(config config.Config) bool {
	return !config.SuppressTypeCompare
}

func (r EqualDifferentTypesRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(config) {
		return false
	}

	return r.checkEqualDifferentTypes(gexp, gexp.GetMatcher(), false, reportBuilder)
}

func (r EqualDifferentTypesRule) checkEqualDifferentTypes(gexp *expression.GomegaExpression, mtchr *matcher.Matcher, parentPointer bool, reportBuilder *reports.Builder) bool {
	actualType := gexp.GetActualArgGOType()

	if parentPointer {
		if t, ok := actualType.(*gotypes.Pointer); ok {
			actualType = t.Elem()
		}
	}

	var (
		matcherType gotypes.Type
		matcherName string
	)

	switch specificMatcher := mtchr.GetMatcherInfo().(type) {
	case *matcher.EqualMatcher:
		matcherType = specificMatcher.GetType()
		matcherName = specificMatcher.MatcherName()

	case *matcher.BeIdenticalToMatcher:
		matcherType = specificMatcher.GetType()
		matcherName = specificMatcher.MatcherName()

	case *matcher.HaveValueMatcher:
		return r.checkEqualDifferentTypes(gexp, specificMatcher.GetNested(), true, reportBuilder)

	case *matcher.MultipleMatchersMatcher:
		foundIssue := false
		for i := range specificMatcher.Len() {
			if r.checkEqualDifferentTypes(gexp, specificMatcher.At(i), parentPointer, reportBuilder) {
				foundIssue = true
			}
		}

		return foundIssue

	case *matcher.EqualNilMatcher:
		matcherType = specificMatcher.GetType()
		matcherName = specificMatcher.MatcherName()

	case *matcher.WithTransformMatcher:
		nested := specificMatcher.GetNested()
		switch specificNested := nested.GetMatcherInfo().(type) {
		case *matcher.EqualMatcher:
			matcherType = specificNested.GetType()
			matcherName = specificNested.MatcherName()

		case *matcher.BeIdenticalToMatcher:
			matcherType = specificNested.GetType()
			matcherName = specificNested.MatcherName()

		default:
			return false
		}

		actualType = specificMatcher.GetFuncType()
	default:
		return false
	}

	if !gotypes.Identical(matcherType, actualType) {
		if r.isImplementing(matcherType, actualType) || r.isImplementing(actualType, matcherType) {
			return false
		}

		reportBuilder.AddIssue(false, compareDifferentTypes, matcherName, actualType, matcherType)
		return true
	}

	return false
}

func (r EqualDifferentTypesRule) isImplementing(ifs, impl gotypes.Type) bool {
	if gotypes.IsInterface(ifs) {
		var (
			theIfs *gotypes.Interface
			ok     bool
		)

		for {
			theIfs, ok = ifs.(*gotypes.Interface)
			if ok {
				break
			}
			ifs = ifs.Underlying()
		}

		return gotypes.Implements(impl, theIfs)
	}

	return false
}
