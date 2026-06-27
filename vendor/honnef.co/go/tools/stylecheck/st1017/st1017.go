package st1017

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
		Name:     "ST1017",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Don't use Yoda conditions`,
		Text: `Yoda conditions are conditions of the kind \"if 42 == x\", where the
literal is on the left side of the comparison. These are a common
idiom in languages in which assignment is an expression, to avoid bugs
of the kind \"if (x = 42)\". In Go, which doesn't allow for this kind of
bug, we prefer the more idiomatic \"if x == 42\".`,
		Since:   "2019.2",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkYodaConditionsQ = pattern.MustParse(`(BinaryExpr left@(TrulyConstantExpression _) tok@(Or "==" "!=") right@(Not (TrulyConstantExpression _)))`)
	checkYodaConditionsR = pattern.MustParse(`(BinaryExpr right tok left)`)
)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkYodaConditionsQ) {
		edits := code.EditMatch(pass, node, m, checkYodaConditionsR)
		report.Report(pass, node, "don't use Yoda conditions",
			report.FilterGenerated(),
			report.Fixes(edit.Fix("Un-Yoda-fy", edits...)))
	}
	return nil, nil
}
