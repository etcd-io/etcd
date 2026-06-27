package sa4027

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4027",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `\'(*net/url.URL).Query\' returns a copy, modifying it doesn't change the URL`,
		Text: `\'(*net/url.URL).Query\' parses the current value of \'net/url.URL.RawQuery\'
and returns it as a map of type \'net/url.Values\'. Subsequent changes to
this map will not affect the URL unless the map gets encoded and
assigned to the URL's \'RawQuery\'.

As a consequence, the following code pattern is an expensive no-op:
\'u.Query().Add(key, value)\'.`,
		Since:    "2021.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var ineffectiveURLQueryAddQ = pattern.MustParse(`(CallExpr (SelectorExpr (CallExpr (SelectorExpr recv (Ident "Query")) []) (Ident meth)) _)`)

func run(pass *analysis.Pass) (any, error) {
	// TODO(dh): We could make this check more complex and detect
	// pointless modifications of net/url.Values in general, but that
	// requires us to get the state machine correct, else we'll cause
	// false positives.

	for node, m := range code.Matches(pass, ineffectiveURLQueryAddQ) {
		if !code.IsOfPointerToTypeWithName(pass, m.State["recv"].(ast.Expr), "net/url.URL") {
			continue
		}
		switch m.State["meth"].(string) {
		case "Add", "Del", "Set":
		default:
			continue
		}
		report.Report(pass, node, "(*net/url.URL).Query returns a copy, modifying it doesn't change the URL")
	}
	return nil, nil
}
