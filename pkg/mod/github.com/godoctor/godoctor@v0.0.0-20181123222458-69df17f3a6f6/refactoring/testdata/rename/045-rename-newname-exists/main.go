package main

import "fmt"

var largescope = ":-(" // This is a different hello

// Test for renaming the local variable hello
func main() {
	largescope = ":-)"  // Don't change this 

	var hello string = "Hello"	// <<<<< rename,11,6,11,6,largescope,fail
	var world string = "world"	
	hello = hello + ", " + world
	hello += "!"
	fmt.Println(hello)
}
