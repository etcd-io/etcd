package main

import "fmt"
import "mypackage"// <<<<< rename,4,10,4,10,Xyz,fail 
//Test for renaming an imported package
func main() {                               
	fmt.Println(mypackage.MyFunction(5))    
}
