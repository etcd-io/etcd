package s1004

import (
	"fmt"
	"go/ast"
	"go/token"

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
		Name:     "S1004",
		Run:      CheckBytesCompare,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title:   `Replace call to \'bytes.Compare\' with \'bytes.Equal\'`,
		Before:  `if bytes.Compare(x, y) == 0 {}`,
		After:   `if bytes.Equal(x, y) {}`,
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkBytesCompareQ  = pattern.MustParse(`(BinaryExpr (CallExpr (Symbol "bytes.Compare") args) op@(Or "==" "!=") (IntegerLiteral "0"))`)
	checkBytesCompareRe = pattern.MustParse(`(CallExpr (SelectorExpr (Ident "bytes") (Ident "Equal")) args)`)
	checkBytesCompareRn = pattern.MustParse(`(UnaryExpr "!" (CallExpr (SelectorExpr (Ident "bytes") (Ident "Equal")) args))`)
)

func CheckBytesCompare(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Path() == "bytes" || pass.Pkg.Path() == "bytes_test" {
		// the bytes package is free to use bytes.Compare as it sees fit
		return nil, nil
	}
	for node, m := range code.Matches(pass, checkBytesCompareQ) {
		args := report.RenderArgs(pass, m.State["args"].([]ast.Expr))
		prefix := ""
		if m.State["op"].(token.Token) == token.NEQ {
			prefix = "!"
		}

		var fix analysis.SuggestedFix
		switch tok := m.State["op"].(token.Token); tok {
		case token.EQL:
			fix = edit.Fix("Simplify use of bytes.Compare", edit.ReplaceWithPattern(pass.Fset, node, checkBytesCompareRe, m.State))
		case token.NEQ:
			fix = edit.Fix("Simplify use of bytes.Compare", edit.ReplaceWithPattern(pass.Fset, node, checkBytesCompareRn, m.State))
		default:
			panic(fmt.Sprintf("unexpected token %v", tok))
		}
		report.Report(pass, node, fmt.Sprintf("should use %sbytes.Equal(%s) instead", prefix, args), report.FilterGenerated(), report.Fixes(fix))
	}
	return nil, nil
}
