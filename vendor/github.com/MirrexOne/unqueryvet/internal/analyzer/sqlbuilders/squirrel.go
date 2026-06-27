// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const squirrelPkgPath = "github.com/Masterminds/squirrel"

// SquirrelChecker checks github.com/Masterminds/squirrel for SELECT * patterns.
type SquirrelChecker struct{}

// NewSquirrelChecker creates a new SquirrelChecker.
func NewSquirrelChecker() *SquirrelChecker {
	return &SquirrelChecker{}
}

// Name returns the name of this checker.
func (c *SquirrelChecker) Name() string {
	return "squirrel"
}

// IsApplicable checks if the call is from Squirrel using type information.
func (c *SquirrelChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from squirrel package
	if IsTypeFromPackage(info, sel.X, squirrelPkgPath) {
		return true
	}

	// Check for package-level function calls like squirrel.Select()
	if ident, ok := sel.X.(*ast.Ident); ok {
		if info != nil {
			if obj := info.Uses[ident]; obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok {
					pkgPath := pkgName.Imported().Path()
					if len(pkgPath) >= len(squirrelPkgPath) && pkgPath[:len(squirrelPkgPath)] == squirrelPkgPath {
						return true
					}
				}
			}
		}
	}

	return false
}

// CheckSelectStar checks for SELECT * in Squirrel calls.
func (c *SquirrelChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	methodName := sel.Sel.Name

	// Check squirrel.Select("*") or builder.Select("*")
	if methodName == "Select" {
		// Empty Select() means SELECT *
		if len(call.Args) == 0 {
			return &SelectStarViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "Squirrel Select() without columns defaults to SELECT * - add specific columns",
				Builder: "squirrel",
				Context: "empty_select",
			}
		}

		// Check for "*" in arguments
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				value := strings.Trim(lit.Value, "`\"")
				if value == "*" || value == "" {
					return &SelectStarViolation{
						Pos:     call.Pos(),
						End:     call.End(),
						Message: "Squirrel Select(\"*\") - explicitly specify columns",
						Builder: "squirrel",
						Context: "explicit_star",
					}
				}
			}
		}
	}

	// Check Columns("*") or Column("*")
	if methodName == "Columns" || methodName == "Column" {
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				value := strings.Trim(lit.Value, "`\"")
				if value == "*" {
					return &SelectStarViolation{
						Pos:     call.Pos(),
						End:     call.End(),
						Message: "Squirrel Columns(\"*\") - explicitly specify columns",
						Builder: "squirrel",
						Context: "explicit_star",
					}
				}
			}
		}
	}

	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
// squirrelChainState tracks state while traversing call chain
type squirrelChainState struct {
	hasSelect  bool
	hasColumns bool
	selectCall *ast.CallExpr
}

func (c *SquirrelChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation
	state := &squirrelChainState{}

	current := call
	for current != nil {
		sel, ok := current.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		if v := c.processChainMethod(sel.Sel.Name, current, state); v != nil {
			violations = append(violations, v)
		}

		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			current = innerCall
		} else {
			break
		}
	}

	return violations
}

// processChainMethod processes a single method in the call chain
func (c *SquirrelChecker) processChainMethod(methodName string, current *ast.CallExpr, state *squirrelChainState) *SelectStarViolation {
	switch methodName {
	case "Select":
		return c.handleSelectMethod(current, state)
	case "Columns", "Column":
		return c.handleColumnsMethod(current, state)
	case "From", "Where", "Join", "LeftJoin", "RightJoin", "InnerJoin":
		return c.handleTerminalMethod(state)
	}
	return nil
}

// handleSelectMethod handles Select() calls in chain
func (c *SquirrelChecker) handleSelectMethod(current *ast.CallExpr, state *squirrelChainState) *SelectStarViolation {
	state.hasSelect = true
	state.selectCall = current

	if len(current.Args) == 0 {
		return nil
	}

	state.hasColumns = true
	if v := c.checkArgsForStar(current, "Squirrel Select(\"*\") in chain - specify columns explicitly"); v != nil {
		return v
	}
	return nil
}

// handleColumnsMethod handles Columns()/Column() calls in chain
func (c *SquirrelChecker) handleColumnsMethod(current *ast.CallExpr, state *squirrelChainState) *SelectStarViolation {
	state.hasColumns = true
	return c.checkArgsForStar(current, "Squirrel Columns(\"*\") in chain - specify columns explicitly")
}

// handleTerminalMethod handles terminal methods (From, Where, Join, etc.)
func (c *SquirrelChecker) handleTerminalMethod(state *squirrelChainState) *SelectStarViolation {
	if state.hasSelect && !state.hasColumns && state.selectCall != nil && len(state.selectCall.Args) == 0 {
		return &SelectStarViolation{
			Pos:     state.selectCall.Pos(),
			End:     state.selectCall.End(),
			Message: "Squirrel Select() without columns in chain defaults to SELECT *",
			Builder: "squirrel",
			Context: "empty_select_chain",
		}
	}
	return nil
}

// checkArgsForStar checks if any argument is "*"
func (c *SquirrelChecker) checkArgsForStar(call *ast.CallExpr, message string) *SelectStarViolation {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			value := strings.Trim(lit.Value, "`\"")
			if value == "*" {
				return &SelectStarViolation{
					Pos:     call.Pos(),
					End:     call.End(),
					Message: message,
					Builder: "squirrel",
					Context: "chained_star",
				}
			}
		}
	}
	return nil
}
