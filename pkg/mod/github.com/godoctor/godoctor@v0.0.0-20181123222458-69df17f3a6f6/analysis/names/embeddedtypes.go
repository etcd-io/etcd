// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/loader"
)

// FindEmbeddedTypes finds each use of the given Object's type as an embedded
// type and returns a set consisting of the given Object and all uses in
// embedded types.
func FindEmbeddedTypes(obj types.Object, program *loader.Program) map[types.Object]bool {
	result := map[types.Object]bool{obj: true}
	pkgInfo := program.AllPackages[obj.Pkg()]
	for _, file := range pkgInfo.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			if s, ok := node.(*ast.StructType); ok {
				for _, field := range s.Fields.List {
					fieldObj := match(field, obj, pkgInfo)
					if fieldObj != nil {
						result[fieldObj] = true
					}
				}
			}
			return true
		})
	}
	return result
}

func match(field *ast.Field, obj types.Object, pkgInfo *loader.PackageInfo) types.Object {
	fieldType := pkgInfo.TypeOf(field.Type)
	if fieldType != obj.Type() {
		return nil
	}

	name := findName(field.Type)
	if name == nil {
		return nil
	}

	return pkgInfo.ObjectOf(name)
}

// findName finds the identifier that determines the implicit name of an
// embedded type
func findName(t ast.Expr) *ast.Ident {
	switch t := t.(type) {
	case *ast.Ident:
		return t
	case *ast.SelectorExpr:
		return t.Sel
	case *ast.StarExpr:
		return findName(t.X)
	}
	return nil
}
