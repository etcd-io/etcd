package sa3001

import (
	"fmt"
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA3001",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `Assigning to \'b.N\' in benchmarks distorts the results`,
		Text: `The testing package dynamically sets \'b.N\' to improve the reliability of
benchmarks and uses it in computations to determine the duration of a
single operation. Benchmark code must not alter \'b.N\' as this would
falsify results.`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query = pattern.MustParse(`(AssignStmt sel@(SelectorExpr selX (Ident "N")) "=" [_] )`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, query) {
		assign := node.(*ast.AssignStmt)
		if !code.IsOfPointerToTypeWithName(pass, m.State["selX"].(ast.Expr), "testing.B") {
			continue
		}
		report.Report(pass, assign,
			fmt.Sprintf("should not assign to %s", report.Render(pass, m.State["sel"])))
	}
	return nil, nil
}
