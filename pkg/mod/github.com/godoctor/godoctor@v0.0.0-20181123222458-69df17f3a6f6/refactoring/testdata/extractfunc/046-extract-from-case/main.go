//<<<<<extract,14,3,17,3,Foo,pass
package main

import "fmt"

func main() {
	var x interface{}
	x = 77.0
	switch i := x.(type) {
	case int:
		fmt.Println("INT", i)
	case float64:
		fmt.Println("FLOAT64", i)
		if i == 77.0 {
			a := i + 3
			fmt.Println("The value of a is ", a)
		}
	case string:
		fmt.Printf("STRING", i)
	default:
		fmt.Println("Unknown type. Sorry!")
	}
}
