package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const relPkgPath = "github.com/go-rel/rel"

// RelChecker detects SELECT * patterns in go-rel/rel queries.
// https://github.com/go-rel/rel
type RelChecker struct{}

// NewRelChecker creates a new rel checker.
func NewRelChecker() *RelChecker {
	return &RelChecker{}
}

// Name returns the name of the SQL builder.
func (c *RelChecker) Name() string {
	return "rel"
}

// IsApplicable checks if the call is from rel using type information.
func (c *RelChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from rel package
	return IsTypeFromPackage(info, sel.X, relPkgPath)
}

// CheckSelectStar checks a single call expression for SELECT * usage.
func (c *RelChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	if !c.isSelectAllPattern(call) {
		return nil
	}

	sel := call.Fun.(*ast.SelectorExpr)
	return &SelectStarViolation{
		Pos:     call.Pos(),
		End:     call.End(),
		Message: "rel: query loads all columns - consider using Select() to specify columns",
		Builder: "rel",
		Context: sel.Sel.Name,
	}
}

// CheckChainedCalls analyzes method chains for SELECT * patterns.
func (c *RelChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	// rel doesn't typically use method chaining for SELECT *
	return nil
}

// Check analyzes a file for rel SELECT * patterns.
func (c *RelChecker) Check(pass *analysis.Pass, file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for rel patterns that load all columns
		if c.isSelectAllPattern(call) {
			pass.Report(analysis.Diagnostic{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "rel: query loads all columns - consider using Select() to specify columns",
			})
		}

		return true
	})
}

// isSelectAllPattern checks if a call represents a SELECT * pattern in rel.
func (c *RelChecker) isSelectAllPattern(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	method := sel.Sel.Name

	// rel patterns that load all columns by default:
	// - repo.Find(ctx, &user) - loads all columns
	// - repo.FindAll(ctx, &users) - loads all columns
	// - repo.FindAndCountAll(ctx, &users) - loads all columns
	// - rel.From("users").All(ctx, &users) - loads all columns without Select()

	switch method {
	case "Find", "FindAll", "FindAndCountAll":
		// These methods load all columns unless combined with Select()
		// Check if this is a call on a rel repository
		if c.isRelRepository(sel.X) {
			// Check if the query chain includes Select()
			if !c.hasSelectInChain(call) {
				return true
			}
		}

	case "All", "One":
		// Check if this is part of a query builder chain without Select()
		if c.isQueryBuilderWithoutSelect(sel.X) {
			return true
		}
	}

	return false
}

// isRelRepository checks if the expression is a rel repository.
func (c *RelChecker) isRelRepository(expr ast.Expr) bool {
	// Check for common rel repository variable names
	if ident, ok := expr.(*ast.Ident); ok {
		name := ident.Name
		return name == "repo" || name == "repository" ||
			name == "r" || name == "db"
	}

	// Check for selector on repo
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return c.isRelRepository(sel.X)
	}

	return false
}

// hasSelectInChain checks if Select() is used in the query chain.
func (c *RelChecker) hasSelectInChain(call *ast.CallExpr) bool {
	// Walk up the chain looking for Select()
	current := call.Fun
	for {
		sel, ok := current.(*ast.SelectorExpr)
		if !ok {
			break
		}

		if sel.Sel.Name == "Select" {
			return true
		}

		// Check if the receiver is a call expression (method chain)
		if callExpr, ok := sel.X.(*ast.CallExpr); ok {
			if innerSel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if innerSel.Sel.Name == "Select" {
					return true
				}
			}
			current = callExpr.Fun
		} else {
			break
		}
	}

	return false
}

// isQueryBuilderWithoutSelect checks if an expression is a query builder without Select().
func (c *RelChecker) isQueryBuilderWithoutSelect(expr ast.Expr) bool {
	// Walk the call chain looking for From() without Select()
	hasFrom := false
	hasSelect := false

	current := expr
	for {
		call, ok := current.(*ast.CallExpr)
		if !ok {
			break
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		switch sel.Sel.Name {
		case "From":
			hasFrom = true
		case "Select":
			hasSelect = true
		}

		current = sel.X
	}

	return hasFrom && !hasSelect
}

// RelViolation represents a rel SELECT * violation.
type RelViolation struct {
	Pos     token.Pos
	End     token.Pos
	Message string
	Method  string
}

// CheckFile checks a file and returns violations.
func (c *RelChecker) CheckFile(file *ast.File, fset *token.FileSet) []RelViolation {
	var violations []RelViolation

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if c.isSelectAllPattern(call) {
			sel := call.Fun.(*ast.SelectorExpr)
			violations = append(violations, RelViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "rel: query loads all columns - consider using Select()",
				Method:  sel.Sel.Name,
			})
		}

		return true
	})

	return violations
}
