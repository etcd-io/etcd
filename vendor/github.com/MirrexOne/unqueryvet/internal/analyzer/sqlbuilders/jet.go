// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

const jetPkgPath = "github.com/go-jet/jet"

// JetChecker checks github.com/go-jet/jet for SELECT * patterns.
type JetChecker struct{}

// NewJetChecker creates a new JetChecker.
func NewJetChecker() *JetChecker {
	return &JetChecker{}
}

// Name returns the name of this checker.
func (c *JetChecker) Name() string {
	return "jet"
}

// IsApplicable checks if the call is from jet using type information.
func (c *JetChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		// Check for direct SELECT call - verify via type info
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if info != nil {
				if obj := info.Uses[ident]; obj != nil {
					if pkg := obj.Pkg(); pkg != nil {
						pkgPath := pkg.Path()
						if len(pkgPath) >= len(jetPkgPath) && pkgPath[:len(jetPkgPath)] == jetPkgPath {
							return true
						}
					}
				}
			}
		}
		return false
	}

	// Check if the receiver type is from jet package
	return IsTypeFromPackage(info, sel.X, jetPkgPath)
}

// CheckSelectStar checks for SELECT * in jet calls.
func (c *JetChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	// Check for direct function calls (SELECT, RawStatement)
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return c.checkIdentCall(call, ident)
	}

	// Check for selector calls (table.AllColumns, pkg.Star, etc.)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return c.checkSelectorCall(call, sel)
	}

	return nil
}

// checkIdentCall handles direct function calls like SELECT() or RawStatement()
func (c *JetChecker) checkIdentCall(call *ast.CallExpr, ident *ast.Ident) *SelectStarViolation {
	switch ident.Name {
	case "SELECT":
		if slices.ContainsFunc(call.Args, c.isAllColumnsOrStar) {
			return &SelectStarViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "Jet SELECT with AllColumns/STAR - specify columns explicitly",
				Builder: "jet",
				Context: "explicit_star",
			}
		}
	case "RawStatement":
		return c.checkRawStatementArgs(call, "Jet RawStatement with SELECT * - specify columns explicitly")
	}
	return nil
}

// checkSelectorCall handles selector calls like table.AllColumns or pkg.Star
func (c *JetChecker) checkSelectorCall(call *ast.CallExpr, sel *ast.SelectorExpr) *SelectStarViolation {
	methodName := sel.Sel.Name

	switch methodName {
	case "AllColumns":
		return &SelectStarViolation{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: "Jet AllColumns fetches all columns (SELECT *) - specify columns explicitly",
			Builder: "jet",
			Context: "all_columns",
		}
	case "Star":
		return &SelectStarViolation{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: "Jet Star() - avoid SELECT * and specify columns explicitly",
			Builder: "jet",
			Context: "explicit_star",
		}
	case "RawStatement", "Raw":
		return c.checkRawStatementArgs(call, "Jet Raw/RawStatement with SELECT * - specify columns explicitly")
	}

	return nil
}

// checkRawStatementArgs checks for SELECT * in raw statement arguments
func (c *JetChecker) checkRawStatementArgs(call *ast.CallExpr, message string) *SelectStarViolation {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			value := strings.Trim(lit.Value, "`\"")
			if strings.Contains(strings.ToUpper(value), "SELECT *") {
				return &SelectStarViolation{
					Pos:     call.Pos(),
					End:     call.End(),
					Message: message,
					Builder: "jet",
					Context: "raw_select_star",
				}
			}
		}
	}
	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
func (c *JetChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation

	// Traverse the call chain looking for SELECT with AllColumns
	current := call
	for current != nil {
		// Check arguments for AllColumns
		for _, arg := range current.Args {
			if c.isAllColumnsOrStar(arg) {
				violations = append(violations, &SelectStarViolation{
					Pos:     current.Pos(),
					End:     current.End(),
					Message: "Jet SELECT chain contains AllColumns/STAR - specify columns explicitly",
					Builder: "jet",
					Context: "chained_star",
				})
			}
		}

		// Move to the next call in the chain
		sel, ok := current.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			current = innerCall
		} else {
			break
		}
	}

	return violations
}

// isAllColumnsOrStar checks if an expression represents AllColumns or STAR.
func (c *JetChecker) isAllColumnsOrStar(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		// table.AllColumns or package.STAR
		return e.Sel.Name == "AllColumns" || e.Sel.Name == "STAR"
	case *ast.CallExpr:
		// Check if it's a call to AllColumns() or Star()
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			return sel.Sel.Name == "AllColumns" || sel.Sel.Name == "Star"
		}
	case *ast.Ident:
		// Direct STAR identifier
		return e.Name == "STAR" || e.Name == "AllColumns"
	}
	return false
}
