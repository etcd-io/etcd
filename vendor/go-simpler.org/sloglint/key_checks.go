package sloglint

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strconv"

	"github.com/ettle/strcase"
	"golang.org/x/tools/go/analysis"
)

func constantKeys(pass *analysis.Pass, key ast.Expr) {
	if sel, ok := key.(*ast.SelectorExpr); ok {
		key = sel.Sel // The key is defined in another package, e.g. pkg.ConstKey.
	}
	if ident, ok := key.(*ast.Ident); ok {
		if _, ok := pass.TypesInfo.ObjectOf(ident).(*types.Const); ok {
			return
		}
	}
	name, _ := keyName(key)
	pass.ReportRangef(key, "the %q key should be a constant", name)
}

func allowedKeys(pass *analysis.Pass, key ast.Expr, allowed []string) {
	if name, ok := keyName(key); ok && !slices.Contains(allowed, name) {
		pass.ReportRangef(key, "the %q key is not allowed and should not be used", name)
	}
}

func forbiddenKeys(pass *analysis.Pass, key ast.Expr, forbidden []string) {
	if name, ok := keyName(key); ok && slices.Contains(forbidden, name) {
		pass.ReportRangef(key, "the %q key is forbidden and should not be used", name)
	}
}

func keyNamingCase(pass *analysis.Pass, key ast.Expr, caseName string) {
	name, ok := keyName(key)
	if !ok {
		return
	}

	var caseFn func(string) string
	switch caseName {
	case keyNamingCaseSnake:
		caseFn = strcase.ToSnake
	case keyNamingCaseKebab:
		caseFn = strcase.ToKebab
	case keyNamingCaseCamel:
		caseFn = strcase.ToCamel
	case keyNamingCasePascal:
		caseFn = strcase.ToPascal
	}

	if name == caseFn(name) {
		return
	}

	pass.Report(analysis.Diagnostic{
		Pos:     key.Pos(),
		End:     key.End(),
		Message: fmt.Sprintf("keys should be written in %s", caseFn(caseName+" case")),
		SuggestedFixes: []analysis.SuggestedFix{{
			TextEdits: []analysis.TextEdit{{
				Pos:     key.Pos(),
				End:     key.End(),
				NewText: strconv.AppendQuote(nil, caseFn(name)),
			}},
		}},
	})
}
