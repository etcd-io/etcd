package s1039

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

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
		Name:     "S1039",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Unnecessary use of \'fmt.Sprint\'`,
		Text: `
Calling \'fmt.Sprint\' with a single string argument is unnecessary
and identical to using the string directly.`,
		Since: "2020.1",
		// MergeIfAll because s might not be a string under all build tags.
		// you shouldn't write code like that…
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkSprintLiteralQ = pattern.MustParse(`
	(CallExpr
		fn@(Or
			(Symbol "fmt.Sprint")
			(Symbol "fmt.Sprintf"))
		[lit@(BasicLit "STRING" _)])`)

func run(pass *analysis.Pass) (any, error) {
	// We only flag calls with string literals, not expressions of
	// type string, because some people use fmt.Sprint(s) as a pattern
	// for copying strings, which may be useful when extracting a small
	// substring from a large string.

	for node, m := range code.Matches(pass, checkSprintLiteralQ) {
		callee := m.State["fn"].(*types.Func)
		lit := m.State["lit"].(*ast.BasicLit)
		if callee.Name() == "Sprintf" {
			if strings.ContainsRune(lit.Value, '%') {
				// This might be a format string
				continue
			}
		}
		report.Report(pass, node, fmt.Sprintf("unnecessary use of fmt.%s", callee.Name()),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Replace with string literal", edit.ReplaceWithNode(pass.Fset, node, lit))))
	}
	return nil, nil
}
