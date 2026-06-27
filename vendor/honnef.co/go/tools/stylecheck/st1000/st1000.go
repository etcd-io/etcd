package st1000

import (
	"fmt"
	"go/ast"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name: "ST1000",
		Run:  run,
	},
	Doc: &lint.RawDocumentation{
		Title: `Incorrect or missing package comment`,
		Text: `Packages must have a package comment that is formatted according to
the guidelines laid out in
https://go.dev/wiki/CodeReviewComments#package-comments.`,
		Since:      "2019.1",
		NonDefault: true,
		MergeIf:    lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	// - At least one file in a non-main package should have a package comment
	//
	// - The comment should be of the form
	// "Package x ...". This has a slight potential for false
	// positives, as multiple files can have package comments, in
	// which case they get appended. But that doesn't happen a lot in
	// the real world.

	if pass.Pkg.Name() == "main" {
		return nil, nil
	}
	hasDocs := false
	for _, f := range pass.Files {
		if code.IsInTest(pass, f) {
			continue
		}
		text, ok := docText(f.Doc)
		if ok {
			hasDocs = true
			prefix := "Package " + f.Name.Name
			isNonAlpha := func(b byte) bool {
				// This check only considers ASCII, which can lead to false negatives for some malformed package
				// comments.
				return !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z'))
			}
			if !strings.HasPrefix(text, prefix) || (len(text) > len(prefix) && !isNonAlpha(text[len(prefix)])) {
				report.Report(pass, f.Doc, fmt.Sprintf(`package comment should be of the form "%s..."`, prefix))
			}
		}
	}

	if !hasDocs {
		for _, f := range pass.Files {
			if code.IsInTest(pass, f) {
				continue
			}
			report.Report(pass, f, "at least one file in a package should have a package comment", report.ShortRange())
		}
	}
	return nil, nil
}

func docText(doc *ast.CommentGroup) (string, bool) {
	if doc == nil {
		return "", false
	}
	// We trim spaces primarily because of /**/ style comments, which often have leading space.
	text := strings.TrimSpace(doc.Text())
	return text, text != ""
}
