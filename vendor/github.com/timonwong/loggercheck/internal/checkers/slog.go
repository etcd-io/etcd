package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

type Slog struct {
	General
}

func (z Slog) FilterKeyAndValues(pass *analysis.Pass, keyAndValues []ast.Expr) []ast.Expr {
	// check slog.Group() constructed group slog.Attr
	// since we also check `slog.Group` so it is OK skip here
	return filterKeyAndValues(pass, keyAndValues, "Attr")
}

var _ Checker = (*Slog)(nil)
