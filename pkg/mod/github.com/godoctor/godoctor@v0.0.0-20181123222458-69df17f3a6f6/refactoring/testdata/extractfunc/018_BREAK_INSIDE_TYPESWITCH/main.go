//<<<<<extract,9,2,21,2,Foo,pass
package main

import "fmt"

func main() {
	var x interface{}
	x = 77.0 + 0
	switch i := x.(type) {
	case int:
		fmt.Println("INT", i)
	case float64:
		fmt.Println("FLOAT64", i)
		if i == 77.0 {
			break
		}
	case string:
		fmt.Printf("STRING", i)
	default:
		fmt.Println("Unknown type. Sorry!")
	}
}
