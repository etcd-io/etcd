package main

import "fmt"

func main() {
	apple := 10
	orange := 3

	average(apple, orange)
}

func average(a int, b int) {
	if a == b { //<<<<< var,13,5,13,5,newVar,pass
		fmt.Printf("a*b == %i", a*b)
	}
}
