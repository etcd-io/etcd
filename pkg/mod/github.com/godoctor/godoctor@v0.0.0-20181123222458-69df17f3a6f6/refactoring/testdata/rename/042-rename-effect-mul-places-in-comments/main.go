package main

import "fmt"

var hello = ":-(" // This is a different hello

func main() {
	hello = ":-)" // Don't change this

	var hello string = "Hello" // <<<<< rename,10,6,10,6,renamed,pass
	var world string = "world"
	hello = hello + ", " + world
	hello += "!"
	// Test for renaming the local variable hello, only hello variable should be changed to renamed
	fmt.Println(hello)
}
