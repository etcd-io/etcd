package sa1032

import (
	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1032",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Wrong order of arguments to \'errors.Is\'`,
		Text: `
The first argument of the function \'errors.Is\' is the error
that we have and the second argument is the error we're trying to match against.
For example:

	if errors.Is(err, io.EOF) { ... }

This check detects some cases where the two arguments have been swapped. It
flags any calls where the first argument is referring to a package-level error
variable, such as

	if errors.Is(io.EOF, err) { /* this is wrong */ }`,
		Since:    "2024.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"errors.Is": validateIs,
}

func validateIs(call *callcheck.Call) {
	if len(call.Args) != 2 {
		return
	}

	global := func(arg *callcheck.Argument) *ir.Global {
		v, ok := arg.Value.Value.(*ir.Load)
		if !ok {
			return nil
		}
		g, _ := v.X.(*ir.Global)
		return g
	}

	x, y := call.Args[0], call.Args[1]
	gx := global(x)
	if gx == nil {
		return
	}

	if pkgx := gx.Package().Pkg; pkgx != nil && pkgx.Path() != call.Pass.Pkg.Path() {
		// x is a global that's not in this package

		if gy := global(y); gy != nil {
			if pkgy := gy.Package().Pkg; pkgy != nil && pkgy.Path() != call.Pass.Pkg.Path() {
				// Both arguments refer to globals that aren't in this package. This can
				// genuinely happen for external tests that check that one error "is"
				// another one. net/http's external tests, for example, do
				// `errors.Is(http.ErrNotSupported, errors.ErrUnsupported)`.
				return
			}
		}

		call.Invalid("arguments have the wrong order")
	}
}
