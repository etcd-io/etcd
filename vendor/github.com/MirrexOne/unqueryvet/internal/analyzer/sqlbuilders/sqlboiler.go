// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const sqlboilerPkgPath = "github.com/volatiletech/sqlboiler"

// SQLBoilerChecker checks github.com/volatiletech/sqlboiler for SELECT * patterns.
type SQLBoilerChecker struct{}

// NewSQLBoilerChecker creates a new SQLBoilerChecker.
func NewSQLBoilerChecker() *SQLBoilerChecker {
	return &SQLBoilerChecker{}
}

// Name returns the name of this checker.
func (c *SQLBoilerChecker) Name() string {
	return "sqlboiler"
}

// IsApplicable checks if the call is from sqlboiler using type information.
func (c *SQLBoilerChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from sqlboiler package
	if IsTypeFromPackage(info, sel.X, sqlboilerPkgPath) {
		return true
	}

	// Check for qm (query mods) package - verify via type info
	if ident, ok := sel.X.(*ast.Ident); ok {
		if info != nil {
			if obj := info.Uses[ident]; obj != nil {
				// For package-level function calls like qm.Select(), obj is *types.PkgName
				if pkgName, ok := obj.(*types.PkgName); ok {
					pkgPath := pkgName.Imported().Path()
					if len(pkgPath) >= len(sqlboilerPkgPath) && pkgPath[:len(sqlboilerPkgPath)] == sqlboilerPkgPath {
						return true
					}
				}
			}
		}
	}

	return false
}

// CheckSelectStar checks for SELECT * in sqlboiler calls.
func (c *SQLBoilerChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// Check qm.Select("*")
	if methodName == "Select" {
		// Check if caller is qm package
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "qm" {
			for _, arg := range call.Args {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					value := strings.Trim(lit.Value, "`\"")
					if value == "*" {
						return &SelectStarViolation{
							Pos:     call.Pos(),
							End:     call.End(),
							Message: "SQLBoiler qm.Select(\"*\") - explicitly specify columns",
							Builder: "sqlboiler",
							Context: "explicit_star",
						}
					}
				}
			}
		}
	}

	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
func (c *SQLBoilerChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation

	// SQLBoiler uses query mods passed to model methods
	// Example: models.Users(qm.Select("*")).All(ctx, db)

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return violations
	}

	// Check terminal methods
	if sel.Sel.Name == "All" || sel.Sel.Name == "One" {
		// Look for the model call that might have query mods
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			// Check if there's a qm.Select in the arguments
			// Note: qm.Select("*") is already detected by CheckSelectStar when
			// the analyzer visits that CallExpr, so we only check for hasSelect here
			hasSelect := false
			for _, arg := range innerCall.Args {
				if callExpr, ok := arg.(*ast.CallExpr); ok {
					if innerSel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
						if innerSel.Sel.Name == "Select" {
							hasSelect = true
							break
						}
					}
				}
			}

			// If no Select query mod, it defaults to SELECT *
			if !hasSelect && len(innerCall.Args) == 0 {
				// Model().All() without query mods = SELECT *
				violations = append(violations, &SelectStarViolation{
					Pos:     innerCall.Pos(),
					End:     call.End(),
					Message: "SQLBoiler model().All() without qm.Select() defaults to SELECT *",
					Builder: "sqlboiler",
					Context: "implicit_star",
				})
			}
		}
	}

	return violations
}
