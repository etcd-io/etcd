package sa1028

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1028",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title:    `\'sort.Slice\' can only be used on slices`,
		Text:     `The first argument of \'sort.Slice\' must be a slice.`,
		Since:    "2020.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"sort.Slice":         check,
	"sort.SliceIsSorted": check,
	"sort.SliceStable":   check,
}

func check(call *callcheck.Call) {
	c := call.Instr.Common().StaticCallee()
	arg := call.Args[0]

	T := arg.Value.Value.Type().Underlying()
	switch T.(type) {
	case *types.Interface:
		// we don't know.
		// TODO(dh): if the value is a phi node we can look at its edges
		if k, ok := arg.Value.Value.(*ir.Const); ok && k.Value == nil {
			// literal nil, e.g. sort.Sort(nil, ...)
			arg.Invalid(fmt.Sprintf("cannot call %s on nil literal", c))
		}
	case *types.Slice:
		// this is fine
	default:
		// this is not fine
		arg.Invalid(fmt.Sprintf("%s must only be called on slices, was called on %s", c, T))
	}
}
