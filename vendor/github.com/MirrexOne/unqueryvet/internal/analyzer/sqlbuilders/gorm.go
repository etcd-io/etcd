// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

const gormPkgPath = "gorm.io/gorm"

// GORMChecker checks gorm.io/gorm for SELECT * patterns.
type GORMChecker struct{}

// NewGORMChecker creates a new GORMChecker.
func NewGORMChecker() *GORMChecker {
	return &GORMChecker{}
}

// Name returns the name of this checker.
func (c *GORMChecker) Name() string {
	return "gorm"
}

// IsApplicable checks if the call is from GORM using type information.
func (c *GORMChecker) IsApplicable(info *types.Info, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the receiver type is from gorm package
	return IsTypeFromPackage(info, sel.X, gormPkgPath)
}

// CheckSelectStar checks for SELECT * in GORM calls.
func (c *GORMChecker) CheckSelectStar(call *ast.CallExpr) *SelectStarViolation {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	switch sel.Sel.Name {
	case "Select":
		return c.checkSelectMethod(call)
	case "Raw":
		return c.checkRawMethod(call, "GORM Raw() with SELECT * - specify columns explicitly")
	case "Exec":
		return c.checkRawMethod(call, "GORM Exec() with SELECT * - specify columns explicitly")
	}

	return nil
}

// checkSelectMethod checks db.Select() for star patterns
func (c *GORMChecker) checkSelectMethod(call *ast.CallExpr) *SelectStarViolation {
	for _, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}

		value := strings.Trim(lit.Value, "`\"")
		if value == "*" {
			return &SelectStarViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "GORM Select(\"*\") - explicitly specify columns",
				Builder: "gorm",
				Context: "explicit_star",
			}
		}

		if strings.Contains(strings.ToUpper(value), "SELECT *") {
			return &SelectStarViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: "GORM Select() contains SELECT * - specify columns explicitly",
				Builder: "gorm",
				Context: "raw_select_star",
			}
		}
	}
	return nil
}

// checkRawMethod checks db.Raw() or db.Exec() for SELECT * patterns
func (c *GORMChecker) checkRawMethod(call *ast.CallExpr, message string) *SelectStarViolation {
	for _, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}

		value := strings.Trim(lit.Value, "`\"")
		if strings.Contains(strings.ToUpper(value), "SELECT *") {
			return &SelectStarViolation{
				Pos:     call.Pos(),
				End:     call.End(),
				Message: message,
				Builder: "gorm",
				Context: "raw_select_star",
			}
		}
	}
	return nil
}

// CheckChainedCalls checks method chains for SELECT * patterns.
// gormChainState tracks state while traversing GORM call chain
type gormChainState struct {
	hasModel  bool
	hasSelect bool
	modelCall *ast.CallExpr
}

func (c *GORMChecker) CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation
	state := &gormChainState{}

	current := call
	for current != nil {
		sel, ok := current.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}

		if v := c.processGormChainMethod(sel.Sel.Name, current, state); v != nil {
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

// processGormChainMethod processes a single method in the GORM call chain
func (c *GORMChecker) processGormChainMethod(methodName string, current *ast.CallExpr, state *gormChainState) *SelectStarViolation {
	switch methodName {
	case "Model", "Table":
		state.hasModel = true
		state.modelCall = current
	case "Select":
		state.hasSelect = true
		return c.checkGormSelectArgs(current)
	case "Find", "First", "Last", "Take", "Scan":
		return c.checkGormTerminalMethod(current, state)
	}
	return nil
}

// checkGormSelectArgs checks Select() arguments for "*"
func (c *GORMChecker) checkGormSelectArgs(current *ast.CallExpr) *SelectStarViolation {
	for _, arg := range current.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if strings.Trim(lit.Value, "`\"") == "*" {
			return &SelectStarViolation{
				Pos:     current.Pos(),
				End:     current.End(),
				Message: "GORM Select(\"*\") in chain - specify columns explicitly",
				Builder: "gorm",
				Context: "chained_star",
			}
		}
	}
	return nil
}

// checkGormTerminalMethod checks terminal methods (Find, First, etc.) for implicit SELECT *
func (c *GORMChecker) checkGormTerminalMethod(current *ast.CallExpr, state *gormChainState) *SelectStarViolation {
	if state.hasModel && !state.hasSelect && state.modelCall != nil {
		return &SelectStarViolation{
			Pos:     state.modelCall.Pos(),
			End:     current.End(),
			Message: "GORM Model() with Find/First without Select() defaults to SELECT *",
			Builder: "gorm",
			Context: "implicit_star",
		}
	}
	return nil
}
