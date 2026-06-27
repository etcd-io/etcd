package pkg

import (
	"go/ast"
	"go/token"
	"slices"
)

func exprEqual(x, y ast.Expr) bool {
	if x == nil || y == nil {
		return x == y
	}

	switch x := x.(type) {
	case *ast.Ident:
		y, ok := y.(*ast.Ident)
		return ok && identEqual(x, y)

	case *ast.BasicLit:
		y, ok := y.(*ast.BasicLit)
		return ok && basicLitEqual(x, y)

	case *ast.FuncLit:
		y, ok := y.(*ast.FuncLit)
		return ok && funcLitEqual(x, y)

	case *ast.CompositeLit:
		y, ok := y.(*ast.CompositeLit)
		return ok && compositeLitEqual(x, y)

	case *ast.ParenExpr:
		y, ok := y.(*ast.ParenExpr)
		return ok && parenExprEqual(x, y)

	case *ast.SelectorExpr:
		y, ok := y.(*ast.SelectorExpr)
		return ok && selectorExprEqual(x, y)

	case *ast.IndexExpr:
		y, ok := y.(*ast.IndexExpr)
		return ok && indexExprEqual(x, y)

	case *ast.IndexListExpr:
		y, ok := y.(*ast.IndexListExpr)
		return ok && indexListExprEqual(x, y)

	case *ast.SliceExpr:
		y, ok := y.(*ast.SliceExpr)
		return ok && sliceExprEqual(x, y)

	case *ast.TypeAssertExpr:
		y, ok := y.(*ast.TypeAssertExpr)
		return ok && typeAssertExprEqual(x, y)

	case *ast.CallExpr:
		y, ok := y.(*ast.CallExpr)
		return ok && callExprEqual(x, y)

	case *ast.StarExpr:
		y, ok := y.(*ast.StarExpr)
		return ok && starExprEqual(x, y)

	case *ast.UnaryExpr:
		y, ok := y.(*ast.UnaryExpr)
		return ok && unaryExprEqual(x, y)

	case *ast.BinaryExpr:
		y, ok := y.(*ast.BinaryExpr)
		return ok && binaryExprEqual(x, y)

	case *ast.KeyValueExpr:
		y, ok := y.(*ast.KeyValueExpr)
		return ok && keyValueExprEqual(x, y)

	case *ast.ArrayType:
		y, ok := y.(*ast.ArrayType)
		return ok && arrayTypeEqual(x, y)

	case *ast.StructType:
		y, ok := y.(*ast.StructType)
		return ok && structTypeEqual(x, y)

	case *ast.FuncType:
		y, ok := y.(*ast.FuncType)
		return ok && funcTypeEqual(x, y)

	case *ast.InterfaceType:
		y, ok := y.(*ast.InterfaceType)
		return ok && interfaceTypeEqual(x, y)

	case *ast.MapType:
		y, ok := y.(*ast.MapType)
		return ok && mapTypeEqual(x, y)

	case *ast.ChanType:
		y, ok := y.(*ast.ChanType)
		return ok && chanTypeEqual(x, y)

	case *ast.Ellipsis:
		y, ok := y.(*ast.Ellipsis)
		return ok && ellipsisEqual(x, y)

	default:
		return false
	}
}

func identEqual(x, y *ast.Ident) bool {
	if x == nil || y == nil {
		return x == y
	}
	return x.Name == y.Name
}

func keyValueExprEqual(x, y *ast.KeyValueExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.Key, y.Key) && exprEqual(x.Value, y.Value)
}

func arrayTypeEqual(x, y *ast.ArrayType) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.Len, y.Len) && exprEqual(x.Elt, y.Elt)
}

func structTypeEqual(x, y *ast.StructType) bool {
	if x == nil || y == nil {
		return x == y
	}
	return fieldListEqual(x.Fields, y.Fields)
}

func funcTypeEqual(x, y *ast.FuncType) bool {
	if x == nil || y == nil {
		return x == y
	}
	return fieldListEqual(x.Params, y.Params) && fieldListEqual(x.Results, y.Results) && fieldListEqual(x.TypeParams, y.TypeParams)
}

func basicLitEqual(x, y *ast.BasicLit) bool {
	if x == nil || y == nil {
		return x == y
	}
	return x.Kind == y.Kind && x.Value == y.Value
}

func funcLitEqual(x, y *ast.FuncLit) bool {
	if x == nil || y == nil {
		return x == y
	}
	return funcTypeEqual(x.Type, y.Type)
}

func compositeLitEqual(x, y *ast.CompositeLit) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.Type, y.Type) && exprSliceEqual(x.Elts, y.Elts)
}

func selectorExprEqual(x, y *ast.SelectorExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.X, y.X) && identEqual(x.Sel, y.Sel)
}

func indexExprEqual(x, y *ast.IndexExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.X, y.X) && exprEqual(x.Index, y.Index)
}

func indexListExprEqual(x, y *ast.IndexListExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.X, y.X) && exprSliceEqual(x.Indices, y.Indices)
}

func sliceExprEqual(x, y *ast.SliceExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.X, y.X) && exprEqual(x.Low, y.Low) && exprEqual(x.High, y.High) && exprEqual(x.Max, y.Max)
}

func typeAssertExprEqual(x, y *ast.TypeAssertExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.X, y.X) && exprEqual(x.Type, y.Type)
}

func interfaceTypeEqual(x, y *ast.InterfaceType) bool {
	if x == nil || y == nil {
		return x == y
	}
	return fieldListEqual(x.Methods, y.Methods)
}

func mapTypeEqual(x, y *ast.MapType) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.Key, y.Key) && exprEqual(x.Value, y.Value)
}

func chanTypeEqual(x, y *ast.ChanType) bool {
	if x == nil || y == nil {
		return x == y
	}
	return x.Dir == y.Dir && exprEqual(x.Value, y.Value)
}

func callExprEqual(x, y *ast.CallExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.Fun, y.Fun) && exprSliceEqual(x.Args, y.Args) && x.Ellipsis.IsValid() == y.Ellipsis.IsValid()
}

func ellipsisEqual(x, y *ast.Ellipsis) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.Elt, y.Elt)
}

func unaryExprEqual(x, y *ast.UnaryExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return x.Op == y.Op && exprEqual(x.X, y.X)
}

func binaryExprEqual(x, y *ast.BinaryExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	if x.Op == y.Op {
		if exprEqual(x.X, y.X) && exprEqual(x.Y, y.Y) {
			return true
		}
		// commutative operators
		switch x.Op {
		case token.ADD, token.MUL, token.AND, token.OR, token.XOR, token.LAND, token.LOR, token.EQL, token.NEQ:
			return exprEqual(x.X, y.Y) && exprEqual(x.Y, y.X)
		default:
		}
	}
	return false
}

func parenExprEqual(x, y *ast.ParenExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.X, y.X)
}

func starExprEqual(x, y *ast.StarExpr) bool {
	if x == nil || y == nil {
		return x == y
	}
	return exprEqual(x.X, y.X)
}

func exprSliceEqual(xs, ys []ast.Expr) bool {
	return slices.EqualFunc(xs, ys, exprEqual)
}

func fieldEqual(x, y *ast.Field) bool {
	if x == nil || y == nil {
		return x == y
	}
	return identSliceEqual(x.Names, y.Names) && exprEqual(x.Type, y.Type)
}

func fieldListEqual(x, y *ast.FieldList) bool {
	if x == nil || y == nil {
		return x == y
	}
	return fieldSliceEqual(x.List, y.List)
}

func identSliceEqual(xs, ys []*ast.Ident) bool {
	return slices.EqualFunc(xs, ys, identEqual)
}

func fieldSliceEqual(xs, ys []*ast.Field) bool {
	return slices.EqualFunc(xs, ys, fieldEqual)
}
