package pkg

import (
	"go/ast"
	"go/token"
	"strconv"
)

func addIntExpr(x, y ast.Expr) ast.Expr {
	if x == nil || y == nil {
		return nil
	}

	xInt, xOK := intValue(x)
	yInt, yOK := intValue(y)

	if xOK && yOK {
		return intExpr(xInt + yInt)
	}
	if xOK {
		if xInt == 0 {
			return y
		}
		if xInt < 0 {
			return &ast.BinaryExpr{X: y, Op: token.SUB, Y: intExpr(-xInt)}
		}
	}
	if yOK {
		if yInt == 0 {
			return x
		}
		if yInt < 0 {
			return &ast.BinaryExpr{X: x, Op: token.SUB, Y: intExpr(-yInt)}
		}
	}
	if unary, ok := y.(*ast.UnaryExpr); ok && unary.Op == token.SUB {
		return &ast.BinaryExpr{X: x, Op: token.SUB, Y: unary.X}
	}
	return &ast.BinaryExpr{X: x, Op: token.ADD, Y: y}
}

func incIntExpr(x ast.Expr) ast.Expr {
	if x == nil {
		return nil
	}

	xInt, xOK := intValue(x)
	if xOK {
		return intExpr(xInt + 1)
	}
	if binary, ok := x.(*ast.BinaryExpr); ok && binary.Op == token.SUB {
		if yInt, yOK := intValue(binary.Y); yOK && yInt == 1 {
			return binary.X
		}
	}
	return &ast.BinaryExpr{X: x, Op: token.ADD, Y: intExpr(1)}
}

func subIntExpr(x, y ast.Expr) ast.Expr {
	if binary, ok := x.(*ast.BinaryExpr); ok && binary.Op == token.ADD {
		if exprEqual(binary.X, y) {
			return binary.Y
		}
		if exprEqual(binary.Y, y) {
			return binary.X
		}
	}

	if unary, ok := y.(*ast.UnaryExpr); ok && unary.Op == token.SUB {
		y = unary.X
	} else {
		y = &ast.UnaryExpr{Op: token.SUB, X: y}
	}
	return addIntExpr(x, y)
}

func mulIntExpr(x, y ast.Expr) ast.Expr {
	if x == nil || y == nil {
		return nil
	}

	xInt, xOK := intValue(x)
	yInt, yOK := intValue(y)

	if xOK && yOK {
		return intExpr(xInt * yInt)
	}
	if xOK {
		if xInt == 0 {
			return intExpr(0)
		}
		if xInt == 1 {
			return y
		}
	}
	if yOK {
		if yInt == 0 {
			return intExpr(0)
		}
		if yInt == 1 {
			return x
		}
	}
	return &ast.BinaryExpr{X: x, Op: token.MUL, Y: y}
}

func divIntExpr(x, y ast.Expr) (ast.Expr, bool) {
	if x == nil || y == nil {
		return nil, false
	}

	xInt, xOK := intValue(x)
	yInt, yOK := intValue(y)

	if xOK && yOK {
		return intExpr(xInt / yInt), xInt%yInt != 0
	}
	if yOK && yInt == 0 {
		return nil, false
	}
	if (xOK && xInt == 0) || (yOK && yInt == 1) {
		return x, false
	}
	return &ast.BinaryExpr{X: x, Op: token.QUO, Y: y}, true
}

func intExpr(n int) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(n)}
}

func intValue(expr ast.Expr) (int, bool) {
	var negate bool
	for {
		switch e := expr.(type) {
		case *ast.UnaryExpr:
			if e.Op == token.SUB {
				negate = !negate
				expr = e.X
				continue
			}
		case *ast.CallExpr:
			if ident, ok := e.Fun.(*ast.Ident); ok && len(e.Args) == 1 {
				switch ident.Name {
				case "byte", "rune", "int", "int8", "int16", "int32", "int64",
					"uint", "uint8", "uint16", "uint32", "uint64", "uintptr":
					expr = e.Args[0]
					continue
				}
			}
		}
		break
	}

	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.INT {
		if i, err := strconv.Atoi(lit.Value); err == nil {
			if negate {
				return -i, true
			}
			return i, true
		}
	}
	return 0, false
}
