package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

// Rule is the interface for all the linter rules
type Rule interface {
	// Apply applies the rule to the given gomega expression
	Apply(*expression.GomegaExpression, config.Config, *reports.Builder) bool
}

// rules is the list of rules that are applied to a sync assertion expression.
var rules = Rules{
	&ForceExpectToRule{},
	&LenRule{},
	&CapRule{},
	&ComparisonRule{},
	&NilCompareRule{},
	&ComparePointerRule{},
	&ErrorEqualNilRule{},
	&MatchErrorRule{},
	getMatcherOnlyRules(),
	&EqualDifferentTypesRule{},
	&HaveOccurredRule{},
	&SucceedRule{},
	&AssertionDescriptionRule{},
}

// asyncRules is the list of rules that are applied to an async assertion expression.
var asyncRules = Rules{
	&AsyncFuncCallRule{},
	&AsyncTimeIntervalsRule{},
	&ErrorEqualNilRule{},
	&MatchErrorRule{},
	&AsyncSucceedRule{},
	&AssertionDescriptionRule{},
	getMatcherOnlyRules(),
}

func GetRules() Rules {
	return rules
}

func GetAsyncRules() Rules {
	return asyncRules
}

type Rules []Rule

func (r Rules) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	for _, rule := range r {
		if rule.Apply(gexp, config, reportBuilder) {
			return true
		}
	}

	return false
}

var missingAssertionRule = MissingAssertionRule{}

func GetMissingAssertionRule() Rule {
	return missingAssertionRule
}
