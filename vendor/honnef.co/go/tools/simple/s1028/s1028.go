package s1028

import (
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
		Name:     "S1028",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title:   `Simplify error construction with \'fmt.Errorf\'`,
		Before:  `errors.New(fmt.Sprintf(...))`,
		After:   `fmt.Errorf(...)`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkErrorsNewSprintfQ = pattern.MustParse(`(CallExpr (Symbol "errors.New") [(CallExpr (Symbol "fmt.Sprintf") args)])`)
	checkErrorsNewSprintfR = pattern.MustParse(`(CallExpr (SelectorExpr (Ident "fmt") (Ident "Errorf")) args)`)
)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkErrorsNewSprintfQ) {
		edits := code.EditMatch(pass, node, m, checkErrorsNewSprintfR)
		// TODO(dh): the suggested fix may leave an unused import behind
		report.Report(pass, node, "should use fmt.Errorf(...) instead of errors.New(fmt.Sprintf(...))",
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Use fmt.Errorf", edits...)))
	}
	return nil, nil
}
