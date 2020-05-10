package main

import "fmt"

type mystruct struct {

myvar string

}

type otherstruct struct  {

othervar string

}

// Test for renaming method

func main() {

mystructvar := mystruct {"helloo" }

mystructvar.mymethod()		// <<<<< rename,17,13,17,13,main,fail


}

func (mystructvar *mystruct)mymethod() {


fmt.Println(mystructvar.myvar)


}

