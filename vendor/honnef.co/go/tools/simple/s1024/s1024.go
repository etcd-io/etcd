package s1024

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1024",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Replace \'x.Sub(time.Now())\' with \'time.Until(x)\'`,
		Text: `The \'time.Until\' helper has the same effect as using \'x.Sub(time.Now())\'
but is easier to read.`,
		Before:  `x.Sub(time.Now())`,
		After:   `time.Until(x)`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkTimeUntilQ = pattern.MustParse(`(CallExpr (Symbol "(time.Time).Sub") [(CallExpr (Symbol "time.Now") [])])`)
	checkTimeUntilR = pattern.MustParse(`(CallExpr (SelectorExpr (Ident "time") (Ident "Until")) [arg])`)
)

func run(pass *analysis.Pass) (any, error) {
	for node := range code.Matches(pass, checkTimeUntilQ) {
		if sel, ok := node.(*ast.CallExpr).Fun.(*ast.SelectorExpr); ok {
			r := pattern.NodeToAST(checkTimeUntilR.Root, map[string]any{"arg": sel.X}).(ast.Node)
			report.Report(pass, node, "should use time.Until instead of t.Sub(time.Now())",
				report.FilterGenerated(),
				report.MinimumStdlibVersion("go1.8"),
				report.Fixes(edit.Fix("Replace with call to time.Until", edit.ReplaceWithNode(pass.Fset, node, r))))
		} else {
			report.Report(pass, node, "should use time.Until instead of t.Sub(time.Now())",
				report.MinimumStdlibVersion("go1.8"),
				report.FilterGenerated())
		}
	}
	return nil, nil
}
