package main

import "fmt"
import "mypackage"

//Test for renaming an imported method

func main() {

	mystructvar := mypackage.Mystruct{"helloo"}

	fmt.Println("value is", mystructvar.Mymethod())

	fmt.Println(mypackage.Dummy) // <<<<< rename,14,24,14,24,renamed,pass
}
