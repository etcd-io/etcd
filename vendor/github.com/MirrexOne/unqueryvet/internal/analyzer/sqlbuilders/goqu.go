package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const goquPkgPath = "github.com/doug-martin/goqu"

// GoquChecker checks for SELECT * in goqu queries.
type GoquChecker struct{}

// NewGoquChecker creates a new goqu checker.
func NewGoquChecker() *GoquChecker {
	return &GoquChecker{}
}

// Name returns the checker name.
func (c *GoquChecker) Name() string {
	return "goqu"
}

// IsApplicable checks if the call is from goqu using type information.
func (c *GoquChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from goqu package
	if IsTypeFromPackage(info, sel.X, goquPkgPath) {
		return true
	}

	// Check for package-level function calls like goqu.From()
	if ident, ok := sel.X.(*ast.Ident); ok {
		if info != nil {
			if obj := info.Uses[ident]; obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok {
					pkgPath := pkgName.Imported().Path()
					if len(pkgPath) >= len(goquPkgPath) && pkgPath[:len(goquPkgPath)] == goquPkgPath {
						return true
					}
				}
			}
		}
	}

	return false
}

// CheckSelectStar checks for SELECT * patterns in goqu.
func (c *GoquChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// goqu.From("table").SelectAll() - this selects all columns
	if methodName == "SelectAll" {
		return &SelectStarViolation{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: "goqu SelectAll() selects all columns - use Select() with explicit column names",
		}
	}

	// goqu.From("table").Select("*")
	if methodName == "Select" {
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				value := strings.Trim(lit.Value, "`\"'")
				if value == "*" {
					return &SelectStarViolation{
						Pos:     lit.Pos(),
						End:     lit.End(),
						Message: "goqu Select(\"*\") - specify columns explicitly",
					}
				}
			}
		}

		// goqu.From("table").Select() without arguments also selects all
		if len(call.Args) == 0 {
			return &SelectStarViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "goqu Select() without arguments selects all columns",
			}
		}
	}

	return nil
}

// CheckChainedCalls checks chained method calls.
func (c *GoquChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation

	// Walk up the chain
	current := call
	for current != nil {
		if v := c.CheckSelectStar(current); v != nil {
			violations = append(violations, v)
		}

		// Move to the receiver if it's also a call
		sel, ok := current.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}
		current, _ = sel.X.(*ast.CallExpr)
	}

	return violations
}
