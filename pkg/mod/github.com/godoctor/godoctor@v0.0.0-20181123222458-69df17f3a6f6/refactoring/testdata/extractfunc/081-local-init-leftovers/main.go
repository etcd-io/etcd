// <<<<< extract,9,2,11,38,changeB,fail
package main

import "fmt"

func main() {
	a := 5
	b := 4
	fmt.Println("Value of a and b", a, b)
	b = 77
	fmt.Println("Modified Value of b", b)
}
