package sa4012

import (
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4012",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Comparing a value against NaN even though no value is equal to NaN`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	isNaN := func(v ir.Value) bool {
		call, ok := v.(*ir.Call)
		if !ok {
			return false
		}
		return irutil.IsCallTo(call.Common(), "math.NaN")
	}
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, block := range fn.Blocks {
			for _, ins := range block.Instrs {
				ins, ok := ins.(*ir.BinOp)
				if !ok {
					continue
				}
				if isNaN(irutil.Flatten(ins.X)) || isNaN(irutil.Flatten(ins.Y)) {
					report.Report(pass, ins, "no value is equal to NaN, not even NaN itself")
				}
			}
		}
	}
	return nil, nil
}
