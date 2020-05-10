package main

import "fmt"

type mystruct struct {

myvar string

}

// Test for renaming method

func main() {

mystructvar := mystruct {"helloo" } 

mystructvar.mymethod()		// <<<<< rename,17,13,17,13,continue,fail


}

func (mystructvar *mystruct)mymethod() {


fmt.Println(mystructvar.myvar)


}

