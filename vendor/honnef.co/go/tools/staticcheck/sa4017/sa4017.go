package sa4017

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/purity"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4017",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer, purity.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Discarding the return values of a function without side effects, making the call pointless`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	pure := pass.ResultOf[purity.Analyzer].(purity.Result)

fnLoop:
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		if code.IsInTest(pass, fn) {
			params := fn.Signature.Params()
			for param := range params.Variables() {
				if typeutil.IsPointerToTypeWithName(param.Type(), "testing.B") {
					// Ignore discarded pure functions in code related
					// to benchmarks. Instead of matching BenchmarkFoo
					// functions, we match any function accepting a
					// *testing.B. Benchmarks sometimes call generic
					// functions for doing the actual work, and
					// checking for the parameter is a lot easier and
					// faster than analyzing call trees.
					continue fnLoop
				}
			}
		}

		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				ins, ok := ins.(*ir.Call)
				if !ok {
					continue
				}
				refs := ins.Referrers()
				if refs == nil || len(irutil.FilterDebug(*refs)) > 0 {
					continue
				}

				callee := ins.Common().StaticCallee()
				if callee == nil {
					continue
				}
				if callee.Object() == nil {
					// TODO(dh): support anonymous functions
					continue
				}
				if _, ok := pure[callee.Object().(*types.Func)]; ok {
					if pass.Pkg.Path() == "fmt_test" && callee.Object().(*types.Func).FullName() == "fmt.Sprintf" {
						// special case for benchmarks in the fmt package
						continue
					}
					report.Report(pass, ins, fmt.Sprintf("%s doesn't have side effects and its return value is ignored", callee.Object().Name()))
				}
			}
		}
	}
	return nil, nil
}
