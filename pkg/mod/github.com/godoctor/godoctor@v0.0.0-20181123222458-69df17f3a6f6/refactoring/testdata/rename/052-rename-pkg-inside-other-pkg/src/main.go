package main

import "fmt"
import "mypackage/subpackage"
//Test for renaming an imported package
func main() {                               
	fmt.Println(subpackage.MyFunction(5))    
}
