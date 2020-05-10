package main

import "fmt"

type mystruct struct {   // <<<<< rename,5,6,5,6,renamed,pass

myvar string

}

// Test for renaming struct

func main() {

mystructvar := mystruct {"helloo" }

mystructvar.mymethod()		


}

func (mystructvar *mystruct)mymethod() {


fmt.Println(mystructvar.myvar)


}

