package main

import "fmt"
import "mypackage"

//Test for renaming an imported struct

func main() {

mystructvar := mypackage.Mystruct {"helloo" }   // <<<<< rename,10,26,10,26,Renamed,pass

fmt.Println("value is",mystructvar.Mymethod())	


}




