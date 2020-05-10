//<<<<<extract,9,1,21,2,Foo,pass
package main

import "fmt"

func main() {
	y := 5
	fmt.Println(y)
ABC:
	for {
		x := 1
		switch {
		case x > 0:
			fmt.Println("GREATER THAN 0")
			break ABC
		case x == 1:
			fmt.Println("EQUAL TO 1")
		default:
			fmt.Println("DEFAULT")
		}
	}
}
