//<<<<<extract,9,2,11,6,Foo,pass
package main

import "fmt"

func main() {
	a := 3
	b := 4
	b = 5
	fmt.Println("The variable b is:", b)
	b = 7
	fmt.Println("the new value of b", b)
	fmt.Println("The variable a is:", a)
}
