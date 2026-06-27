package sa1010

import (
	"fmt"
	"go/constant"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1010",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(checkRegexpFindAllRules),
	},
	Doc: &lint.RawDocumentation{
		Title: `\'(*regexp.Regexp).FindAll\' called with \'n == 0\', which will always return zero results`,
		Text: `If \'n >= 0\', the function returns at most \'n\' matches/submatches. To
return all results, specify a negative number.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny, // MergeIfAny if we only flag literals, not named constants
	},
})

var Analyzer = SCAnalyzer.Analyzer

var checkRegexpFindAllRules = map[string]callcheck.Check{
	"(*regexp.Regexp).FindAll":                    RepeatZeroTimes("a FindAll method", 1),
	"(*regexp.Regexp).FindAllIndex":               RepeatZeroTimes("a FindAll method", 1),
	"(*regexp.Regexp).FindAllString":              RepeatZeroTimes("a FindAll method", 1),
	"(*regexp.Regexp).FindAllStringIndex":         RepeatZeroTimes("a FindAll method", 1),
	"(*regexp.Regexp).FindAllStringSubmatch":      RepeatZeroTimes("a FindAll method", 1),
	"(*regexp.Regexp).FindAllStringSubmatchIndex": RepeatZeroTimes("a FindAll method", 1),
	"(*regexp.Regexp).FindAllSubmatch":            RepeatZeroTimes("a FindAll method", 1),
	"(*regexp.Regexp).FindAllSubmatchIndex":       RepeatZeroTimes("a FindAll method", 1),
}

func RepeatZeroTimes(name string, arg int) callcheck.Check {
	return func(call *callcheck.Call) {
		arg := call.Args[arg]
		if k, ok := arg.Value.Value.(*ir.Const); ok && k.Value.Kind() == constant.Int {
			if v, ok := constant.Int64Val(k.Value); ok && v == 0 {
				arg.Invalid(fmt.Sprintf("calling %s with n == 0 will return no results, did you mean -1?", name))
			}
		}
	}
}
