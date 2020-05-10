package main

import "fmt"
import "mypackage"

//Test for renaming an imported method

func main() {

mystructvar := mypackage.Mystruct {"helloo" }

fmt.Println("value is",mystructvar.Mymethod())	// <<<<< rename,12,36,12,36,Renamed,pass


}




