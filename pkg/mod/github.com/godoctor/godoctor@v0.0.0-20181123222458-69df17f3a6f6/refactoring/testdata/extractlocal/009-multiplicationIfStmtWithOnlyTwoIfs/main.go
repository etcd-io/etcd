package main

import "fmt"

func main() {
	a := 1
	b := 2
	c := 3
	if a < c {
		fmt.Println("")
	}
	if a*b < c { // <<<<< var,12,4,12,7,newVar,pass
		fmt.Println("")
	}
}
