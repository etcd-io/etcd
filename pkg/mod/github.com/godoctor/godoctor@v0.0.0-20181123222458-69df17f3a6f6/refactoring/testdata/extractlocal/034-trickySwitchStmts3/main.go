package main

import (
	"fmt"
	"go/ast"
)

func main() {
	x := 2
	y := 5
	var choice ast.Node
	fmt.Println("please choose: x + x, x * y?")
	switch choice.(type) {
	case *ast.AssignStmt: // <<<<< var,14,7,14,21,newVar,fail
		fmt.Println(x + x)
	case *ast.GenDecl:
		fmt.Println(x * y)
	default:
		fmt.Println("works but not a switch case")
	}
}
