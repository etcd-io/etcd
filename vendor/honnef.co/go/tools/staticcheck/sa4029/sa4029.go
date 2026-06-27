package sa4029

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4029",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: "Ineffective attempt at sorting slice",
		Text: `
\'sort.Float64Slice\', \'sort.IntSlice\', and \'sort.StringSlice\' are
types, not functions. Doing \'x = sort.StringSlice(x)\' does nothing,
especially not sort any values. The correct usage is
\'sort.Sort(sort.StringSlice(x))\' or \'sort.StringSlice(x).Sort()\',
but there are more convenient helpers, namely \'sort.Float64s\',
\'sort.Ints\', and \'sort.Strings\'.
`,
		Since:    "2022.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var ineffectiveSortQ = pattern.MustParse(`(AssignStmt target@(Ident _) "=" (CallExpr typ@(Symbol (Or "sort.Float64Slice" "sort.IntSlice" "sort.StringSlice")) [target]))`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, ineffectiveSortQ) {
		_, ok := types.Unalias(pass.TypesInfo.TypeOf(m.State["target"].(ast.Expr))).(*types.Slice)
		if !ok {
			// Avoid flagging 'x = sort.StringSlice(x)' where TypeOf(x) == sort.StringSlice
			continue
		}

		var alternative string
		typeName := types.TypeString(types.Unalias(m.State["typ"].(*types.TypeName).Type()), nil)
		switch typeName {
		case "sort.Float64Slice":
			alternative = "Float64s"
		case "sort.IntSlice":
			alternative = "Ints"
		case "sort.StringSlice":
			alternative = "Strings"
		default:
			panic(fmt.Sprintf("unreachable: %q", typeName))
		}

		r := &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: "sort"},
				Sel: &ast.Ident{Name: alternative},
			},
			Args: []ast.Expr{m.State["target"].(ast.Expr)},
		}

		report.Report(pass, node,
			fmt.Sprintf("%s is a type, not a function, and %s doesn't sort your values; consider using sort.%s instead",
				typeName,
				report.Render(pass, node.(*ast.AssignStmt).Rhs[0]),
				alternative),
			report.Fixes(edit.Fix(fmt.Sprintf("Replace with call to sort.%s", alternative), edit.ReplaceWithNode(pass.Fset, node, r))))
	}
	return nil, nil
}
