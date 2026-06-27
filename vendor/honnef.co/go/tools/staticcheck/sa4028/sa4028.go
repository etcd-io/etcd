package sa4028

import (
	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4028",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `\'x % 1\' is always zero`,
		Since:    "2022.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny, // MergeIfAny if we only flag literals, not named constants
	},
})

var Analyzer = SCAnalyzer.Analyzer

var moduloOneQ = pattern.MustParse(`(BinaryExpr _ "%" (IntegerLiteral "1"))`)

func run(pass *analysis.Pass) (any, error) {
	for node := range code.Matches(pass, moduloOneQ) {
		report.Report(pass, node, "x % 1 is always zero")
	}
	return nil, nil
}
