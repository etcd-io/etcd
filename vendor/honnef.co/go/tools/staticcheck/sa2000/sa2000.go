package sa2000

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
		Name:     "SA2000",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `\'sync.WaitGroup.Add\' called inside the goroutine, leading to a race condition`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkWaitgroupAddQ = pattern.MustParse(`
	(GoStmt
		(CallExpr
			(FuncLit
				_
				call@(CallExpr (Symbol "(*sync.WaitGroup).Add") _):_) _))`)

func run(pass *analysis.Pass) (any, error) {
	for _, m := range code.Matches(pass, checkWaitgroupAddQ) {
		call := m.State["call"].(ast.Node)
		report.Report(pass, call, fmt.Sprintf("should call %s before starting the goroutine to avoid a race", report.Render(pass, call)))
	}
	return nil, nil
}
