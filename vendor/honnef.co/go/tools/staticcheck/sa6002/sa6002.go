package sa6002

import (
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA6002",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Storing non-pointer values in \'sync.Pool\' allocates memory`,
		Text: `A \'sync.Pool\' is used to avoid unnecessary allocations and reduce the
amount of work the garbage collector has to do.

When passing a value that is not a pointer to a function that accepts
an interface, the value needs to be placed on the heap, which means an
additional allocation. Slices are a common thing to put in sync.Pools,
and they're structs with 3 fields (length, capacity, and a pointer to
an array). In order to avoid the extra allocation, one should store a
pointer to the slice instead.

See the comments on https://go-review.googlesource.com/c/go/+/24371
that discuss this problem.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"(*sync.Pool).Put": func(call *callcheck.Call) {
		arg := call.Args[knowledge.Arg("(*sync.Pool).Put.x")]
		typ := arg.Value.Value.Type()
		_, isSlice := typ.Underlying().(*types.Slice)
		if !typeutil.IsPointerLike(typ) || isSlice {
			arg.Invalid("argument should be pointer-like to avoid allocations")
		}
	},
}
