// <<<<< extract,10,2,10,13,Square,pass
package main

import "fmt"

func main() {
	x := 1.5
	fmt.Println("x", x)
	y := &x
	*y = *y * *y
	fmt.Println("square(x)", *y)
}
