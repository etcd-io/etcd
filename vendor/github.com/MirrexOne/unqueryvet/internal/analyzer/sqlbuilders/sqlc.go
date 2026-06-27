package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

// sqlc generates code, so we check if the package path contains "sqlc"
const sqlcPkgPath = "sqlc"

// SQLCChecker checks for SELECT * in sqlc generated code.
type SQLCChecker struct{}

// NewSQLCChecker creates a new sqlc checker.
func NewSQLCChecker() *SQLCChecker {
	return &SQLCChecker{}
}

// Name returns the checker name.
func (c *SQLCChecker) Name() string {
	return "sqlc"
}

// IsApplicable checks if the call is from sqlc generated code using type information.
func (c *SQLCChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// sqlc generates code, check if receiver type's package contains "sqlc"
	if info != nil {
		typ := info.TypeOf(sel.X)
		if typ != nil {
			if named, ok := typ.(*types.Named); ok {
				if obj := named.Obj(); obj != nil {
					if pkg := obj.Pkg(); pkg != nil {
						if strings.Contains(pkg.Path(), sqlcPkgPath) {
							return true
						}
					}
				}
			}
			// Check pointer types
			if ptr, ok := typ.(*types.Pointer); ok {
				if named, ok := ptr.Elem().(*types.Named); ok {
					if obj := named.Obj(); obj != nil {
						if pkg := obj.Pkg(); pkg != nil {
							if strings.Contains(pkg.Path(), sqlcPkgPath) {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

// CheckSelectStar checks for SELECT * in the call.
func (c *SQLCChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	// sqlc doesn't typically have SELECT * visible in Go code
	// but we can check string arguments
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			value := strings.ToUpper(lit.Value)
			if strings.Contains(value, "SELECT *") || strings.Contains(value, "SELECT\t*") {
				return &SelectStarViolation{
					Pos:     lit.Pos(),
					End:     lit.End(),
					Message: "sqlc query contains SELECT * - specify columns explicitly in your .sql file",
				}
			}
		}
	}
	return nil
}

// CheckChainedCalls checks chained method calls.
func (c *SQLCChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	// sqlc doesn't typically use chained calls
	return nil
}
