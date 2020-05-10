package main

import "fmt"
import "mypackage"
//Test for renaming an imported package
func main() {                               
	fmt.Println(mypackage.MyFunction(5))    
}
