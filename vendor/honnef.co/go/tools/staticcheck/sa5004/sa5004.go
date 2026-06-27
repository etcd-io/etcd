package sa5004

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA5004",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `\"for { select { ...\" with an empty default branch spins`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query = pattern.MustParse(`(ForStmt nil nil nil (SelectStmt body))`)

func run(pass *analysis.Pass) (any, error) {
	for _, m := range code.Matches(pass, query) {
		for _, c := range m.State["body"].([]ast.Stmt) {
			// FIXME this leaves behind an empty line, and possibly
			// comments in the default branch. We can't easily fix
			// either.
			if comm, ok := c.(*ast.CommClause); ok && comm.Comm == nil && len(comm.Body) == 0 {
				report.Report(pass, comm,
					"should not have an empty default case in a for+select loop; the loop will spin",
					report.Fixes(edit.Fix("Remove empty default branch", edit.Delete(comm))))
				// there can only be one default case
				break
			}
		}
	}
	return nil, nil
}
