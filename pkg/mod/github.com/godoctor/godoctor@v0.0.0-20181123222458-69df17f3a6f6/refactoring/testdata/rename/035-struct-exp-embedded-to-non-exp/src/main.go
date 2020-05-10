package main

import "fmt"
import "mypackage"

//Test for renaming an imported struct that is embedded in other struct

func main() {
	mystructvar := mypackage.Mystruct{"helloo"} // <<<<< rename,9,27,9,27,renamed,pass
	fmt.Println("value is", mystructvar.Mymethod())
}
