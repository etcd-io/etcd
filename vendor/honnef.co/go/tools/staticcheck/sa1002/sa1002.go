package sa1002

import (
	"go/constant"
	"strings"
	"time"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1002",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title:    `Invalid format in \'time.Parse\'`,
		Since:    "2017.1",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"time.Parse": func(call *callcheck.Call) {
		arg := call.Args[knowledge.Arg("time.Parse.layout")]
		if c := callcheck.ExtractConstExpectKind(arg.Value, constant.String); c != nil {
			s := constant.StringVal(c.Value)
			s = strings.Replace(s, "_", " ", -1)
			s = strings.Replace(s, "Z", "-", -1)
			_, err := time.Parse(s, s)
			if err != nil {
				arg.Invalid(err.Error())
			}
		}
	},
}
