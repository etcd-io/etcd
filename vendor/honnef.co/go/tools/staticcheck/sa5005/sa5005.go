package sa5005

import (
	"fmt"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA5005",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `The finalizer references the finalized object, preventing garbage collection`,
		Text: `A finalizer is a function associated with an object that runs when the
garbage collector is ready to collect said object, that is when the
object is no longer referenced by anything.

If the finalizer references the object, however, it will always remain
as the final reference to that object, preventing the garbage
collector from collecting the object. The finalizer will never run,
and the object will never be collected, leading to a memory leak. That
is why the finalizer should instead use its first argument to operate
on the object. That way, the number of references can temporarily go
to zero before the object is being passed to the finalizer.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	cb := func(caller *ir.Function, site ir.CallInstruction, callee *ir.Function) {
		if callee.RelString(nil) != "runtime.SetFinalizer" {
			return
		}
		arg0 := site.Common().Args[knowledge.Arg("runtime.SetFinalizer.obj")]
		if iface, ok := arg0.(*ir.MakeInterface); ok {
			arg0 = iface.X
		}
		load, ok := arg0.(*ir.Load)
		if !ok {
			return
		}
		v, ok := load.X.(*ir.Alloc)
		if !ok {
			return
		}
		arg1 := site.Common().Args[knowledge.Arg("runtime.SetFinalizer.finalizer")]
		if iface, ok := arg1.(*ir.MakeInterface); ok {
			arg1 = iface.X
		}
		mc, ok := arg1.(*ir.MakeClosure)
		if !ok {
			return
		}
		for _, b := range mc.Bindings {
			if b == v {
				pos := report.DisplayPosition(pass.Fset, mc.Fn.Pos())
				report.Report(pass, site, fmt.Sprintf("the finalizer closes over the object, preventing the finalizer from ever running (at %s)", pos))
			}
		}
	}
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		eachCall(fn, cb)
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
