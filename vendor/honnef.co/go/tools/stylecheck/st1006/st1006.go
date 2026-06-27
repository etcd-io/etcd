package st1006

import (
	"go/types"

	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1006",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Poorly chosen receiver name`,
		Text: `Quoting Go Code Review Comments:

> The name of a method's receiver should be a reflection of its
> identity; often a one or two letter abbreviation of its type
> suffices (such as "c" or "cl" for "Client"). Don't use generic
> names such as "me", "this" or "self", identifiers typical of
> object-oriented languages that place more emphasis on methods as
> opposed to functions. The name need not be as descriptive as that
> of a method argument, as its role is obvious and serves no
> documentary purpose. It can be very short as it will appear on
> almost every line of every method of the type; familiarity admits
> brevity. Be consistent, too: if you call the receiver "c" in one
> method, don't call it "cl" in another.`,
		Since:   "2019.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg
	for _, m := range irpkg.Members {
		if T, ok := m.Object().(*types.TypeName); ok && !T.IsAlias() {
			ms := typeutil.IntuitiveMethodSet(T.Type(), nil)
			for _, sel := range ms {
				fn := sel.Obj().(*types.Func)
				recv := fn.Type().(*types.Signature).Recv()
				if typeutil.Dereference(recv.Type()) != T.Type() {
					// skip embedded methods
					continue
				}
				if recv.Name() == "self" || recv.Name() == "this" {
					report.Report(pass, recv, `receiver name should be a reflection of its identity; don't use generic names such as "this" or "self"`, report.FilterGenerated())
				}
				if recv.Name() == "_" {
					report.Report(pass, recv, "receiver name should not be an underscore, omit the name if it is unused", report.FilterGenerated())
				}
			}
		}
	}
	return nil, nil
}
