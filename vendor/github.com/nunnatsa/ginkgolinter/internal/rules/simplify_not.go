package rules

import (
	"github.com/nunnatsa/ginkgolinter/config"
	"github.com/nunnatsa/ginkgolinter/internal/expression"
	"github.com/nunnatsa/ginkgolinter/internal/reports"
)

type SimplifyNotRule struct{}

func (r *SimplifyNotRule) Apply(gexp *expression.GomegaExpression, config config.Config, reportBuilder *reports.Builder) bool {
	if !r.isApplied(gexp, config) {
		return false
	}
	reportBuilder.AddIssue(true, "simplify negation by removing the 'Not' matcher")
	return true
}

func (r *SimplifyNotRule) isApplied(gexp *expression.GomegaExpression, config config.Config) bool {
	return config.ForeToNot && gexp.GetMatcher().HasNotMatcher()
}
