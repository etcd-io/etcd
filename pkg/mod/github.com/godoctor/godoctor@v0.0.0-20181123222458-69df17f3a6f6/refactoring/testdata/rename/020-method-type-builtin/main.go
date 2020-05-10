package main

import "fmt"


type simple string

// Test for renaming method in interface

func main() {

var simplevar simple = "hellooo"


simplevar.mymethod()		// <<<<< rename,15,12,15,12,renamed,pass


}

func (simplevar simple)mymethod() {


fmt.Println(simplevar)


}


