package s1007

import (
	"fmt"
	"go/ast"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1007",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Simplify regular expression by using raw string literal`,
		Text: `Raw string literals use backticks instead of quotation marks and do not support
any escape sequences. This means that the backslash can be used
freely, without the need of escaping.

Since regular expressions have their own escape sequences, raw strings
can improve their readability.`,
		Before:  `regexp.Compile("\\A(\\w+) profile: total \\d+\\n\\z")`,
		After:   "regexp.Compile(`\\A(\\w+) profile: total \\d+\\n\\z`)",
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

// TODO(dominikh): support string concat, maybe support constants
var query = pattern.MustParse(`(CallExpr (Symbol fn@(Or "regexp.MustCompile" "regexp.Compile")) [lit@(BasicLit "STRING" _)])`)

func run(pass *analysis.Pass) (any, error) {
outer:
	for _, m := range code.Matches(pass, query) {
		lit := m.State["lit"].(*ast.BasicLit)
		val := lit.Value
		if lit.Value[0] != '"' {
			// already a raw string
			continue
		}
		if !strings.Contains(val, `\\`) {
			continue
		}
		if strings.Contains(val, "`") {
			continue
		}

		bs := false
		for _, c := range val {
			if !bs && c == '\\' {
				bs = true
				continue
			}
			if bs && c == '\\' {
				bs = false
				continue
			}
			if bs {
				// backslash followed by non-backslash -> escape sequence
				continue outer
			}
		}

		report.Report(pass, lit, fmt.Sprintf("should use raw string (`...`) with %s to avoid having to escape twice", m.State["fn"]), report.FilterGenerated())
	}
	return nil, nil
}
