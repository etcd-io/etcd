package sa1024

import (
	"go/constant"
	"sort"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1024",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title: `A string cutset contains duplicate characters`,
		Text: `The \'strings.TrimLeft\' and \'strings.TrimRight\' functions take cutsets, not
prefixes. A cutset is treated as a set of characters to remove from a
string. For example,

    strings.TrimLeft("42133word", "1234")

will result in the string \'"word"\' – any characters that are 1, 2, 3 or
4 are cut from the left of the string.

In order to remove one string from another, use \'strings.TrimPrefix\' instead.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var rules = map[string]callcheck.Check{
	"strings.Trim":      check,
	"strings.TrimLeft":  check,
	"strings.TrimRight": check,
}

func check(call *callcheck.Call) {
	arg := call.Args[1]
	if !isUniqueStringCutset(arg.Value) {
		const MsgNonUniqueCutset = "cutset contains duplicate characters"
		arg.Invalid(MsgNonUniqueCutset)
	}
}

func isUniqueStringCutset(v callcheck.Value) bool {
	if c := callcheck.ExtractConstExpectKind(v, constant.String); c != nil {
		s := constant.StringVal(c.Value)
		rs := runeSlice(s)
		if len(rs) < 2 {
			return true
		}
		sort.Sort(rs)
		for i, r := range rs[1:] {
			if rs[i] == r {
				return false
			}
		}
	}
	return true
}

type runeSlice []rune

func (rs runeSlice) Len() int               { return len(rs) }
func (rs runeSlice) Less(i int, j int) bool { return rs[i] < rs[j] }
func (rs runeSlice) Swap(i int, j int)      { rs[i], rs[j] = rs[j], rs[i] }
