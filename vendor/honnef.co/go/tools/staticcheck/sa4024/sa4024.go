package sa4024

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
		Name:     "SA4024",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `Checking for impossible return value from a builtin function`,
		Text: `Return values of the \'len\' and \'cap\' builtins cannot be negative.

See https://golang.org/pkg/builtin/#len and https://golang.org/pkg/builtin/#cap.

Example:

    if len(slice) < 0 {
        fmt.Println("unreachable code")
    }`,
		Since:    "2021.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var builtinLessThanZeroQ = pattern.MustParse(`
	(Or
		(BinaryExpr
			(IntegerLiteral "0")
			">"
			(CallExpr builtin@(Builtin (Or "len" "cap")) _))
		(BinaryExpr
			(CallExpr builtin@(Builtin (Or "len" "cap")) _)
			"<"
			(IntegerLiteral "0")))
`)

func run(pass *analysis.Pass) (any, error) {
	for node, matcher := range code.Matches(pass, builtinLessThanZeroQ) {
		builtin := matcher.State["builtin"].(*ast.Ident)
		report.Report(pass, node, fmt.Sprintf("builtin function %s does not return negative values", builtin.Name))
	}
	return nil, nil
}
