package main

import "fmt"

func main() {
	a := 1
	b := -2
	n := a + b
	if n < a { // <<<<< var,9,4,9,9,newVar,pass
		a = 5
		fmt.Println(a)
	}
	fmt.Println(a)
	a = 10
	fmt.Println(a)
}
