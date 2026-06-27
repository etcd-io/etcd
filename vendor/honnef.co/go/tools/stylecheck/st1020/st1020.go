package st1020

import (
	"fmt"
	"go/ast"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1020",
		Run:      run,
		Requires: []*analysis.Analyzer{generated.Analyzer, inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "The documentation of an exported function should start with the function's name",
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
	fn := func(node ast.Node) {
		if code.IsInTest(pass, node) {
			return
		}

		decl := node.(*ast.FuncDecl)
		text, ok := docText(decl.Doc)
		if !ok {
			return
		}
		if !ast.IsExported(decl.Name.Name) {
			return
		}
		if strings.HasPrefix(text, "Deprecated: ") {
			return
		}

		kind := "function"
		if decl.Recv != nil {
			kind = "method"
			var ident *ast.Ident
			T := decl.Recv.List[0].Type
			if T_, ok := T.(*ast.StarExpr); ok {
				T = T_.X
			}
			switch T := T.(type) {
			case *ast.IndexExpr:
				ident = T.X.(*ast.Ident)
			case *ast.IndexListExpr:
				ident = T.X.(*ast.Ident)
			case *ast.Ident:
				ident = T
			default:
				lint.ExhaustiveTypeSwitch(T)
			}
			if !ast.IsExported(ident.Name) {
				return
			}
		}
		prefix := decl.Name.Name + " "
		if !strings.HasPrefix(text, prefix) {
			report.Report(pass, decl.Doc, fmt.Sprintf(`comment on exported %s %s should be of the form "%s..."`, kind, decl.Name.Name, prefix), report.FilterGenerated())
		}
	}

	code.Preorder(pass, fn, (*ast.FuncDecl)(nil))
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
