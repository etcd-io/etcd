package analysisutil

import (
	"go/ast"
	"go/types"
)

// ObjectOf works in context of Golang package and returns types.Object for the given object's package and name.
// The search is based on the provided package and its dependencies (imports).
// Returns nil if the object is not found.
func ObjectOf(pkg *types.Package, objPkg, objName string) types.Object {
	if pkg.Path() == objPkg {
		return pkg.Scope().Lookup(objName)
	}

	for _, i := range pkg.Imports() {
		if trimVendor(i.Path()) == objPkg {
			return i.Scope().Lookup(objName)
		}
	}
	return nil
}

// IsObj returns true if expression is identifier which notes to given types.Object.
// Useful in combination with types.Universe objects.
func IsObj(typesInfo *types.Info, expr ast.Expr, expected types.Object) bool {
	id, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}

	obj := typesInfo.ObjectOf(id)
	return obj == expected
}
