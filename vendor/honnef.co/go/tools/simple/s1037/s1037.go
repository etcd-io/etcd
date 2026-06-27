package s1037

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
		Name:     "S1037",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Elaborate way of sleeping`,
		Text: `Using a select statement with a single case receiving
from the result of \'time.After\' is a very elaborate way of sleeping that
can much simpler be expressed with a simple call to time.Sleep.`,
		Since:   "2020.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkElaborateSleepQ = pattern.MustParse(`(SelectStmt (CommClause (UnaryExpr "<-" (CallExpr (Symbol "time.After") [arg])) body))`)
	checkElaborateSleepR = pattern.MustParse(`(CallExpr (SelectorExpr (Ident "time") (Ident "Sleep")) [arg])`)
)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkElaborateSleepQ) {
		if body, ok := m.State["body"].([]ast.Stmt); ok && len(body) == 0 {
			report.Report(pass, node, "should use time.Sleep instead of elaborate way of sleeping",
				report.ShortRange(),
				report.FilterGenerated(),
				report.Fixes(edit.Fix("Use time.Sleep", edit.ReplaceWithPattern(pass.Fset, node, checkElaborateSleepR, m.State))))
		} else {
			// TODO(dh): we could make a suggested fix if the body
			// doesn't declare or shadow any identifiers
			report.Report(pass, node, "should use time.Sleep instead of elaborate way of sleeping",
				report.ShortRange(),
				report.FilterGenerated())
		}
	}
	return nil, nil
}
