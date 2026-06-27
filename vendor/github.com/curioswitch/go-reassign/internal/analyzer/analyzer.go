package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const FlagPattern = "pattern"

func New() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name:     "reassign",
		Doc:      "Checks that package variables are not reassigned",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      run,
	}
	a.Flags.String(FlagPattern, `^(Err.*|EOF)$`, "Pattern to match package variables against to prevent reassignment")
	return a
}

func run(pass *analysis.Pass) (interface{}, error) {
	checkRE, err := regexp.Compile(pass.Analyzer.Flags.Lookup(FlagPattern).Value.String())
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	inspect.Preorder([]ast.Node{(*ast.AssignStmt)(nil), (*ast.UnaryExpr)(nil)}, func(node ast.Node) {
		switch node := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				reportImported(pass, lhs, checkRE, "reassigning")
			}
		default:
			// TODO(chokoswitch): Consider handling operations other than assignment on globals, for example
			// taking their address.
		}
	})
	return nil, nil
}

func reportImported(pass *analysis.Pass, expr ast.Expr, checkRE *regexp.Regexp, prefix string) {
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		selectIdent, ok := x.X.(*ast.Ident)
		if !ok {
			return
		}

		var pkgPath string
		if selectObj, ok := pass.TypesInfo.Uses[selectIdent]; ok {
			pkg, ok := selectObj.(*types.PkgName)
			if !ok || pkg.Imported() == pass.Pkg {
				return
			}
			pkgPath = pkg.Imported().Path()
		}

		matches := false
		if checkRE.MatchString(x.Sel.Name) {
			matches = true
		}
		if !matches {
			// Expression may include a package name, so check that too. Support was added later so we check
			// just name and qualified name separately for compatibility.
			if checkRE.MatchString(pkgPath + "." + x.Sel.Name) {
				matches = true
			}
		}

		if matches {
			pass.Reportf(expr.Pos(), "%s variable %s in other package %s", prefix, x.Sel.Name, selectIdent.Name)
		}
	case *ast.Ident:
		use, ok := pass.TypesInfo.Uses[x].(*types.Var)
		if !ok {
			return
		}

		if use.Pkg() == pass.Pkg {
			return
		}

		if !checkRE.MatchString(x.Name) {
			return
		}

		pass.Reportf(expr.Pos(), "%s variable %s from other package %s", prefix, x.Name, use.Pkg().Path())
	}
}
