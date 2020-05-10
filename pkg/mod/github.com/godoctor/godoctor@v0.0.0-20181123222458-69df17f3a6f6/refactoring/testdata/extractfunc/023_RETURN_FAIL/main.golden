//<<<<<extract,13,3,16,3,Foo,fail
package main

import "fmt"

func main() {
	j := 4
	switch j {
	case 1, 3, 5, 7, 9:
		fmt.Println(j, "is odd")
	case 0, 2, 4, 6, 8:
		fmt.Println(j, "is even")
		if j == 4 {
			fmt.Println("ABOVE RETURN")
			return
		}
		fallthrough
	default:
		fmt.Println("DEFAULT STATEMENT EXECUTED!")
	}
	fmt.Println("this is in MAIN")
}
