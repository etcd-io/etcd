package sa4022

import (
	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4022",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `Comparing the address of a variable against nil`,
		Text:     `Code such as \"if &x == nil\" is meaningless, because taking the address of a variable always yields a non-nil pointer.`,
		Since:    "2020.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var CheckAddressIsNilQ = pattern.MustParse(
	`(BinaryExpr
		(UnaryExpr "&" _)
		(Or "==" "!=")
		(Builtin "nil"))`)

func run(pass *analysis.Pass) (any, error) {
	for node := range code.Matches(pass, CheckAddressIsNilQ) {
		report.Report(pass, node, "the address of a variable cannot be nil")
	}
	return nil, nil
}
