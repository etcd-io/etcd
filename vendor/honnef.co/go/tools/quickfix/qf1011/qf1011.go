package qf1011

import (
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/sharedcheck"
)

func init() {
	SCAnalyzer.Analyzer.Name = "QF1011"
}

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: sharedcheck.RedundantTypeInDeclarationChecker("could", true),
	Doc: &lint.RawDocumentation{
		Title:    "Omit redundant type from variable declaration",
		Since:    "2021.1",
		Severity: lint.SeverityHint,
	},
})

var Analyzer = SCAnalyzer.Analyzer
