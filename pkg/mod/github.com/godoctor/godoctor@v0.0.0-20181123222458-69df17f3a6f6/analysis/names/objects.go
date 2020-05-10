// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/loader"
)

// FindOccurrences receives an Object and returns the set of all identifiers
// that refer to that Object.
//
// Note that this method cannot be used to find occurrences of package names or
// variables defined by type switch statements; those must be handled using
// different methods in this package.
func FindOccurrences(obj types.Object, prog *loader.Program) map[*ast.Ident]bool {
	decls := map[types.Object]bool{obj: true}
	if _, ok := obj.(*types.TypeName); ok {
		decls = FindEmbeddedTypes(obj, prog)
	} else if isMethod(obj) {
		decls = FindDeclarationsAcrossInterfaces(obj, prog)
	}

	result := make(map[*ast.Ident]bool)
	for pkgInfo := range packages(decls, prog) {
		for id, obj := range pkgInfo.Defs {
			if decls[obj] {
				result[id] = true
			}
		}
		for id, obj := range pkgInfo.Uses {
			if decls[obj] {
				result[id] = true
			}
		}
	}
	return result
}

// packages returns a set of PackageInfos that may reference the given
// Objects.  If at least one of the given declarations is exported, the method
// returns all the packages of this program; otherwise, it returns the
// package(s) containing the given declarations.
func packages(decls map[types.Object]bool, program *loader.Program) map[*loader.PackageInfo]bool {
	// XXX(review D7): If performance is a concern, you could return only
	// the packages in the reverse transitive closure of the package import
	// graph, rather than all the packages.

	result := make(map[*loader.PackageInfo]bool)
	for decl := range decls {
		if decl.Exported() {
			return allPackages(program)
		}
		pkgInfo := program.AllPackages[decl.Pkg()]
		result[pkgInfo] = true
	}
	return result
}

func allPackages(prog *loader.Program) map[*loader.PackageInfo]bool {
	result := map[*loader.PackageInfo]bool{}
	for _, pkgInfo := range prog.AllPackages {
		result[pkgInfo] = true
	}
	return result
}
