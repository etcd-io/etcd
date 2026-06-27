// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const pgxPkgPath = "github.com/jackc/pgx"

// PGXChecker checks github.com/jackc/pgx for SELECT * patterns.
type PGXChecker struct{}

// NewPGXChecker creates a new PGXChecker.
func NewPGXChecker() *PGXChecker {
	return &PGXChecker{}
}

// Name returns the name of this checker.
func (c *PGXChecker) Name() string {
	return "pgx"
}

// IsApplicable checks if the call is from pgx using type information.
func (c *PGXChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from pgx package
	return IsTypeFromPackage(info, sel.X, pgxPkgPath)
}

// CheckSelectStar checks for SELECT * in pgx calls.
func (c *PGXChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// pgx methods where the SQL query is typically the second argument (after context)
	// conn.Query(ctx, sql, args...), conn.QueryFunc(ctx, sql, args, func...)
	switch methodName {
	case "Query", "QueryRow", "Exec", "Prepare", "QueryFunc":
		// supported methods
	default:
		return nil
	}

	const queryArgIndex = 1
	if queryArgIndex >= len(call.Args) {
		return nil
	}

	arg := call.Args[queryArgIndex]
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		value := strings.Trim(lit.Value, "`\"")
		upperValue := strings.ToUpper(value)
		if strings.Contains(upperValue, "SELECT *") {
			return &SelectStarViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "pgx " + methodName + "() with SELECT * - specify columns explicitly",
				Builder: "pgx",
				Context: "raw_select_star",
			}
		}
	}

	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
func (c *PGXChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	// pgx doesn't have significant chaining patterns
	return nil
}
