package main

import "fmt"

func main() {
	a := 1
	b := 2
	c := 3
	if a+b < c { // <<<<< var,9,4,9,7,newVar,pass
		fmt.Println("")
	}
}
