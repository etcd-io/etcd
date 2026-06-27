package sa5010

import (
	"fmt"
	"go/types"
	"strings"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA5010",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Impossible type assertion`,

		Text: `Some type assertions can be statically proven to be
impossible. This is the case when the method sets of both
arguments of the type assertion conflict with each other, for
example by containing the same method with different
signatures.

The Go compiler already applies this check when asserting from an
interface value to a concrete type. If the concrete type misses
methods from the interface, or if function signatures don't match,
then the type assertion can never succeed.

This check applies the same logic when asserting from one interface to
another. If both interface types contain the same method but with
different signatures, then the type assertion can never succeed,
either.`,

		Since:    "2020.1",
		Severity: lint.SeverityWarning,
		// Technically this should be MergeIfAll, but the Go compiler
		// already flags some impossible type assertions, so
		// MergeIfAny is consistent with the compiler.
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	type entry struct {
		l, r *types.Func
	}

	msc := &pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg.Prog.MethodSets
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, b := range fn.Blocks {
		instrLoop:
			for _, instr := range b.Instrs {
				assert, ok := instr.(*ir.TypeAssert)
				if !ok {
					continue
				}
				var wrong []entry
				left := assert.X.Type()
				right := assert.AssertedType
				righti, ok := right.Underlying().(*types.Interface)

				if !ok {
					// We only care about interface->interface
					// assertions. The Go compiler already catches
					// impossible interface->concrete assertions.
					continue
				}

				ms := msc.MethodSet(left)
				for mr := range righti.Methods() {
					sel := ms.Lookup(mr.Pkg(), mr.Name())
					if sel == nil {
						continue
					}
					ml := sel.Obj().(*types.Func)
					if ml.Origin() != ml || mr.Origin() != mr {
						// Give up when we see generics.
						//
						// TODO(dh): support generics once go/types gets an
						// exported API for type unification.
						continue instrLoop
					}
					if types.AssignableTo(ml.Type(), mr.Type()) {
						continue
					}

					wrong = append(wrong, entry{ml, mr})
				}

				if len(wrong) != 0 {
					var s strings.Builder
					s.WriteString(fmt.Sprintf("impossible type assertion; %s and %s contradict each other:",
						types.TypeString(left, types.RelativeTo(pass.Pkg)),
						types.TypeString(right, types.RelativeTo(pass.Pkg))))
					for _, e := range wrong {
						s.WriteString(fmt.Sprintf("\n\twrong type for %s method", e.l.Name()))
						s.WriteString(fmt.Sprintf("\n\t\thave %s", e.l.Type()))
						s.WriteString(fmt.Sprintf("\n\t\twant %s", e.r.Type()))
					}
					report.Report(pass, assert, s.String())
				}
			}
		}
	}
	return nil, nil
}
