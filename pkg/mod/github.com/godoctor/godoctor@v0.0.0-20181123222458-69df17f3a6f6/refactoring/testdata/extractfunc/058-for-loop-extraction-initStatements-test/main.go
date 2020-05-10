//<<<<<extract,8,3,17,3,newFunction,pass
package main

import "fmt"

func main() {
	for i := 0; i < 3; i++ {
		if num := 9; num < 10 {
			A := num
			fmt.Println(num, "is negative")
			A += i
			fmt.Println("Value of A is", A, "\n")
		} else if num < 0 {
			fmt.Println(num, "has 1 digit")
		} else {
			fmt.Println(num, "has multiple digits")
		}
	}
}
