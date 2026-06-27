// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/types"
)

const entPkgPath = "entgo.io/ent"

// EntChecker checks entgo.io/ent for SELECT * patterns.
type EntChecker struct{}

// NewEntChecker creates a new EntChecker.
func NewEntChecker() *EntChecker {
	return &EntChecker{}
}

// Name returns the name of this checker.
func (c *EntChecker) Name() string {
	return "ent"
}

// IsApplicable checks if the call is from ent using type information.
func (c *EntChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from ent package
	return IsTypeFromPackage(info, sel.X, entPkgPath)
}

// CheckSelectStar checks for SELECT * in ent calls.
func (c *EntChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	// ent typically doesn't have explicit SELECT * patterns
	// The implicit SELECT * happens when Query().All() is called without Select()
	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
func (c *EntChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation

	// Track chain state
	hasQuery := false
	hasSelect := false
	var queryCall *ast.CallExpr

	// Traverse the call chain
	current := call
	for current != nil {
		sel, ok := current.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		switch sel.Sel.Name {
		case "Query":
			hasQuery = true
			queryCall = current
		case "Select":
			hasSelect = true
		case "All", "Only", "OnlyX", "First", "FirstX":
			// Terminal methods - check if we have Query without Select
			if hasQuery && !hasSelect && queryCall != nil {
				violations = append(violations, &SelectStarViolation{
					Pos:     queryCall.Pos(),
					End:     current.End(),
					Message: "ent Query() with All/Only without Select() fetches all columns - use Select() to specify columns",
					Builder: "ent",
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
