// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const bunPkgPath = "github.com/uptrace/bun"

// BunChecker checks github.com/uptrace/bun for SELECT * patterns.
type BunChecker struct{}

// NewBunChecker creates a new BunChecker.
func NewBunChecker() *BunChecker {
	return &BunChecker{}
}

// Name returns the name of this checker.
func (c *BunChecker) Name() string {
	return "bun"
}

// IsApplicable checks if the call is from bun using type information.
func (c *BunChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from bun package
	return IsTypeFromPackage(info, sel.X, bunPkgPath)
}

// CheckSelectStar checks for SELECT * in bun calls.
func (c *BunChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// Check ColumnExpr("*")
	if methodName == "ColumnExpr" || methodName == "Column" {
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				value := strings.Trim(lit.Value, "`\"")
				if value == "*" {
					return &SelectStarViolation{
						Pos:     call.Pos(),
						End:     call.End(),
						Message: "bun " + methodName + "(\"*\") - explicitly specify columns",
						Builder: "bun",
						Context: "explicit_star",
					}
				}
			}
		}
	}

	// Check NewRaw or Raw with SELECT *
	if methodName == "NewRaw" || methodName == "Raw" {
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				value := strings.Trim(lit.Value, "`\"")
				upperValue := strings.ToUpper(value)
				if strings.Contains(upperValue, "SELECT *") {
					return &SelectStarViolation{
						Pos:     call.Pos(),
						End:     call.End(),
						Message: "bun Raw() with SELECT * - specify columns explicitly",
						Builder: "bun",
						Context: "raw_select_star",
					}
				}
			}
		}
	}

	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
func (c *BunChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation

	// Track chain state
	hasNewSelect := false
	hasColumn := false
	var selectCall *ast.CallExpr

	// Traverse the call chain
	current := call
	for current != nil {
		sel, ok := current.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		switch sel.Sel.Name {
		case "NewSelect":
			hasNewSelect = true
			selectCall = current
		case "Column", "ColumnExpr":
			hasColumn = true
			// Check for "*" argument
			for _, arg := range current.Args {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					value := strings.Trim(lit.Value, "`\"")
					if value == "*" {
						violations = append(violations, &SelectStarViolation{
							Pos:     current.Pos(),
							End:     current.End(),
							Message: "bun Column(\"*\") in chain - specify columns explicitly",
							Builder: "bun",
							Context: "chained_star",
						})
					}
				}
			}
		case "Scan", "Exec":
			// Terminal methods - check if we have NewSelect without Column
			if hasNewSelect && !hasColumn && selectCall != nil {
				violations = append(violations, &SelectStarViolation{
					Pos:     selectCall.Pos(),
					End:     current.End(),
					Message: "bun NewSelect() with Scan/Exec without Column() defaults to SELECT *",
					Builder: "bun",
					Context: "implicit_star",
				})
			}
		}

		// Move to the next call in the chain
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			current = innerCall
		} else {
			break
		}
	}

	return violations
}
