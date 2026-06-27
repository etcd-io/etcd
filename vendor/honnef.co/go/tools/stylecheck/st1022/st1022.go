package st1022

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
		Name:     "ST1022",
		Run:      run,
		Requires: []*analysis.Analyzer{generated.Analyzer, inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "The documentation of an exported variable or constant should start with variable's name",
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
		case *ast.ValueSpec:
			if genDecl.Lparen.IsValid() || len(node.Names) > 1 {
				// Don't try to guess the user's intention
				return false
			}
			name := node.Names[0].Name
			if !ast.IsExported(name) {
				return false
			}
			text, ok := docText(genDecl.Doc)
			if !ok {
				return false
			}
			prefix := name + " "
			if !strings.HasPrefix(text, prefix) {
				kind := "var"
				if genDecl.Tok == token.CONST {
					kind = "const"
				}
				report.Report(pass, genDecl.Doc, fmt.Sprintf(`comment on exported %s %s should be of the form "%s..."`, kind, name, prefix), report.FilterGenerated())
			}
			return false
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		default:
			lint.ExhaustiveTypeSwitch(node)
			return false
		}
	}

	pass.ResultOf[inspect.Analyzer].(*inspector.Inspector).Nodes([]ast.Node{(*ast.GenDecl)(nil), (*ast.ValueSpec)(nil), (*ast.FuncLit)(nil), (*ast.FuncDecl)(nil)}, fn)
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
