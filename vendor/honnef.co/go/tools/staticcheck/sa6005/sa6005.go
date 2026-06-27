package sa6005

import (
	"go/ast"
	"go/token"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA6005",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `Inefficient string comparison with \'strings.ToLower\' or \'strings.ToUpper\'`,
		Text: `Converting two strings to the same case and comparing them like so

    if strings.ToLower(s1) == strings.ToLower(s2) {
        ...
    }

is significantly more expensive than comparing them with
\'strings.EqualFold(s1, s2)\'. This is due to memory usage as well as
computational complexity.

\'strings.ToLower\' will have to allocate memory for the new strings, as
well as convert both strings fully, even if they differ on the very
first byte. strings.EqualFold, on the other hand, compares the strings
one character at a time. It doesn't need to create two intermediate
strings and can return as soon as the first non-matching character has
been found.

For a more in-depth explanation of this issue, see
https://blog.digitalocean.com/how-to-efficiently-compare-strings-in-go/`,
		Since:    "2019.2",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkToLowerToUpperComparisonQ = pattern.MustParse(`
	(BinaryExpr
		(CallExpr fun@(Symbol (Or "strings.ToLower" "strings.ToUpper")) [a])
 		tok@(Or "==" "!=")
 		(CallExpr fun [b]))`)
	checkToLowerToUpperComparisonR = pattern.MustParse(`(CallExpr (SelectorExpr (Ident "strings") (Ident "EqualFold")) [a b])`)
)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkToLowerToUpperComparisonQ) {
		rn := pattern.NodeToAST(checkToLowerToUpperComparisonR.Root, m.State).(ast.Expr)
		if m.State["tok"].(token.Token) == token.NEQ {
			rn = &ast.UnaryExpr{
				Op: token.NOT,
				X:  rn,
			}
		}

		report.Report(pass, node,
			"should use strings.EqualFold instead",
			report.Fixes(edit.Fix("Replace with strings.EqualFold", edit.ReplaceWithNode(pass.Fset, node, rn))))
	}
	return nil, nil
}
