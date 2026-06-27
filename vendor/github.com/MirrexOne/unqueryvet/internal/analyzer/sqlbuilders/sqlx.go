// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const sqlxPkgPath = "github.com/jmoiron/sqlx"

// SQLxChecker checks github.com/jmoiron/sqlx for SELECT * patterns.
type SQLxChecker struct{}

// NewSQLxChecker creates a new SQLxChecker.
func NewSQLxChecker() *SQLxChecker {
	return &SQLxChecker{}
}

// Name returns the name of this checker.
func (c *SQLxChecker) Name() string {
	return "sqlx"
}

// IsApplicable checks if the call is from sqlx using type information.
func (c *SQLxChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from sqlx package
	return IsTypeFromPackage(info, sel.X, sqlxPkgPath)
}

// CheckSelectStar checks for SELECT * in sqlx calls.
func (c *SQLxChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// Methods where the SQL query is typically the first or second argument
	queryArgIndex := 0
	switch methodName {
	case "Select", "Get":
		// db.Select(&dest, query, args...)
		// db.Get(&dest, query, args...)
		queryArgIndex = 1
	case "Queryx", "QueryRowx", "MustExec", "NamedExec":
		// db.Queryx(query, args...)
		queryArgIndex = 0
	case "NamedQuery":
		// db.NamedQuery(query, arg)
		queryArgIndex = 0
	default:
		return nil
	}

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
				Message: "sqlx " + methodName + "() with SELECT * - specify columns explicitly",
				Builder: "sqlx",
				Context: "raw_select_star",
			}
		}
	}

	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
func (c *SQLxChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	// sqlx doesn't have significant chaining patterns like GORM
	// Most queries are single method calls
	return nil
}
