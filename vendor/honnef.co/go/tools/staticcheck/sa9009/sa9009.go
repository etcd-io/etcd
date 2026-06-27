package sa9009

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/analysis"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA9009",
		Run:      run,
		Requires: []*analysis.Analyzer{},
	},
	Doc: &lint.RawDocumentation{
		Title: "Ineffectual Go compiler directive",
		Text: `
A potential Go compiler directive was found, but is ineffectual as it begins
with whitespace.`,
		Since:    "2024.1",
		Severity: lint.SeverityWarning,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, f := range pass.Files {
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				// Compiler directives have to be // comments
				if !strings.HasPrefix(c.Text, "//") {
					continue
				}
				if pass.Fset.PositionFor(c.Pos(), false).Column != 1 {
					// Compiler directives have to be top-level. This also
					// avoids a false positive for
					// 'import _ "unsafe" // go:linkname'
					continue
				}
				text := strings.TrimLeft(c.Text[2:], " \t")
				if len(text) == len(c.Text[2:]) {
					// There was no leading whitespace
					continue
				}
				if !strings.HasPrefix(text, "go:") {
					// Not an attempted compiler directive
					continue
				}
				text = text[3:]
				if len(text) == 0 || text[0] < 'a' || text[0] > 'z' {
					// A letter other than a-z after "go:", so unlikely to be an
					// attempted compiler directive
					continue
				}
				report.Report(pass, c,
					fmt.Sprintf(
						"ineffectual compiler directive due to extraneous space: %q",
						c.Text))
			}
		}
	}
	return nil, nil
}
