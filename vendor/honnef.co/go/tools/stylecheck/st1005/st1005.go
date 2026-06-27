package st1005

import (
	"go/constant"
	"strings"
	"unicode"
	"unicode/utf8"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1005",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Incorrectly formatted error string`,
		Text: `Error strings follow a set of guidelines to ensure uniformity and good
composability.

Quoting Go Code Review Comments:

> Error strings should not be capitalized (unless beginning with
> proper nouns or acronyms) or end with punctuation, since they are
> usually printed following other context. That is, use
> \'fmt.Errorf("something bad")\' not \'fmt.Errorf("Something bad")\', so
> that \'log.Printf("Reading %s: %v", filename, err)\' formats without a
> spurious capital letter mid-message.`,
		Since:   "2019.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	objNames := map[*ir.Package]map[string]bool{}
	irpkg := pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg
	objNames[irpkg] = map[string]bool{}
	for _, m := range irpkg.Members {
		if typ, ok := m.(*ir.Type); ok {
			objNames[irpkg][typ.Name()] = true
		}
	}
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		objNames[fn.Package()][fn.Name()] = true
	}

	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		if code.IsInTest(pass, fn) {
			// We don't care about malformed error messages in tests;
			// they're usually for direct human consumption, not part
			// of an API
			continue
		}
		for _, block := range fn.Blocks {
		instrLoop:
			for _, ins := range block.Instrs {
				call, ok := ins.(*ir.Call)
				if !ok {
					continue
				}
				if !irutil.IsCallToAny(call.Common(), "errors.New", "fmt.Errorf") {
					continue
				}

				k, ok := call.Common().Args[0].(*ir.Const)
				if !ok {
					continue
				}

				s := constant.StringVal(k.Value)
				if len(s) == 0 {
					continue
				}
				switch s[len(s)-1] {
				case '.', ':', '!', '\n':
					report.Report(pass, call, "error strings should not end with punctuation or newlines")
				}
				before, _, ok0 := strings.Cut(s, " ")
				if !ok0 {
					// single word error message, probably not a real
					// error but something used in tests or during
					// debugging
					continue
				}
				word := before
				first, n := utf8.DecodeRuneInString(word)
				if !unicode.IsUpper(first) {
					continue
				}
				for _, c := range word[n:] {
					if unicode.IsUpper(c) || unicode.IsDigit(c) {
						// Word is probably an initialism or multi-word function name. Digits cover elliptic curves like
						// P384.
						continue instrLoop
					}
				}

				if strings.ContainsRune(word, '(') {
					// Might be a function call
					continue instrLoop
				}
				word = strings.TrimRightFunc(word, func(r rune) bool { return unicode.IsPunct(r) })
				if objNames[fn.Package()][word] {
					// Word is probably the name of a function or type in this package
					continue
				}
				// First word in error starts with a capital
				// letter, and the word doesn't contain any other
				// capitals, making it unlikely to be an
				// initialism or multi-word function name.
				//
				// It could still be a proper noun, though.

				report.Report(pass, call, "error strings should not be capitalized")
			}
		}
	}
	return nil, nil
}
