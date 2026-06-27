package sa1015

import (
	"go/token"
	"go/version"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1015",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Using \'time.Tick\' in a way that will leak. Consider using \'time.NewTicker\', and only use \'time.Tick\' in tests, commands and endless functions`,

		Text: `Before Go 1.23, \'time.Ticker\'s had to be closed to be able to be garbage
collected. Since \'time.Tick\' doesn't make it possible to close the underlying
ticker, using it repeatedly would leak memory.

Go 1.23 fixes this by allowing tickers to be collected even if they weren't closed.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		if fn.Pos() == token.NoPos || version.Compare(code.StdlibVersion(pass, fn), "go1.23") >= 0 {
			// Beginning with Go 1.23, the GC is able to collect unreferenced, unclosed
			// tickers, which makes time.Tick safe(r) to use.
			//
			// When we don't have a valid position, we err on the side of false negatives.
			// This shouldn't actually lead to any false negatives, as no functions
			// without valid positions (such as the synthesized init function) should be
			// able to use time.Tick.
			continue
		}

		if code.IsMainLike(pass) || code.IsInTest(pass, fn) {
			continue
		}
		for _, block := range fn.Blocks {
			for _, ins := range block.Instrs {
				call, ok := ins.(*ir.Call)
				if !ok || !irutil.IsCallTo(call.Common(), "time.Tick") {
					continue
				}
				if !irutil.Terminates(call.Parent()) {
					continue
				}
				report.Report(pass, call, "using time.Tick leaks the underlying ticker, consider using it only in endless functions, tests and the main package, and use time.NewTicker here")
			}
		}
	}
	return nil, nil
}
