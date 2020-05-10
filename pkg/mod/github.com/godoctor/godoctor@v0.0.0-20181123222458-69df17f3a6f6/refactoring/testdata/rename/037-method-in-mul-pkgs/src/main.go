package main

import(
"fmt"
"mypackage"
"secondpackage"
)

//Test for renaming an imported method,imported by multiple packages 

func main() {

mystructvar := mypackage.Mystruct {"helloo" }

fmt.Println("value is",mystructvar.Mymethod())	// <<<<< rename,15,36,15,36,Renamed,pass

secondstructvar :=secondpackage.Secondstruct {"howareyou"}

fmt.Println("second value is", secondstructvar.Secondmethod())

}




