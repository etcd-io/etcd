//<<<<<extract,28,4,31,4,foo,fail
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	src := `
						package p
						const c = 1.0
						var X = f(3.14)*2 + c
						`
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, "src.go", src, 0)
	if err != nil {
		panic(err)
	}

	// Inspect the AST and print all identifiers and literals.
	ast.Inspect(f, func(n ast.Node) bool {
		var s string
		switch x := n.(type) {
		case *ast.BasicLit:
			if x.Value == "3.14" {
				s = x.Value
				fmt.Println("This is the value of s as a BasicLit", s)
			}
		case *ast.Ident:
			s = x.Name

		}

		return true
	})

}
