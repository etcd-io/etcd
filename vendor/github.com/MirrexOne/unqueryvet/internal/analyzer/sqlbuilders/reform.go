package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const reformPkgPath = "gopkg.in/reform.v1"

// ReformChecker detects SELECT * patterns in gopkg.in/reform.v1 queries.
// https://github.com/go-reform/reform
type ReformChecker struct{}

// NewReformChecker creates a new reform checker.
func NewReformChecker() *ReformChecker {
	return &ReformChecker{}
}

// Name returns the name of the SQL builder.
func (c *ReformChecker) Name() string {
	return "reform"
}

// IsApplicable checks if the call is from reform using type information.
func (c *ReformChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from reform package
	return IsTypeFromPackage(info, sel.X, reformPkgPath)
}

// CheckSelectStar checks a single call expression for SELECT * usage.
func (c *ReformChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	v := c.checkCall(call)
	if v == nil {
		return nil
	}

	return &SelectStarViolation{
		Pos:     v.Pos,
		End:     v.End,
		Message: v.Message,
		Builder: "reform",
		Context: v.Method,
	}
}

// CheckChainedCalls analyzes method chains for SELECT * patterns.
func (c *ReformChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	// reform doesn't typically use method chaining for SELECT *
	return nil
}

// Check analyzes a file for reform SELECT * patterns.
func (c *ReformChecker) Check(pass *analysis.Pass, file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if v := c.checkCall(call); v != nil {
			pass.Report(analysis.Diagnostic{
				Pos:     v.Pos,
				End:     v.End,
				Message: v.Message,
			})
		}

		return true
	})
}

// ReformViolation represents a reform SELECT * violation.
type ReformViolation struct {
	Pos     token.Pos
	End     token.Pos
	Message string
	Method  string
}

// checkCall checks a call expression for reform SELECT * patterns.
func (c *ReformChecker) checkCall(call *ast.CallExpr) *ReformViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	method := sel.Sel.Name

	// reform patterns that load all columns:
	// - db.FindByPrimaryKeyFrom(table, pk, &record)
	// - db.FindOneFrom(table, column, value, &record)
	// - db.FindAllFrom(table, column, values...)
	// - db.SelectOneFrom(table, tail, args...) - if tail doesn't specify columns
	// - db.SelectAllFrom(table, tail, args...) - if tail doesn't specify columns
	// - querier.SelectRows(tail, args...) - may need column specification

	switch method {
	case "FindByPrimaryKeyFrom", "FindOneFrom", "FindAllFrom":
		// These methods always load all columns
		if c.isReformDB(sel.X) {
			return &ReformViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "reform: " + method + " loads all columns - consider using SelectAllFrom with specific columns",
				Method:  method,
			}
		}

	case "SelectOneFrom", "SelectAllFrom":
		// Check if the tail argument specifies columns
		if c.isReformDB(sel.X) && !c.hasColumnSpecification(call) {
			return &ReformViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "reform: " + method + " should specify columns in the tail argument",
				Method:  method,
			}
		}

	case "SelectRows":
		// Check querier.SelectRows
		if c.isQuerier(sel.X) && !c.hasSelectClause(call) {
			return &ReformViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "reform: SelectRows query may select all columns - verify column specification",
				Method:  method,
			}
		}
	}

	return nil
}

// isReformDB checks if the expression is a reform DB.
func (c *ReformChecker) isReformDB(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		name := ident.Name
		return name == "db" || name == "DB" || name == "querier" ||
			name == "tx" || name == "Tx"
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == "DB" || sel.Sel.Name == "Querier"
	}

	return false
}

// isQuerier checks if the expression is a reform Querier.
func (c *ReformChecker) isQuerier(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		name := ident.Name
		return name == "querier" || name == "q" || name == "db" || name == "tx"
	}

	return false
}

// hasColumnSpecification checks if the call specifies columns.
func (c *ReformChecker) hasColumnSpecification(call *ast.CallExpr) bool {
	// The tail argument (usually second) should specify columns
	// e.g., "WHERE id = ? ORDER BY name" doesn't specify columns
	// but "id, name WHERE id = ?" does

	// Find the tail argument (string)
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			tail := strings.Trim(lit.Value, "`\"")
			tailUpper := strings.ToUpper(tail)

			// Check if tail starts with column names (not WHERE, ORDER BY, etc.)
			if strings.HasPrefix(tailUpper, "WHERE") ||
				strings.HasPrefix(tailUpper, "ORDER") ||
				strings.HasPrefix(tailUpper, "LIMIT") ||
				strings.HasPrefix(tailUpper, "GROUP") ||
				strings.HasPrefix(tailUpper, "HAVING") ||
				tail == "" {
				return false
			}

			// If it doesn't start with common clauses, assume it has columns
			return true
		}
	}

	return false
}

// hasSelectClause checks if a SelectRows call has a proper SELECT clause.
func (c *ReformChecker) hasSelectClause(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			query := strings.ToUpper(strings.Trim(lit.Value, "`\""))

			// Check if query has SELECT with specific columns (not *)
			if strings.Contains(query, "SELECT") {
				if strings.Contains(query, "SELECT *") ||
					strings.Contains(query, "SELECT\t*") ||
					strings.Contains(query, "SELECT\n*") {
					return false
				}
				return true
			}
		}
	}

	return false
}

// CheckFile checks a file and returns violations.
func (c *ReformChecker) CheckFile(file *ast.File, fset *token.FileSet) []ReformViolation {
	var violations []ReformViolation

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if v := c.checkCall(call); v != nil {
			violations = append(violations, *v)
		}

		return true
	})

	return violations
}
