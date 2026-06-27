package sa1029

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1029",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(checkWithValueKeyRules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Inappropriate key in call to \'context.WithValue\'`,
		Text: `The provided key must be comparable and should not be
of type \'string\' or any other built-in type to avoid collisions between
packages using context. Users of \'WithValue\' should define their own
types for keys.

To avoid allocating when assigning to an \'interface{}\',
context keys often have concrete type \'struct{}\'. Alternatively,
exported context key variables' static type should be a pointer or
interface.`,
		Since:    "2020.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkWithValueKeyRules = map[string]callcheck.Check{
	"context.WithValue": checkWithValueKey,
}

func checkWithValueKey(call *callcheck.Call) {
	arg := call.Args[1]
	T := arg.Value.Value.Type()
	if typ, ok := types.Unalias(T).(*types.Basic); ok {
		if _, ok := T.(*types.Alias); ok {
			arg.Invalid(
				fmt.Sprintf("should not use built-in type %s (via alias %s) as key for value; define your own type to avoid collisions", typ, types.TypeString(T, types.RelativeTo(call.Pass.Pkg))))
		} else {
			arg.Invalid(
				fmt.Sprintf("should not use built-in type %s as key for value; define your own type to avoid collisions", typ))
		}
	}
	// TODO(dh): we should probably flag all anonymous structs, as they all risk collisions
	if s, ok := T.(*types.Struct); ok && s.NumFields() == 0 {
		arg.Invalid("should not use empty anonymous struct as key for value; define your own type to avoid collisions")
	} else if !types.Comparable(T) {
		arg.Invalid(fmt.Sprintf("keys used with context.WithValue must be comparable, but type %s is not comparable", T))
	}
}
