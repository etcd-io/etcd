package sa4025

import (
	"fmt"
	"go/ast"
	"go/constant"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4025",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: "Integer division of literals that results in zero",
		Text: `When dividing two integer constants, the result will
also be an integer. Thus, a division such as \'2 / 3\' results in \'0\'.
This is true for all of the following examples:

	_ = 2 / 3
	const _ = 2 / 3
	const _ float64 = 2 / 3
	_ = float64(2 / 3)

Staticcheck will flag such divisions if both sides of the division are
integer literals, as it is highly unlikely that the division was
intended to truncate to zero. Staticcheck will not flag integer
division involving named constants, to avoid noisy positives.
`,
		Since:    "2021.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var integerDivisionQ = pattern.MustParse(`(BinaryExpr (IntegerLiteral _) "/" (IntegerLiteral _))`)

func run(pass *analysis.Pass) (any, error) {
	for node := range code.Matches(pass, integerDivisionQ) {
		val := constant.ToInt(pass.TypesInfo.Types[node.(ast.Expr)].Value)
		if v, ok := constant.Uint64Val(val); ok && v == 0 {
			report.Report(pass, node, fmt.Sprintf("the integer division '%s' results in zero", report.Render(pass, node)))
		}

		// TODO: we could offer a suggested fix here, but I am not
		// sure what it should be. There are many options to choose
		// from.

		// Note: we experimented with flagging divisions that truncate
		// (e.g. 4 / 3), but it ran into false positives in Go's
		// 'time' package, which does this, deliberately:
		//
		//   unixToInternal int64 = (1969*365 + 1969/4 - 1969/100 + 1969/400) * secondsPerDay
		//
		// The check also found a real bug in other code, but I don't
		// think we can outright ban this kind of division.
	}
	return nil, nil
}
