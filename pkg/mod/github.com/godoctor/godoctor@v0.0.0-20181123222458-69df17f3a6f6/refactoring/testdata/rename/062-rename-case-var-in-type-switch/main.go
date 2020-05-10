package main

import "fmt"

// Test for renaming the type switch variable
func main() {
	var t interface{}
	t = bool(true)
	switch t.(type) {
	case bool:
		fmt.Printf("boolean %t\n", t)
	case int:
		fmt.Printf("integer %d\n", t) //<<<<<rename,13,30,13,30,renamed,pass
	case *bool:
		fmt.Printf("pointer to boolean %t\n", *t)
	case *int:
		fmt.Printf("pointer to integer %d\n", *t)
	default:
		fmt.Printf("unexpected type %T", t)
	}

}
