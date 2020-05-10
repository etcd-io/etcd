package main

import "fmt"

// hello

func main() {
	// hello
	hello := "hello"
	// hello
	x := func(hello string) { // <<<<<rename,11,12,11,12,xxx,pass
		// hello
		fmt.Println(hello)
		// hello
	}
	// hello
	x(hello)
}
