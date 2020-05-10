// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dataflow provides data flow analyses that can be performed on a
// previously constructed control flow graph, including a reaching definitions
// analysis and a live variables analysis for local variables.
package dataflow

// This file contains functions common to all data flow analyses, as well as
// one exported function.

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/loader"
)

// ReferencedVars returns the sets of local variables that are defined or used
// within the given list of statements (based on syntax).
func ReferencedVars(stmts []ast.Stmt, info *loader.PackageInfo) (asgt, updt, decl, use map[*types.Var]struct{}) {
	asgt = make(map[*types.Var]struct{})
	updt = make(map[*types.Var]struct{})
	decl = make(map[*types.Var]struct{})
	use = make(map[*types.Var]struct{})
	for _, stmt := range stmts {
		for _, d := range assignments(stmt, info, false) {
			asgt[d] = struct{}{}
		}
		for _, d := range assignments(stmt, info, true) {
			updt[d] = struct{}{}
		}
		for _, d := range decls(stmt, info) {
			decl[d] = struct{}{}
		}
		for _, u := range uses(stmt, info) {
			use[u] = struct{}{}

		}
	}
	return asgt, updt, decl, use
}

// defs returns the set of variables that are assigned a value in the given
// statement, either by being assigned or declared
func defs(stmt ast.Stmt, info *loader.PackageInfo) []*types.Var {
	union := make(map[*types.Var]struct{})
	for _, v := range assignments(stmt, info, true) {
		union[v] = struct{}{}
	}
	for _, v := range assignments(stmt, info, false) {
		union[v] = struct{}{}
	}
	for _, v := range decls(stmt, info) {
		union[v] = struct{}{}
	}

	result := []*types.Var{}
	for v := range union {
		result = append(result, v)
	}
	return result
}

// assignments extracts any local variables whose values are assigned in the given statement.
func assignments(stmt ast.Stmt, info *loader.PackageInfo, wantMembers bool) []*types.Var {
	idnts := make(map[*ast.Ident]struct{})

	switch stmt := stmt.(type) {
	case *ast.AssignStmt: // =, &=, etc. except x[i] (IndexExpr)
		if stmt.Tok != token.DEFINE {
			for _, x := range stmt.Lhs {
				switch x.(type) {
				case *ast.IndexExpr, *ast.SelectorExpr, *ast.StarExpr:
					if wantMembers {
						idnts = union(idnts, assigned(x))
					}
				default:
					idnts = union(idnts, idents(x))
				}
			}
		}
	case *ast.IncDecStmt: // i++, i--
		switch x := stmt.X.(type) {
		case *ast.IndexExpr, *ast.SelectorExpr, *ast.StarExpr:
			if wantMembers {
				idnts = union(idnts, assigned(x))
			}
		default:
			idnts = union(idnts, idents(x))
		}
	}

	return collectVars(idnts, info)
}

// idents returns the set of identifiers assigned in a given node.
func assigned(node ast.Expr) map[*ast.Ident]struct{} {
	switch expr := node.(type) {
	case *ast.IndexExpr:
		return assigned(expr.X)
	case *ast.SelectorExpr:
		field := map[*ast.Ident]struct{}{expr.Sel: {}}
		return union(assigned(expr.X), field)
	case *ast.StarExpr:
		return map[*ast.Ident]struct{}{}
	default:
		return idents(node)
	}
}

// decls extracts any local variables that are declared (and implicitly or
// explicitly assigned a value) in the given statement.
func decls(stmt ast.Stmt, info *loader.PackageInfo) []*types.Var {
	idnts := make(map[*ast.Ident]struct{})

	switch stmt := stmt.(type) {
	case *ast.DeclStmt: // vars (1+) in decl; zero values
		ast.Inspect(stmt, func(n ast.Node) bool {
			if v, ok := n.(*ast.ValueSpec); ok {
				idnts = union(idnts, idents(v))
			}
			return true
		})
	case *ast.AssignStmt: // := except x[i] (IndexExpr)
		if stmt.Tok == token.DEFINE {
			for _, x := range stmt.Lhs {
				indExp := false
				ast.Inspect(x, func(n ast.Node) bool {
					if _, ok := n.(*ast.IndexExpr); ok {
						indExp = true
						return false
					}
					return true
				})
				if !indExp {
					idnts = union(idnts, idents(x))
				}
			}
		}
	case *ast.RangeStmt: // only [ x, y ] on Lhs
		idnts = union(idents(stmt.Key), idents(stmt.Value))
	case *ast.TypeSwitchStmt:
		// The assigned variable does not have a types.Var
		// associated in this stmt; rather, the uses of that
		// variable in the case clauses have several different
		// types.Vars associated with them, according to type
		var vars []*types.Var
		ast.Inspect(stmt.Body, func(n ast.Node) bool {
			switch cc := n.(type) {
			case *ast.CaseClause:
				v := typeCaseVar(info, cc)
				if v != nil {
					vars = append(vars, v)
				}
				return false
			default:
				return true
			}
		})
		return vars
	}

	return collectVars(idnts, info)
}

func collectVars(idnts map[*ast.Ident]struct{}, info *loader.PackageInfo) []*types.Var {
	var vars []*types.Var
	// should all map to types.Var's, if not we don't want anyway
	for i := range idnts {
		if v, ok := info.ObjectOf(i).(*types.Var); ok {
			if v.Pkg() == info.Pkg && !v.IsField() {
				vars = append(vars, v)
			}
		}
	}
	return vars
}

// typeCaseVar returns the implicit variable associated with a case clause in a
// type switch statement.
func typeCaseVar(info *loader.PackageInfo, cc *ast.CaseClause) *types.Var {
	// Removed from go/loader
	if v := info.Implicits[cc]; v != nil {
		return v.(*types.Var)
	}
	return nil
}

// uses extracts local variables whose values are used in the given statement.
func uses(stmt ast.Stmt, info *loader.PackageInfo) []*types.Var {
	idnts := make(map[*ast.Ident]struct{})

	ast.Inspect(stmt, func(n ast.Node) bool {
		switch stmt := stmt.(type) {
		case *ast.AssignStmt: // mostly rhs of =, :=, &=, etc.
			// some LHS are uses, e.g. x[i]
			for _, x := range stmt.Lhs {
				indExp := false
				switch T := x.(type) {
				case *ast.IndexExpr:
					indExp = true
				case *ast.SelectorExpr:
					idnts = union(idnts, idents(T))
				}
				if indExp || // x[i] is a uses of x and i
					(stmt.Tok != token.ASSIGN &&
						stmt.Tok != token.DEFINE) { // e.g. +=, ^=, etc.
					idnts = union(idnts, idents(x))
				}
			}
			// all RHS are uses
			for _, s := range stmt.Rhs {
				idnts = union(idnts, idents(s))
			}
		case *ast.BlockStmt: // no uses, skip - should not appear in cfg
		case *ast.BranchStmt: // no uses, skip
		case *ast.CaseClause:
			for _, i := range stmt.List {
				idnts = union(idnts, idents(i))
			}
		case *ast.CommClause: // no uses, skip
		case *ast.DeclStmt: // no uses, skip
		case *ast.DeferStmt:
			idnts = union(idnts, idents(stmt.Call))
		case *ast.ForStmt:
			idnts = union(idnts, idents(stmt.Cond))
		case *ast.IfStmt:
			idnts = union(idnts, idents(stmt.Cond))
		case *ast.LabeledStmt: // no uses, skip
		case *ast.RangeStmt: // list in _, _ = range [ list ]
			idnts = union(idnts, idents(stmt.X))
		case *ast.SelectStmt: // no uses, skip
		case *ast.SwitchStmt:
			idnts = union(idnts, idents(stmt.Tag))
		case *ast.TypeSwitchStmt:
			idnts = union(idnts, idents(stmt.Assign))
		case ast.Stmt: // everything else is all uses
			idnts = union(idnts, idents(stmt))

		}
		return true
	})

	return collectVars(idnts, info)
}

// idents returns the set of all identifiers in given node.
func idents(node ast.Node) map[*ast.Ident]struct{} {
	idents := make(map[*ast.Ident]struct{})
	if node == nil {
		return idents
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			idents[n] = struct{}{}
		}
		return true
	})
	return idents
}

func union(one, two map[*ast.Ident]struct{}) map[*ast.Ident]struct{} {
	for o := range one {
		two[o] = struct{}{}
	}
	return two
}

// Vars returns the set of variables appearing in node
func Vars(node ast.Node, info *loader.PackageInfo) map[*types.Var]struct{} {
	result := map[*types.Var]struct{}{}
	for _, variable := range collectVars(idents(node), info) {
		result[variable] = struct{}{}
	}
	return result
}
