package sa1001

import (
	"go/ast"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/knowledge"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1001",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title:    `Invalid template`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query = pattern.MustParse(`
	(CallExpr
		(Symbol
		name@(Or
			"(*text/template.Template).Parse"
			"(*html/template.Template).Parse"))
		[s])`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, query) {
		name := m.State["name"].(string)
		var kind string
		switch name {
		case "(*text/template.Template).Parse":
			kind = "text"
		case "(*html/template.Template).Parse":
			kind = "html"
		}

		call := node.(*ast.CallExpr)
		sel := call.Fun.(*ast.SelectorExpr)
		if !code.IsCallToAny(pass, sel.X, "text/template.New", "html/template.New") {
			// TODO(dh): this is a cheap workaround for templates with
			// different delims. A better solution with less false
			// negatives would use data flow analysis to see where the
			// template comes from and where it has been
			continue
		}

		s, ok := code.ExprToString(pass, m.State["s"].(ast.Expr))
		if !ok {
			continue
		}
		var err error
		switch kind {
		case "text":
			_, err = texttemplate.New("").Parse(s)
		case "html":
			_, err = htmltemplate.New("").Parse(s)
		}
		if err != nil {
			// TODO(dominikh): whitelist other parse errors, if any
			if strings.Contains(err.Error(), "unexpected") ||
				strings.Contains(err.Error(), "bad character") {
				report.Report(pass, call.Args[knowledge.Arg("(*text/template.Template).Parse.text")], err.Error())
			}
		}
	}
	return nil, nil
}
