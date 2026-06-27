package s1031

import (
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1031",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title: `Omit redundant nil check around loop`,
		Text: `You can use range on nil slices and maps, the loop will simply never
execute. This makes an additional nil check around the loop
unnecessary.`,
		Before: `
if s != nil {
    for _, x := range s {
        ...
    }
}`,
		After: `
for _, x := range s {
    ...
}`,
		Since: "2017.1",
		// MergeIfAll because x might be a channel under some build tags.
		// you shouldn't write code like that…
		MergeIf: lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkNilCheckAroundRangeQ = pattern.MustParse(`
	(IfStmt
		nil
		(BinaryExpr x@(Object _) "!=" (Builtin "nil"))
		[(RangeStmt _ _ _ x _)]
		nil)`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, checkNilCheckAroundRangeQ) {
		ok := typeutil.All(m.State["x"].(types.Object).Type(), func(term *types.Term) bool {
			switch term.Type().Underlying().(type) {
			case *types.Slice, *types.Map:
				return true
			case *types.TypeParam, *types.Chan, *types.Pointer, *types.Signature:
				return false
			default:
				lint.ExhaustiveTypeSwitch(term.Type().Underlying())
				return false
			}
		})
		if ok {
			report.Report(pass, node, "unnecessary nil check around range", report.ShortRange(), report.FilterGenerated())
		}
	}
	return nil, nil
}
