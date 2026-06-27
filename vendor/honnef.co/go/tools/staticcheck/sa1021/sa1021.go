package sa1021

import (
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1021",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title: `Using \'bytes.Equal\' to compare two \'net.IP\'`,
		Text: `A \'net.IP\' stores an IPv4 or IPv6 address as a slice of bytes. The
length of the slice for an IPv4 address, however, can be either 4 or
16 bytes long, using different ways of representing IPv4 addresses. In
order to correctly compare two \'net.IP\'s, the \'net.IP.Equal\' method should
be used, as it takes both representations into account.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"bytes.Equal": func(call *callcheck.Call) {
		if isConvertedFrom(call.Args[knowledge.Arg("bytes.Equal.a")].Value, "net.IP") &&
			isConvertedFrom(call.Args[knowledge.Arg("bytes.Equal.b")].Value, "net.IP") {
			call.Invalid("use net.IP.Equal to compare net.IPs, not bytes.Equal")
		}
	},
}

// ConvertedFrom reports whether value v was converted from type typ.
func isConvertedFrom(v callcheck.Value, typ string) bool {
	change, ok := v.Value.(*ir.ChangeType)
	return ok && types.TypeString(types.Unalias(change.X.Type()), nil) == typ
}
