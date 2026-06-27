package astequal

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"

	"github.com/google/go-cmp/cmp"
)

func Diff(x, y ast.Node) string {
	var buf bytes.Buffer
	format.Node(&buf, token.NewFileSet(), x)
	s1 := buf.String()

	buf.Reset()
	format.Node(&buf, token.NewFileSet(), y)
	s2 := buf.String()

	// TODO(cristaloleg): replace with a more lightweight diff impl.
	return cmp.Diff(s1, s2)
}
