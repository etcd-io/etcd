package main

import "fmt"

// hello

func main() {
	// hello
	var x interface{} = "hello"
	// hello
	switch hello := x.(type) { // <<<<<rename,11,9,11,9,xxx,pass
	// hello
	case int:
		// hello
		fmt.Println(hello)
		// hello
	case string:
		// hello
		fmt.Println(hello)
		// hello
	default:
		// hello
		fmt.Println(hello)
		// hello
	}
	// hello
}
