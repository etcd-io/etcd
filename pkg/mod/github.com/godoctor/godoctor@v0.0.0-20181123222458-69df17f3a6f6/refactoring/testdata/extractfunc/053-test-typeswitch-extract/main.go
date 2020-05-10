//<<<<< extract,9,2,18,2,newFunc,pass
package main

import "fmt"

func main() {
	var x interface{}
	x = 77.0 + 0
	switch x.(type) {
	case int:
		fmt.Println("INT")
	case float64:
		fmt.Println("FLOAT64")
	case string:
		fmt.Printf("STRING")
	default:
		fmt.Println("Unknown type. Sorry!")
	}
}
