package usetesting

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// because [os.CreateTemp] takes 2 args.
const nbArgCreateTemp = 2

func (a *analyzer) reportCallExpr(pass *analysis.Pass, ce *ast.CallExpr, fnInfo *FuncInfo) bool {
	if !a.osCreateTemp {
		return false
	}

	if len(ce.Args) != nbArgCreateTemp {
		return false
	}

	switch fun := ce.Fun.(type) {
	case *ast.SelectorExpr:
		if fun.Sel == nil || fun.Sel.Name != createTempName {
			return false
		}

		expr, ok := fun.X.(*ast.Ident)
		if !ok {
			return false
		}

		if expr.Name == osPkgName && isFirstArgEmptyString(ce) {
			pass.Report(diagnosticOSCreateTemp(ce, fnInfo))

			return true
		}

	case *ast.Ident:
		if fun.Name != createTempName {
			return false
		}

		pkgName := getPkgNameFromType(pass, fun)

		if pkgName == osPkgName && isFirstArgEmptyString(ce) {
			pass.Report(diagnosticOSCreateTemp(ce, fnInfo))

			return true
		}
	}

	return false
}

func diagnosticOSCreateTemp(ce *ast.CallExpr, fnInfo *FuncInfo) analysis.Diagnostic {
	diagnostic := analysis.Diagnostic{
		Pos: ce.Pos(),
		Message: fmt.Sprintf(
			`%s.%s("", ...) could be replaced by %[1]s.%[2]s(%s.%s(), ...) in %s`,
			osPkgName, createTempName, fnInfo.ArgName, tempDirName, fnInfo.Name,
		),
	}

	// Skip `<t/b>` arg names.
	if !strings.Contains(fnInfo.ArgName, "<") {
		g := &ast.CallExpr{
			Fun: ce.Fun,
			Args: []ast.Expr{
				&ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   &ast.Ident{Name: fnInfo.ArgName},
						Sel: &ast.Ident{Name: tempDirName},
					},
				},
				ce.Args[1],
			},
		}

		buf := bytes.NewBuffer(nil)

		err := printer.Fprint(buf, token.NewFileSet(), g)
		if err != nil {
			diagnostic.Message = fmt.Sprintf("Suggested fix error: %v", err)
			return diagnostic
		}

		diagnostic.SuggestedFixes = append(diagnostic.SuggestedFixes, analysis.SuggestedFix{
			TextEdits: []analysis.TextEdit{{
				Pos:     ce.Pos(),
				End:     ce.End(),
				NewText: buf.Bytes(),
			}},
		})
	}

	return diagnostic
}

func (a *analyzer) reportSelector(pass *analysis.Pass, se *ast.SelectorExpr, fnInfo *FuncInfo, geGo124 bool) bool {
	if se.Sel == nil || !se.Sel.IsExported() {
		return false
	}

	ident, ok := se.X.(*ast.Ident)
	if !ok {
		return false
	}

	return a.report(pass, se, ident.Name, se.Sel.Name, fnInfo, geGo124)
}

func (a *analyzer) reportIdent(pass *analysis.Pass, ident *ast.Ident, fnInfo *FuncInfo, geGo124 bool) bool {
	if !ident.IsExported() {
		return false
	}

	if !slices.Contains(a.fieldNames, ident.Name) {
		return false
	}

	pkgName := getPkgNameFromType(pass, ident)

	return a.report(pass, ident, pkgName, ident.Name, fnInfo, geGo124)
}

//nolint:gocyclo // The complexity is expected by the number of cases to check.
func (a *analyzer) report(pass *analysis.Pass, rg analysis.Range, origPkgName, origName string, fnInfo *FuncInfo, geGo124 bool) bool {
	switch {
	case a.osMkdirTemp && origPkgName == osPkgName && origName == mkdirTempName:
		report(pass, rg, origPkgName, origName, tempDirName, fnInfo)

	case a.osTempDir && origPkgName == osPkgName && origName == tempDirName:
		report(pass, rg, origPkgName, origName, tempDirName, fnInfo)

	case a.osSetenv && origPkgName == osPkgName && origName == setenvName:
		report(pass, rg, origPkgName, origName, setenvName, fnInfo)

	case geGo124 && a.osChdir && origPkgName == osPkgName && origName == chdirName:
		report(pass, rg, origPkgName, origName, chdirName, fnInfo)

	case geGo124 && a.contextBackground && origPkgName == contextPkgName && origName == backgroundName:
		report(pass, rg, origPkgName, origName, contextName, fnInfo)

	case geGo124 && a.contextTodo && origPkgName == contextPkgName && origName == todoName:
		report(pass, rg, origPkgName, origName, contextName, fnInfo)

	default:
		return false
	}

	return true
}

func report(pass *analysis.Pass, rg analysis.Range, origPkgName, origName, expectName string, fnInfo *FuncInfo) {
	diagnostic := analysis.Diagnostic{
		Pos: rg.Pos(),
		Message: fmt.Sprintf("%s.%s() could be replaced by %s.%s() in %s",
			origPkgName, origName, fnInfo.ArgName, expectName, fnInfo.Name,
		),
	}

	// Skip `<t/b>` arg names.
	// Only applies on `context.XXX` because the nb of return parameters is the same as the replacement.
	if !strings.Contains(fnInfo.ArgName, "<") && origPkgName == contextPkgName {
		diagnostic.SuggestedFixes = append(diagnostic.SuggestedFixes, analysis.SuggestedFix{
			TextEdits: []analysis.TextEdit{{
				Pos:     rg.Pos(),
				End:     rg.End(),
				NewText: []byte(fmt.Sprintf("%s.%s", fnInfo.ArgName, expectName)),
			}},
		})
	}

	pass.Report(diagnostic)
}

func isFirstArgEmptyString(ce *ast.CallExpr) bool {
	bl, ok := ce.Args[0].(*ast.BasicLit)
	if !ok {
		return false
	}

	return bl.Kind == token.STRING && bl.Value == `""`
}

func getPkgNameFromType(pass *analysis.Pass, ident *ast.Ident) string {
	o := pass.TypesInfo.ObjectOf(ident)

	if o == nil || o.Pkg() == nil {
		return ""
	}

	return o.Pkg().Name()
}
