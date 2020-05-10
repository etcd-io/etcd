package main

import "fmt"
import "mypackage"
//Test for renaming an imported function
func main() {                               
	fmt.Println(mypackage.MyFunction(5))    // <<<<< rename,7,24,7,24,Xyz,fail 
}
