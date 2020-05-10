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
func main() {
	var arr1 [5]int 		//first way to declare an array
	arr2 := []int {10, 20, 30}	//second way to declare an array
	fmt.Println(arr1)		//print the array
	fmt.Println(arr2)	
	fmt.Println(len(arr1))		//print the length of the array, also len(array) gets the length of the array
}
type Apple struct {
}

`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		panic(err)
	}

	for i, d := range f.Decls {
		if i > 0 {

		}
		if decl, ok := d.(*ast.GenDecl); ok {
			for j, spec := range decl.Specs {
				if spec, ok := spec.(*ast.TypeSpec); ok {
					if ast.IsExported(spec.Name.Name) && spec.Doc == nil && j > 0 { // <<<<< var,37,24,37,37,newVar,pass
						fmt.Println("works")
					}
				}
			}
		}
	}
}
