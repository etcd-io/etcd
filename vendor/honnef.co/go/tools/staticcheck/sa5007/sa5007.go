package sa5007

import (
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA5007",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Infinite recursive call`,
		Text: `A function that calls itself recursively needs to have an exit
condition. Otherwise it will recurse forever, until the system runs
out of memory.

This issue can be caused by simple bugs such as forgetting to add an
exit condition. It can also happen "on purpose". Some languages have
tail call optimization which makes certain infinite recursive calls
safe to use. Go, however, does not implement TCO, and as such a loop
should be used instead.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		eachCall(fn, func(caller *ir.Function, site ir.CallInstruction, callee *ir.Function) {
			if callee != fn {
				return
			}
			if _, ok := site.(*ir.Go); ok {
				// Recursively spawning goroutines doesn't consume
				// stack space infinitely, so don't flag it.
				return
			}

			block := site.Block()
			for _, b := range fn.Blocks {
				if block.Dominates(b) {
					continue
				}
				if len(b.Instrs) == 0 {
					continue
				}
				if _, ok := b.Control().(*ir.Return); ok {
					return
				}
			}
			report.Report(pass, site, "infinite recursive call")
		})
	}
	return nil, nil
}

func eachCall(fn *ir.Function, cb func(caller *ir.Function, site ir.CallInstruction, callee *ir.Function)) {
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if site, ok := instr.(ir.CallInstruction); ok {
				if g := site.Common().StaticCallee(); g != nil {
					cb(fn, site, g)
				}
			}
		}
	}
}
