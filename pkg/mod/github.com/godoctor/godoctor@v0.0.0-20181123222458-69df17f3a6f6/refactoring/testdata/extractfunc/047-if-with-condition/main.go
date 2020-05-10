//<<<<<extract,9,2,15,2,foo,pass
package main

import "fmt"

func main() {
	a := 7 + 0
	b := 5 + 0
	b = b + 2
	if a == b {
		fmt.Println("a and b are equal")
		b = b + 1
	} else {
		fmt.Println("a is not equal to b")
	}
	fmt.Println("the new value of b is ", b)
}
