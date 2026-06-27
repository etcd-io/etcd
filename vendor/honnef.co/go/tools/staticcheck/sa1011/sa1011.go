package sa1011

import (
	"go/constant"
	"unicode/utf8"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1011",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(checkUTF8CutsetRules),
	},
	Doc: &lint.RawDocumentation{
		Title:    `Various methods in the \"strings\" package expect valid UTF-8, but invalid input is provided`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkUTF8CutsetRules = map[string]callcheck.Check{
	"strings.IndexAny":     check,
	"strings.LastIndexAny": check,
	"strings.ContainsAny":  check,
	"strings.Trim":         check,
	"strings.TrimLeft":     check,
	"strings.TrimRight":    check,
}

func check(call *callcheck.Call) {
	arg := call.Args[1]
	if c := callcheck.ExtractConstExpectKind(arg.Value, constant.String); c != nil {
		s := constant.StringVal(c.Value)
		if !utf8.ValidString(s) {
			arg.Invalid("argument is not a valid UTF-8 encoded string")
		}
	}
}
