package main

import "fmt"

var hello = ":-(" // This is a different hello

// Test for renaming the local variable hello to a keyword
func main() {
	hello = ":-)"  // Don't change this

	var hello string = "Hello"	// <<<<< rename,11,6,11,6,continue,fail
	var world string = "world"	
	hello = hello + ", " + world
	hello += "!"
	fmt.Println(hello)
}
