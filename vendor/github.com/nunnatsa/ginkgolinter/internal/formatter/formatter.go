package formatter

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
)

type GoFmtFormatter struct {
	fset *token.FileSet
}

func NewGoFmtFormatter(fset *token.FileSet) *GoFmtFormatter {
	return &GoFmtFormatter{fset: fset}
}

func (f GoFmtFormatter) Format(exp ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, f.fset, exp)
	return buf.String()
}
