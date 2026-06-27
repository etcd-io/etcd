package sa9005

import (
	"fmt"
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name: "SA9005",
		Requires: []*analysis.Analyzer{
			buildir.Analyzer,
			// Filtering generated code because it may include empty structs generated from data models.
			generated.Analyzer,
		},
		Run: callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Trying to marshal a struct with no public fields nor custom marshaling`,
		Text: `
The \'encoding/json\' and \'encoding/xml\' packages only operate on exported
fields in structs, not unexported ones. It is usually an error to try
to (un)marshal structs that only consist of unexported fields.

This check will not flag calls involving types that define custom
marshaling behavior, e.g. via \'MarshalJSON\' methods. It will also not
flag empty structs.`,
		Since:    "2019.2",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	// TODO(dh): should we really flag XML? Even an empty struct
	// produces a non-zero amount of data, namely its type name.
	// Let's see if we encounter any false positives.
	//
	// Also, should we flag gob?
	"encoding/json.Marshal":           check(knowledge.Arg("json.Marshal.v"), "MarshalJSON", "MarshalText"),
	"encoding/xml.Marshal":            check(knowledge.Arg("xml.Marshal.v"), "MarshalXML", "MarshalText"),
	"(*encoding/json.Encoder).Encode": check(knowledge.Arg("(*encoding/json.Encoder).Encode.v"), "MarshalJSON", "MarshalText"),
	"(*encoding/xml.Encoder).Encode":  check(knowledge.Arg("(*encoding/xml.Encoder).Encode.v"), "MarshalXML", "MarshalText"),

	"encoding/json.Unmarshal":         check(knowledge.Arg("json.Unmarshal.v"), "UnmarshalJSON", "UnmarshalText"),
	"encoding/xml.Unmarshal":          check(knowledge.Arg("xml.Unmarshal.v"), "UnmarshalXML", "UnmarshalText"),
	"(*encoding/json.Decoder).Decode": check(knowledge.Arg("(*encoding/json.Decoder).Decode.v"), "UnmarshalJSON", "UnmarshalText"),
	"(*encoding/xml.Decoder).Decode":  check(knowledge.Arg("(*encoding/xml.Decoder).Decode.v"), "UnmarshalXML", "UnmarshalText"),
}

func check(argN int, meths ...string) callcheck.Check {
	return func(call *callcheck.Call) {
		if code.IsGenerated(call.Pass, call.Instr.Pos()) {
			return
		}
		arg := call.Args[argN]
		T := arg.Value.Value.Type()
		Ts, ok := typeutil.Dereference(T).Underlying().(*types.Struct)
		if !ok {
			return
		}
		if Ts.NumFields() == 0 {
			return
		}
		fields := typeutil.FlattenFields(Ts)
		for _, field := range fields {
			if field.Var.Exported() {
				return
			}
		}
		// OPT(dh): we could use a method set cache here
		ms := call.Instr.Parent().Prog.MethodSets.MethodSet(T)
		// TODO(dh): we're not checking the signature, which can cause false negatives.
		// This isn't a huge problem, however, since vet complains about incorrect signatures.
		for _, meth := range meths {
			if ms.Lookup(nil, meth) != nil {
				return
			}
		}
		arg.Invalid(fmt.Sprintf("struct type '%s' doesn't have any exported fields, nor custom marshaling", typeutil.Dereference(T)))
	}
}
