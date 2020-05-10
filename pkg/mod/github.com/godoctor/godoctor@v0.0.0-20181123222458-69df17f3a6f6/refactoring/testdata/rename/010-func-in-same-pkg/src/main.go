package main

import "fmt"
import "mypackage"
//Test for renaming an imported function
func main() {                               
	fmt.Println(mypackage.MyFunction(5))     
}
