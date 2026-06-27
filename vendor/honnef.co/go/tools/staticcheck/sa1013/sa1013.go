package sa1013

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/knowledge"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1013",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `\'io.Seeker.Seek\' is being called with the whence constant as the first argument, but it should be the second`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkSeekerQ = pattern.MustParse(`(CallExpr fun@(SelectorExpr _ (Ident "Seek")) [arg1@(SelectorExpr _ (Symbol (Or "io.SeekStart" "io.SeekCurrent" "io.SeekEnd"))) arg2])`)
	checkSeekerR = pattern.MustParse(`(CallExpr fun [arg2 arg1])`)
)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkSeekerQ) {
		if !code.IsMethod(pass, m.State["fun"].(*ast.SelectorExpr), "Seek", knowledge.Signatures["(io.Seeker).Seek"]) {
			continue
		}
		edits := code.EditMatch(pass, node, m, checkSeekerR)
		report.Report(pass, node, "the first argument of io.Seeker is the offset, but an io.Seek* constant is being used instead",
			report.Fixes(edit.Fix("Swap arguments", edits...)))
	}
	return nil, nil
}
