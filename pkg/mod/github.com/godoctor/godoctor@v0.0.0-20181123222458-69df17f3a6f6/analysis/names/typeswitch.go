// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/loader"
)

// FindTypeSwitchVarOccurrences returns the set of all identifiers that refer
// to the variable defined in the type switch statement.
func FindTypeSwitchVarOccurrences(typeSwitch *ast.TypeSwitchStmt, pkgInfo *loader.PackageInfo, program *loader.Program) map[*ast.Ident]bool {
	result := make(map[*ast.Ident]bool)

	// Add v from "switch v := e.(type)"
	if asgt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
		if id, ok := asgt.Lhs[0].(*ast.Ident); ok {
			result[id] = true
			if obj := pkgInfo.ObjectOf(id); obj != nil {
				result = FindOccurrences(obj, program)
			}
		}
	}

	// Find references to the implicit *types.Var for each case clause
	caseVars := caseClauseVars(typeSwitch, pkgInfo)
	ast.Inspect(typeSwitch.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := pkgInfo.ObjectOf(id).(*types.Var); ok {
				if caseVars[v] {
					result[id] = true
				}
			}
		}
		return true
	})

	return result
}

// result returns the set of implicit *types.Vars defined by the case clauses
// of the given TypeSwitchStmt.
func caseClauseVars(typeSwitch *ast.TypeSwitchStmt, pkgInfo *loader.PackageInfo) map[*types.Var]bool {
	result := map[*types.Var]bool{}
	for _, stmt := range typeSwitch.Body.List {
		obj := pkgInfo.Implicits[stmt.(*ast.CaseClause)].(*types.Var)
		result[obj] = true
	}
	return result
}
