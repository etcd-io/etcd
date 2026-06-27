package st1008

import (
	"go/types"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1008",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:   `A function's error value should be its last return value`,
		Text:    `A function's error value should be its last return value.`,
		Since:   `2019.1`,
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
fnLoop:
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		sig := fn.Type().(*types.Signature)
		rets := sig.Results()
		if rets == nil || rets.Len() < 2 {
			continue
		}

		if types.Unalias(rets.At(rets.Len()-1).Type()) == types.Universe.Lookup("error").Type() {
			// Last return type is error. If the function also returns
			// errors in other positions, that's fine.
			continue
		}

		if rets.Len() >= 2 &&
			types.Unalias(rets.At(rets.Len()-1).Type()) == types.Universe.Lookup("bool").Type() &&
			types.Unalias(rets.At(rets.Len()-2).Type()) == types.Universe.Lookup("error").Type() {
			// Accept (..., error, bool) and assume it's a comma-ok function. It's not clear whether the bool should come last or not for these kinds of functions.
			continue
		}
		for i := rets.Len() - 2; i >= 0; i-- {
			if types.Unalias(rets.At(i).Type()) == types.Universe.Lookup("error").Type() {
				report.Report(pass, rets.At(i), "error should be returned as the last argument", report.ShortRange())
				continue fnLoop
			}
		}
	}
	return nil, nil
}
