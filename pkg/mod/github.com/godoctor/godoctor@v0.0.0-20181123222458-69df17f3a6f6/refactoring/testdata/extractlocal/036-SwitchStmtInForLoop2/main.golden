package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	src := `
package main

import (
	"fmt"
	"go/ast"
)

func main() {
	a := 1
	b := 2
	c := "t"
	d := "f"
	node * ast.Node
	if a + b {
		fmt.Println("this function steps through the tree")
	}
	if a > b {
		fmt.Println("this function steps through the tree")

	}
	if b > a {
		fmt.Println("this function steps through the tree")
	}
	if b+c > d {
		fmt.Println("this function steps through the tree")
	}
	if !c && d {
		fmt.Println("this function steps through the tree")
	}
	if apple, ok := node.(*ast.Node); ok {
		fmt.Println("this function steps through the tree")
	}
	if ast.IsExported(node.(*ast.Ident).Name) {
		fmt.Println("this function steps through the tree")	
	}
	if ast.IsExported(node.(*ast.Ident).Name) && b == nil && a > 0 {
		fmt.Println("this function steps through the tree")
	}
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	ast.Print(fset, f)
	for _, d := range f.Decls {
		switch decl := d.(type) { // <<<<< var,60,18,60,25,newVar,fail
		case *ast.FuncDecl:
			fmt.Println("funcDecl" + decl.Name.Name)
		case *ast.GenDecl: // types (including structs/interfaces)
			fmt.Println("gendecl")
		default:
			fmt.Println("works but not a switch case")
		}
	}
}
