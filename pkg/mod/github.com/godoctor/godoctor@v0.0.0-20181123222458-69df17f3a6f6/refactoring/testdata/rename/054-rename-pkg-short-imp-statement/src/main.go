package main

import "fmt"
import  foo "mypackage" 
//Test for renaming an imported package short name
func main() {                               
	fmt.Println(foo.MyFunction(5))    
}
