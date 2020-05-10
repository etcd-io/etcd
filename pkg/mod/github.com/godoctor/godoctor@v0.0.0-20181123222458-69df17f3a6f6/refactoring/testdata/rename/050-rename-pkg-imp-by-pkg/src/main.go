package main

import "fmt"
import "mypackage"
import "secondpackage"
//Test for renaming a package imported by other package and main file
func main() {                               
	fmt.Println(mypackage.MyFunction(5))
        fmt.Println(secondpackage.Simplesquare(9))     
}
