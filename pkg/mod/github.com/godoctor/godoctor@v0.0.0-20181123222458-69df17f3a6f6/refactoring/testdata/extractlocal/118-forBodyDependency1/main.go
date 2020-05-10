package main

import (
	"fmt"
)

func main() {
	y := 1
	for x := 2 + y; x < y; x++ { //<<<<< var,9,22,9,23,newVar,fail
		y++ // Changes the value of y in the condition expression
	}
	fmt.Printf("y = %d\n", y)
}
