package s1038

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1038",
		Run:      run,
		Requires: append([]*analysis.Analyzer{generated.Analyzer}, code.RequiredAnalyzers...),
	},
	Doc: &lint.RawDocumentation{
		Title:   "Unnecessarily complex way of printing formatted string",
		Text:    `Instead of using \'fmt.Print(fmt.Sprintf(...))\', one can use \'fmt.Printf(...)\'.`,
		Since:   "2020.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkPrintSprintQ = pattern.MustParse(`
		(Or
			(CallExpr
				fn@(Or
					(Symbol "fmt.Print")
					(Symbol "fmt.Sprint")
					(Symbol "fmt.Println")
					(Symbol "fmt.Sprintln"))
				[(CallExpr (Symbol "fmt.Sprintf") f:_)])
			(CallExpr
				fn@(Or
					(Symbol "fmt.Fprint")
					(Symbol "fmt.Fprintln"))
				[_ (CallExpr (Symbol "fmt.Sprintf") f:_)]))`)

	checkTestingErrorSprintfQ = pattern.MustParse(`
		(CallExpr
			sel@(SelectorExpr
				recv
				(Ident
					name@(Or
						"Error"
						"Fatal"
						"Fatalln"
						"Log"
						"Panic"
						"Panicln"
						"Print"
						"Println"
						"Skip")))
			[(CallExpr (Symbol "fmt.Sprintf") args)])`)

	checkLogSprintfQ = pattern.MustParse(`
		(CallExpr
			(Symbol
				(Or
					"log.Fatal"
					"log.Fatalln"
					"log.Panic"
					"log.Panicln"
					"log.Print"
					"log.Println"))
			[(CallExpr (Symbol "fmt.Sprintf") args)])`)

	checkSprintfMapping = map[string]struct {
		recv        string
		alternative string
	}{
		"(*testing.common).Error": {"(*testing.common)", "Errorf"},
		"(testing.TB).Error":      {"(testing.TB)", "Errorf"},
		"(*testing.common).Fatal": {"(*testing.common)", "Fatalf"},
		"(testing.TB).Fatal":      {"(testing.TB)", "Fatalf"},
		"(*testing.common).Log":   {"(*testing.common)", "Logf"},
		"(testing.TB).Log":        {"(testing.TB)", "Logf"},
		"(*testing.common).Skip":  {"(*testing.common)", "Skipf"},
		"(testing.TB).Skip":       {"(testing.TB)", "Skipf"},
		"(*log.Logger).Fatal":     {"(*log.Logger)", "Fatalf"},
		"(*log.Logger).Fatalln":   {"(*log.Logger)", "Fatalf"},
		"(*log.Logger).Panic":     {"(*log.Logger)", "Panicf"},
		"(*log.Logger).Panicln":   {"(*log.Logger)", "Panicf"},
		"(*log.Logger).Print":     {"(*log.Logger)", "Printf"},
		"(*log.Logger).Println":   {"(*log.Logger)", "Printf"},
		"log.Fatal":               {"", "log.Fatalf"},
		"log.Fatalln":             {"", "log.Fatalf"},
		"log.Panic":               {"", "log.Panicf"},
		"log.Panicln":             {"", "log.Panicf"},
		"log.Print":               {"", "log.Printf"},
		"log.Println":             {"", "log.Printf"},
	}
)

func run(pass *analysis.Pass) (any, error) {
	fmtPrintf := func(node ast.Node) {
		m, ok := code.Match(pass, checkPrintSprintQ, node)
		if !ok {
			return
		}

		name := m.State["fn"].(*types.Func).Name()
		var msg string
		switch name {
		case "Print", "Fprint", "Sprint":
			newname := name + "f"
			msg = fmt.Sprintf("should use fmt.%s instead of fmt.%s(fmt.Sprintf(...))", newname, name)
		case "Println", "Fprintln", "Sprintln":
			if _, ok := m.State["f"].(*ast.BasicLit); !ok {
				// This may be an instance of
				// fmt.Println(fmt.Sprintf(arg, ...)) where arg is an
				// externally provided format string and the caller
				// cannot guarantee that the format string ends with a
				// newline.
				return
			}
			newname := name[:len(name)-2] + "f"
			msg = fmt.Sprintf("should use fmt.%s instead of fmt.%s(fmt.Sprintf(...)) (but don't forget the newline)", newname, name)
		}
		report.Report(pass, node, msg,
			report.FilterGenerated())
	}

	methSprintf := func(node ast.Node) {
		m, ok := code.Match(pass, checkTestingErrorSprintfQ, node)
		if !ok {
			return
		}
		mapped, ok := checkSprintfMapping[code.CallName(pass, node.(*ast.CallExpr))]
		if !ok {
			return
		}

		// Ensure that Errorf/Fatalf refer to the right method
		recvTV, ok := pass.TypesInfo.Types[m.State["recv"].(ast.Expr)]
		if !ok {
			return
		}
		obj, _, _ := types.LookupFieldOrMethod(recvTV.Type, recvTV.Addressable(), nil, mapped.alternative)
		f, ok := obj.(*types.Func)
		if !ok {
			return
		}
		if typeutil.FuncName(f) != mapped.recv+"."+mapped.alternative {
			return
		}

		alt := &ast.SelectorExpr{
			X:   m.State["recv"].(ast.Expr),
			Sel: &ast.Ident{Name: mapped.alternative},
		}
		report.Report(pass, node, fmt.Sprintf("should use %s(...) instead of %s(fmt.Sprintf(...))", report.Render(pass, alt), report.Render(pass, m.State["sel"].(*ast.SelectorExpr))))
	}

	pkgSprintf := func(node ast.Node) {
		_, ok := code.Match(pass, checkLogSprintfQ, node)
		if !ok {
			return
		}
		callName := code.CallName(pass, node.(*ast.CallExpr))
		mapped, ok := checkSprintfMapping[callName]
		if !ok {
			return
		}
		report.Report(pass, node, fmt.Sprintf("should use %s(...) instead of %s(fmt.Sprintf(...))", mapped.alternative, callName))
	}

	fn := func(node ast.Node) {
		fmtPrintf(node)
		// TODO(dh): add suggested fixes
		methSprintf(node)
		pkgSprintf(node)
	}
	if !code.CouldMatchAny(pass, checkLogSprintfQ, checkPrintSprintQ, checkTestingErrorSprintfQ) {
		return nil, nil
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
