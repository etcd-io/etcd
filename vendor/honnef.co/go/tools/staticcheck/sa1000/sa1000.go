package sa1000

import (
	"go/constant"
	"regexp"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1000",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title:    `Invalid regular expression`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"regexp.MustCompile": check,
	"regexp.Compile":     check,
	"regexp.Match":       check,
	"regexp.MatchReader": check,
	"regexp.MatchString": check,
}

func check(call *callcheck.Call) {
	arg := call.Args[0]
	if c := callcheck.ExtractConstExpectKind(arg.Value, constant.String); c != nil {
		s := constant.StringVal(c.Value)
		if _, err := regexp.Compile(s); err != nil {
			arg.Invalid(err.Error())
		}
	}
}
