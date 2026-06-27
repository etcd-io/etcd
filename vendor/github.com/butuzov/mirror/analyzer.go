package mirror

import (
	"flag"
	"go/ast"
	"strings"

	"github.com/butuzov/mirror/internal/checker"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

func NewAnalyzer() *analysis.Analyzer {
	flags := flags()

	return &analysis.Analyzer{
		Name: "mirror",
		Doc:  "reports wrong mirror patterns of bytes/strings usage",
		Run:  run,
		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
		Flags: flags,
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	withTests := pass.Analyzer.Flags.Lookup("with-tests").Value.String() == "true"
	// --- Reporting violations via issues ---------------------------------------
	for _, violation := range Run(pass, withTests) {
		pass.Report(violation.Diagnostic(pass.Fset))
	}

	return nil, nil
}

func Run(pass *analysis.Pass, withTests bool) []*checker.Violation {
	violations := []*checker.Violation{}
	// --- Setup -----------------------------------------------------------------

	check := checker.New(
		BytesFunctions, BytesBufferMethods,
		RegexpFunctions, RegexpRegexpMethods,
		StringFunctions, StringsBuilderMethods,
		MaphashMethods, MaphashFunctions,
		BufioMethods, HTTPTestMethods,
		OsFileMethods, UTF8Functions,
	)

	check.Type = checker.WrapType(pass.TypesInfo)
	check.Print = checker.WrapPrint(pass.Fset)

	ins, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	imports := checker.Load(pass.Fset, ins)

	// --- Preorder Checker ------------------------------------------------------
	ins.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		callExpr := n.(*ast.CallExpr)
		fileName := pass.Fset.Position(callExpr.Pos()).Filename

		if !withTests && strings.HasSuffix(fileName, "_test.go") {
			return
		}

		// -------------------------------------------------------------------------
		switch expr := callExpr.Fun.(type) {
		// NOTE(butuzov): Regular calls (`*ast.SelectorExpr`) like strings.HasPrefix
		//                or re.Match are handled by this check
		case *ast.SelectorExpr:

			x, ok := expr.X.(*ast.Ident)
			if !ok {
				return
			}

			// TODO(butuzov): Add check for the ast.ParenExpr in e.Fun so we can
			//                target the constructions like this (and other calls)
			// -----------------------------------------------------------------------
			// Example:
			//       (&maphash.Hash{}).Write([]byte("foobar"))
			// -----------------------------------------------------------------------

			// Case 1: Is this is a function call?
			pkgName, name := x.Name, expr.Sel.Name
			if pkg, ok := imports.Lookup(fileName, pkgName); ok {
				if v := check.Match(pkg, name); v != nil {
					if args, found := check.Handle(v, callExpr); found {
						violations = append(violations, v.With(check.Print(expr.X), callExpr, args))
					}
					return
				}
			}

			// Case 2: Is this is a method call?
			tv := pass.TypesInfo.Types[expr.X]
			if !tv.IsValue() || tv.Type == nil {
				return
			}

			pkgStruct, name := cleanAsterisk(tv.Type.String()), expr.Sel.Name
			for _, v := range check.Matches(pkgStruct, name) {
				if v == nil {
					continue
				}

				if args, found := check.Handle(v, callExpr); found {
					violations = append(violations, v.With(check.Print(expr.X), callExpr, args))
					return
				}
			}

		case *ast.Ident:
			// NOTE(butuzov): Special case of "." imported packages, only functions.

			if pkg, ok := imports.Lookup(fileName, "."); ok {
				if v := check.Match(pkg, expr.Name); v != nil {
					if args, found := check.Handle(v, callExpr); found {
						violations = append(violations, v.With(nil, callExpr, args))
					}
					return
				}
			}
		}
	})

	return violations
}

func flags() flag.FlagSet {
	set := flag.NewFlagSet("", flag.PanicOnError)
	set.Bool("with-tests", false, "do not skip tests in reports")
	set.Bool("with-debug", false, "debug linter run (development only)")
	return *set
}

func cleanAsterisk(s string) string {
	if strings.HasPrefix(s, "*") {
		return s[1:]
	}

	return s
}
