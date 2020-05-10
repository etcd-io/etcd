package main

import "fmt"

// Test for renaming interface

type mystruct struct {   

myvar string

}

type simple interface {  // <<<<< rename,13,6,13,6,renamed,pass

mymethod()

}



func main() {

mystructvar := mystruct {"helloo" }

mystructvar.mymethod()		


}

func (mystructvar *mystruct)mymethod() {


fmt.Println(mystructvar.myvar)


}

