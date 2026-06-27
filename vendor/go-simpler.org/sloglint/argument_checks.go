package sloglint

import (
	"go/ast"
	"slices"

	"golang.org/x/tools/go/analysis"
)

func noMixedArguments(pass *analysis.Pass, keys, attrs []ast.Expr) {
	if len(keys) == 0 {
		return
	}
	for _, attr := range attrs {
		if call, ok := attr.(*ast.CallExpr); ok && funcName(pass.TypesInfo, call) == "log/slog.Group" {
			continue // Special case: slog.Group() should always be allowed.
		}
		pass.ReportRangef(attr, "key-value pairs and attributes should not be mixed")
		return
	}
}

func keyValuePairsOnly(pass *analysis.Pass, attrs []ast.Expr) {
	for _, attr := range attrs {
		if call, ok := attr.(*ast.CallExpr); ok && funcName(pass.TypesInfo, call) == "log/slog.Group" {
			continue // Special case: slog.Group() should always be allowed.
		}
		pass.ReportRangef(attr, "attributes should not be used")
		return
	}
}

func attributesOnly(pass *analysis.Pass, keys []ast.Expr) {
	for _, key := range keys {
		pass.ReportRangef(key, "key-value pairs should not be used")
		return
	}
}

func argumentsOnSeparateLines(pass *analysis.Pass, keys, attrs []ast.Expr) {
	args := slices.Concat(keys, attrs)
	if len(args) <= 1 {
		return // Special case: slog.Info("msg", "key", "value") is fine.
	}

	prevLine := pass.Fset.Position(args[0].Pos()).Line
	for _, arg := range args[1:] {
		currLine := pass.Fset.Position(arg.Pos()).Line
		if currLine == prevLine {
			pass.Reportf(arg.Pos(), "arguments should be put on separate lines")
			return
		}
		prevLine = currLine
	}
}
