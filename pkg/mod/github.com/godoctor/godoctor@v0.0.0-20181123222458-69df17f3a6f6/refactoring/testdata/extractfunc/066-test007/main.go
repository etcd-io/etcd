//<<<<<extract,8,2,17,2,newFunction,fail
package main

import "fmt"

func main() {
	x := 5
	for i := 0; i < 3; i++ {
		fn := func(key int) {
			fmt.Println("x is", x, "key is ", key)
			x = x + 20
			fmt.Println("the new value of x is ", x)
		}
		fn(333)
		x++
		fn(555)
	}
}
