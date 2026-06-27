package sa2002

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA2002",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Called \'testing.T.FailNow\' or \'SkipNow\' in a goroutine, which isn't allowed`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, block := range fn.Blocks {
			for _, ins := range block.Instrs {
				gostmt, ok := ins.(*ir.Go)
				if !ok {
					continue
				}
				var fn *ir.Function
				switch val := gostmt.Call.Value.(type) {
				case *ir.Function:
					fn = val
				case *ir.MakeClosure:
					fn = val.Fn.(*ir.Function)
				default:
					continue
				}
				if fn.Blocks == nil {
					continue
				}
				for _, block := range fn.Blocks {
					for _, ins := range block.Instrs {
						call, ok := ins.(*ir.Call)
						if !ok {
							continue
						}
						if call.Call.IsInvoke() {
							continue
						}
						callee := call.Call.StaticCallee()
						if callee == nil {
							continue
						}
						recv := callee.Signature.Recv()
						if recv == nil {
							continue
						}
						if !typeutil.IsPointerToTypeWithName(recv.Type(), "testing.common") {
							continue
						}
						fn, ok := call.Call.StaticCallee().Object().(*types.Func)
						if !ok {
							continue
						}
						name := fn.Name()
						switch name {
						case "FailNow", "Fatal", "Fatalf", "SkipNow", "Skip", "Skipf":
						default:
							continue
						}
						// TODO(dh): don't report multiple diagnostics
						// for multiple calls to T.Fatal, but do
						// collect all of them as related information
						report.Report(pass, gostmt, fmt.Sprintf("the goroutine calls T.%s, which must be called in the same goroutine as the test", name),
							report.Related(call, fmt.Sprintf("call to T.%s", name)))
					}
				}
			}
		}
	}
	return nil, nil
}
