package p

import "go/ast"

func test(decl *ast.FuncDecl) {
	// Error: cannot type switch on non-interface value decl (type *ast.FuncDecl)
	switch decl := decl.(type) {
	}
}
