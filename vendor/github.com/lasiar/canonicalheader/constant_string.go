package canonicalheader

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

type constantString struct {
	originalValue,
	nameOfConst string

	pos token.Pos
	end token.Pos
}

func newConstantKey(info *types.Info, ident *ast.Ident) (constantString, error) {
	c, ok := info.ObjectOf(ident).(*types.Const)
	if !ok {
		return constantString{}, fmt.Errorf("type %T is not support", c)
	}

	return constantString{
		nameOfConst:   c.Name(),
		originalValue: constant.StringVal(c.Val()),
		pos:           ident.Pos(),
		end:           ident.End(),
	}, nil
}

func (c constantString) diagnostic(canonicalHeader string) analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos: c.pos,
		End: c.end,
		Message: fmt.Sprintf(
			"const %q used as a key at http.Header, but %q is not canonical, want %q",
			c.nameOfConst,
			c.originalValue,
			canonicalHeader,
		),
	}
}

func (c constantString) value() string {
	return c.originalValue
}
