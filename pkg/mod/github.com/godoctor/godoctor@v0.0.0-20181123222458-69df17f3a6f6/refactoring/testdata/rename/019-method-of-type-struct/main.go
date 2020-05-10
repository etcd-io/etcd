package main

import "fmt"


type simple interface {

mymethod()

} 


type mystruct struct {

myvar string

}

// Test for renaming method in interface

func main() {

mystructvar := mystruct {"helloo" }

mystructvar.mymethod()		// <<<<< rename,25,13,25,13,renamed,pass


}

func (mystructvar mystruct)mymethod() {


fmt.Println(mystructvar.myvar)


}


