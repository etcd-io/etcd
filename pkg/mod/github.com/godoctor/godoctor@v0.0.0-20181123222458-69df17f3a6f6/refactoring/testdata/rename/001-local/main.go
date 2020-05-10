package main

import "fmt"

var hello = ":-(" // This is a different hello

// Test for renaming the local variable hello
func main() {
	hello = ":-)"  // Don't change this

	var hello string = "Hello"	// <<<<< rename,11,6,11,6,renamed,pass
	var world string = "world"	
	hello = hello + ", " + world
	hello += "!"
	fmt.Println(hello)
}
