package st1021

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1021",
		Run:      run,
		Requires: []*analysis.Analyzer{generated.Analyzer, inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "The documentation of an exported type should start with type's name",
		Text: `Doc comments work best as complete sentences, which
allow a wide variety of automated presentations. The first sentence
should be a one-sentence summary that starts with the name being
declared.

If every doc comment begins with the name of the item it describes,
you can use the \'doc\' subcommand of the \'go\' tool and run the output
through grep.

See https://go.dev/doc/effective_go#commentary for more
information on how to write good documentation.`,
		Since:      "2020.1",
		NonDefault: true,
		MergeIf:    lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	var genDecl *ast.GenDecl
	fn := func(node ast.Node, push bool) bool {
		if !push {
			genDecl = nil
			return false
		}
		if code.IsInTest(pass, node) {
			return false
		}

		switch node := node.(type) {
		case *ast.GenDecl:
			if node.Tok == token.IMPORT {
				return false
			}
			genDecl = node
			return true
		case *ast.TypeSpec:
			if !ast.IsExported(node.Name.Name) {
				return false
			}

			doc := node.Doc
			text, ok := docText(doc)
			if !ok {
				if len(genDecl.Specs) != 1 {
					// more than one spec in the GenDecl, don't validate the
					// docstring
					return false
				}
				if genDecl.Lparen.IsValid() {
					// 'type ( T )' is weird, don't guess the user's intention
					return false
				}
				doc = genDecl.Doc
				text, ok = docText(doc)
				if !ok {
					return false
				}
			}

			// Check comment before we strip articles in case the type's name is an article.
			if strings.HasPrefix(text, node.Name.Name+" ") {
				return false
			}

			s := text
			articles := [...]string{"A", "An", "The"}
			for _, a := range articles {
				if strings.HasPrefix(s, a+" ") {
					s = s[len(a)+1:]
					break
				}
			}
			if !strings.HasPrefix(s, node.Name.Name+" ") {
				report.Report(pass, doc, fmt.Sprintf(`comment on exported type %s should be of the form "%s ..." (with optional leading article)`, node.Name.Name, node.Name.Name), report.FilterGenerated())
			}
			return false
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		default:
			lint.ExhaustiveTypeSwitch(node)
			return false
		}
	}

	pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Nodes([]ast.Node{(*ast.GenDecl)(nil), (*ast.TypeSpec)(nil), (*ast.FuncLit)(nil), (*ast.FuncDecl)(nil)}, fn)
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
