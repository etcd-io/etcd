package main

import "fmt"
import "mypackage"

func main() { // <<<<< null,1,1,1,1,false,pass
	mystructvar := mypackage.Mystruct{"hello"}
	fmt.Println("value is", mystructvar.Mymethod())
}
