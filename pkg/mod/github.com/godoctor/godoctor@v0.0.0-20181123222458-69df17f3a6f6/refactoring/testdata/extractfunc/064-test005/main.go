//<<<<<extract,12,2,12,19,foo,pass
package main

import "fmt"

func plus(a int, b int) int {
	return a + b
}

func main() {
	fmt.Println("Before calling the function")
	res := plus(1, 2)
	fmt.Println("1+2 =", res)
}
