package main

import (
	"fmt"
)

func main() {
	y := 1
	for x := 2 + y; x < 5; x++ { //<<<<< var,9,25,9,27,newVar,fail
		y++
	}
	fmt.Printf("y = %d\n", y)
}
