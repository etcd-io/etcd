// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package names provides functions to analyze the usage names (functions,
// types, variables, etc.) across multiple files.
package names

import (
	"go/token"
	"go/types"
)

// FindConflict determines if there already exists an identifier with the given
// newName such that the given ident cannot be renamed to name.  It returns
// one such conflicting declaration, if possible, and nil if there are none.
func FindConflict(obj types.Object, name string) types.Object {
	// XXX: The checks here are unnecessarily conservative.  This looks for
	// any declaration with the same name in any related scope; it does not
	// consider *references*, which really determine whether a conflict
	// exists.

	if obj == nil { // Probably package or switch variable
		return nil
	}

	// Check for conflicts in the current scope or any child scope
	if obj.Parent() != nil {
		if result := findConflictInChildScope(obj.Parent(), name); result != nil {
			return result
		}
	}

	// Check for conflicting methods on the receiver type, if applicable
	if isMethod(obj) {
		objfound, _, pointerindirections := types.LookupFieldOrMethod(
			methodReceiver(obj).Type(), true, obj.Pkg(), name)
		if isMethod(objfound) && pointerindirections {
			return objfound
		}
	}

	// Check for possible conflicts from a parent scope
	if _, obj := obj.Parent().LookupParent(name, token.NoPos); obj != nil {
		return obj
	}

	return nil
}

func findConflictInChildScope(scope *types.Scope, name string) types.Object {
	if obj := scope.Lookup(name); obj != nil {
		return obj
	}

	for i := 0; i < scope.NumChildren(); i++ {
		if obj := findConflictInChildScope(scope.Child(i), name); obj != nil {
			return obj
		}
	}
	return nil
}
