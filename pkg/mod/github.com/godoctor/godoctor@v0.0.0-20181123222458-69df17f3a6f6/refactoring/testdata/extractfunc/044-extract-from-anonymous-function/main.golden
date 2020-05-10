//<<<<<extract,10,3,11,42,increment,fail
package main

import "fmt"

func main() {
	x := 5
	fn := func() {
		fmt.Println("x is", x)
		x = x + 20
		fmt.Println("the new value of x is ", x)
	}
	fn()
	x++
	fn()
}
