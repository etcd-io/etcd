package checkers

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

type Zap struct {
	General
}

func (z Zap) FilterKeyAndValues(pass *analysis.Pass, keyAndValues []ast.Expr) []ast.Expr {
	// Skip any zapcore.Field we found
	// This is a strongly-typed field. Consume it and move on.
	// Actually it's go.uber.org/zap/zapcore.Field, however for simplicity
	// we don't check the import path
	return filterKeyAndValues(pass, keyAndValues, "Field")
}

var _ Checker = (*Zap)(nil)
