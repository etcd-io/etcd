//<<<<<extract,11,3,12,5,foo,pass
package main

import "fmt"

func main() {
	a := 7
	b := 5
	b = b + 2
	if a == b {
		fmt.Println("a and b are equal")
		b = b + 1
	} else {
		fmt.Println("a is not equal to b")
	}
	fmt.Println("the new value of b is ", b)
}
