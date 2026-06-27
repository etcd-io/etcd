package model

import "golang.org/x/tools/go/analysis"

// AnalysisContext provides contextual information about the running analysis.
type AnalysisContext struct {
	// Config provides analyzer configuration.
	Config Config

	// InspectorResult is the analysis result of the pre-run inspector.
	InspectorResult *InspectorResult

	// Pass is the analysis Pass instance.
	Pass *analysis.Pass
}

// Checker defines a rule checker.
type Checker interface {
	// GetCoveredRules returns the set of rules applied by the checker.
	GetCoveredRules() RuleSet

	// Apply checks for the rule(s).
	Apply(actx *AnalysisContext) error
}
