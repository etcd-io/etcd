package gochecksumtype

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// inexhaustiveError is returned from check for each occurrence of inexhaustive
// case analysis in a Go type switch statement.
type inexhaustiveError struct {
	Position token.Position
	Def      sumTypeDef
	Missing  []types.Object
}

func (e inexhaustiveError) Pos() token.Position { return e.Position }
func (e inexhaustiveError) Error() string {
	return fmt.Sprintf(
		"%s: exhaustiveness check failed for sum type %q (from %s): missing cases for %s",
		e.Pos(), e.Def.Decl.TypeName, e.Def.Decl.Pos, strings.Join(e.Names(), ", "))
}

// Names returns a sorted list of names corresponding to the missing variant
// cases.
func (e inexhaustiveError) Names() []string {
	list := make([]string, 0, len(e.Missing))
	for _, o := range e.Missing {
		list = append(list, o.Name())
	}
	sort.Strings(list)
	return list
}

// check does exhaustiveness checking for the given sum type definitions in the
// given package. Every instance of inexhaustive case analysis is returned.
func check(pkg *packages.Package, defs []sumTypeDef, config Config) []error {
	var errs []error
	for _, astfile := range pkg.Syntax {
		ast.Inspect(astfile, func(n ast.Node) bool {
			swtch, ok := n.(*ast.TypeSwitchStmt)
			if !ok {
				return true
			}
			if err := checkSwitch(pkg, defs, swtch, config); err != nil {
				errs = append(errs, err)
			}
			return true
		})
	}
	return errs
}

// checkSwitch performs an exhaustiveness check on the given type switch
// statement. If the type switch is used on a sum type and does not cover
// all variants of that sum type, then an error is returned indicating which
// variants were missed.
//
// Note that if the type switch contains a non-panicing default case, then
// exhaustiveness checks are disabled.
func checkSwitch(
	pkg *packages.Package,
	defs []sumTypeDef,
	swtch *ast.TypeSwitchStmt,
	config Config,
) error {
	def, missing := missingVariantsInSwitch(pkg, defs, swtch, config)
	if len(missing) > 0 {
		return inexhaustiveError{
			Position: pkg.Fset.Position(swtch.Pos()),
			Def:      *def,
			Missing:  missing,
		}
	}
	return nil
}

// missingVariantsInSwitch returns a list of missing variants corresponding to
// the given switch statement. The corresponding sum type definition is also
// returned. (If no sum type definition could be found, then no exhaustiveness
// checks are performed, and therefore, no missing variants are returned.)
func missingVariantsInSwitch(
	pkg *packages.Package,
	defs []sumTypeDef,
	swtch *ast.TypeSwitchStmt,
	config Config,
) (*sumTypeDef, []types.Object) {
	asserted := findTypeAssertExpr(swtch)
	ty := pkg.TypesInfo.TypeOf(asserted)
	if ty == nil {
		panic(fmt.Sprintf("no type found for asserted expression: %v", asserted))
	}

	def := findDef(defs, ty)
	if def == nil {
		// We couldn't find a corresponding sum type, so there's
		// nothing we can do to check it.
		return nil, nil
	}
	variantExprs, hasDefault := switchVariants(swtch)
	if config.DefaultSignifiesExhaustive && hasDefault && !defaultClauseAlwaysPanics(swtch) {
		// A catch-all case defeats all exhaustiveness checks.
		return def, nil
	}
	variantTypes := make([]types.Type, 0, len(variantExprs))
	for _, expr := range variantExprs {
		variantTypes = append(variantTypes, pkg.TypesInfo.TypeOf(expr))
	}
	return def, def.missing(variantTypes, config.IncludeSharedInterfaces)
}

// switchVariants returns all case expressions found in a type switch. This
// includes expressions from cases that have a list of expressions.
func switchVariants(swtch *ast.TypeSwitchStmt) (exprs []ast.Expr, hasDefault bool) {
	for _, stmt := range swtch.Body.List {
		clause := stmt.(*ast.CaseClause)
		if clause.List == nil {
			hasDefault = true
		} else {
			exprs = append(exprs, clause.List...)
		}
	}
	return
}

// defaultClauseAlwaysPanics returns true if the given switch statement has a
// default clause that always panics. Note that this is done on a best-effort
// basis. While there will never be any false positives, there may be false
// negatives.
//
// If the given switch statement has no default clause, then this function
// panics.
func defaultClauseAlwaysPanics(swtch *ast.TypeSwitchStmt) bool {
	var clause *ast.CaseClause
	for _, stmt := range swtch.Body.List {
		c := stmt.(*ast.CaseClause)
		if c.List == nil {
			clause = c
			break
		}
	}
	if clause == nil {
		panic("switch statement has no default clause")
	}
	if len(clause.Body) != 1 {
		return false
	}
	exprStmt, ok := clause.Body[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	callExpr, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	fun, ok := callExpr.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	return fun.Name == "panic"
}

// findTypeAssertExpr extracts the expression that is being type asserted from a
// type swtich statement.
func findTypeAssertExpr(swtch *ast.TypeSwitchStmt) ast.Expr {
	var expr ast.Expr
	if assign, ok := swtch.Assign.(*ast.AssignStmt); ok {
		expr = assign.Rhs[0]
	} else {
		expr = swtch.Assign.(*ast.ExprStmt).X
	}
	return expr.(*ast.TypeAssertExpr).X
}

// findDef returns the sum type definition corresponding to the given type. If
// no such sum type definition exists, then nil is returned.
func findDef(defs []sumTypeDef, needle types.Type) *sumTypeDef {
	for i := range defs {
		def := &defs[i]
		if types.Identical(needle.Underlying(), def.Ty) {
			return def
		}
	}
	return nil
}
