package sa1006

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
		Name:     "SA1006",
		Run:      run,
		Requires: code.RequiredAnalyzers,
	},
	Doc: &lint.RawDocumentation{
		Title: `\'Printf\' with dynamic first argument and no further arguments`,
		Text: `Using \'fmt.Printf\' with a dynamic first argument can lead to unexpected
output. The first argument is a format string, where certain character
combinations have special meaning. If, for example, a user were to
enter a string such as

    Interest rate: 5%

and you printed it with

    fmt.Printf(s)

it would lead to the following output:

    Interest rate: 5%!(NOVERB).

Similarly, forming the first parameter via string concatenation with
user input should be avoided for the same reason. When printing user
input, either use a variant of \'fmt.Print\', or use the \'%s\' Printf verb
and pass the string as an argument.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var query1 = pattern.MustParse(`
	(CallExpr
		(Symbol
			name@(Or
				"fmt.Errorf"
				"fmt.Printf"
				"fmt.Sprintf"
				"log.Fatalf"
				"log.Panicf"
				"log.Printf"
				"(*log.Logger).Printf"
				"(*testing.common).Logf"
				"(*testing.common).Errorf"
				"(*testing.common).Fatalf"
				"(*testing.common).Skipf"
				"(testing.TB).Logf"
				"(testing.TB).Errorf"
				"(testing.TB).Fatalf"
				"(testing.TB).Skipf"))
		format:[])
`)

var query2 = pattern.MustParse(`(CallExpr (Symbol "fmt.Fprintf") _:format:[])`)

func run(pass *analysis.Pass) (any, error) {
	for node, m := range code.Matches(pass, query1, query2) {
		call := node.(*ast.CallExpr)
		name, ok := m.State["name"].(string)
		if !ok {
			name = "fmt.Fprintf"
		}

		arg := m.State["format"].(ast.Expr)
		switch arg.(type) {
		case *ast.CallExpr, *ast.Ident:
		default:
			continue
		}

		if _, ok := pass.TypesInfo.TypeOf(arg).(*types.Tuple); ok {
			// the called function returns multiple values and got
			// splatted into the call. for all we know, it is
			// returning good arguments.
			continue
		}

		var alt string
		if name == "fmt.Errorf" {
			// The alternative to fmt.Errorf isn't fmt.Error but errors.New
			alt = "errors.New"
		} else {
			// This can be either a function call like log.Printf or a method call with an
			// arbitrarily complex selector, such as foo.bar[0].Printf. In either case,
			// all we have to do is remove the final 'f' from the existing call.Fun
			// expression.
			alt = report.Render(pass, call.Fun)
			alt = alt[:len(alt)-1]
		}
		report.Report(pass, call,
			"printf-style function with dynamic format string and no further arguments should use print-style function instead",
			report.Fixes(edit.Fix(fmt.Sprintf("Use %s instead of %s", alt, name), edit.ReplaceWithString(call.Fun, alt))))
	}
	return nil, nil
}
