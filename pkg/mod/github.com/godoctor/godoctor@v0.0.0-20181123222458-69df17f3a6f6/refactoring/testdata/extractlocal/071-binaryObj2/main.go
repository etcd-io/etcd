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

import "fmt"

func main() {
	apple := 10
	orange := 3
	switch apple {
	case 1:
		fmt.Printf("this is a pizza")
		break
	case 3:
		fmt.Printf("this is an orange: %i", orange)
		break
	case 10:
		fmt.Printf("this is an apple: %i ", apple)
		fmt.Printf("this has a branchStmt")
		break //<<<<< var,18,3,18,8,newVar,fail
	}
}`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		panic(err)
	}
	x, y := false, false

	fmt.Println("hello world")
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncType:
			x = fieldListCheck(node.Params)
			y = fieldListCheck(node.Results)
		}
		if x || y {
			return false
		}
		return true
	})
}

func fieldListCheck(f *ast.FieldList) bool {
	if f == nil { //<<<<< var,55,5,55,5,newVar,pass
		return true
	}
	return false
}
