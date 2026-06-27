package util

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// IsChObj returns true if the input ast.Expr is of type clickhouse go driver <name>
func IsChObj(pass *analysis.Pass, expr ast.Expr, name string) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == "github.com/ClickHouse/clickhouse-go/v2/lib/driver" &&
		obj.Name() == name
}

func IdentName(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
