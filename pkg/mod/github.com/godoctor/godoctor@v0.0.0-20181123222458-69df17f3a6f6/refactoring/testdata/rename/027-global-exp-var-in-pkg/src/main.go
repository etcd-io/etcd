package main

import "fmt"
import "mypackage"

//Test for renaming an imported method

func main() {

mystructvar := mypackage.Mystruct {"helloo" }

fmt.Println("value is",mystructvar.Mymethod())	

fmt.Println(mypackage.Dumy)   // <<<<< rename,14,23,14,23,Renamed,pass

}




