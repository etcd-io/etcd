//<<<<< extract,8,1,19,2,newFunc,pass
package main

import "fmt"

func main() {
	x := 1 + 0
d:
	for {
		switch {
		case x < 0:
			fmt.Println("Value of x is greater than 0")
		case x == 1:
			fmt.Println("X is 1")
			break d
		default:
			fmt.Println("X is not greater than 0")
		}
	}
}
