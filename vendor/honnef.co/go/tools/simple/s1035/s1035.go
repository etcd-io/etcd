package s1035

import (
	"fmt"
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
		Name:     "S1035",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Redundant call to \'net/http.CanonicalHeaderKey\' in method call on \'net/http.Header\'`,
		Text: `
The methods on \'net/http.Header\', namely \'Add\', \'Del\', \'Get\'
and \'Set\', already canonicalize the given header name.`,
		Since:   "2020.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query = pattern.MustParse(`
	(CallExpr
		(Symbol
			callName@(Or
				"(net/http.Header).Add"
				"(net/http.Header).Del"
				"(net/http.Header).Get"
				"(net/http.Header).Set"))
		arg@(CallExpr (Symbol "net/http.CanonicalHeaderKey") _):_)`)

func run(pass *analysis.Pass) (any, error) {
	for _, m := range code.Matches(pass, query) {
		arg := m.State["arg"].(*ast.CallExpr)
		report.Report(pass, m.State["arg"].(ast.Expr),
			fmt.Sprintf("calling net/http.CanonicalHeaderKey on the 'key' argument of %s is redundant", m.State["callName"].(string)),
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Remove call to CanonicalHeaderKey", edit.ReplaceWithNode(pass.Fset, arg, arg.Args[0]))))
	}
	return nil, nil
}
