package main

import (
	"fmt"
)

func main() {
	y := 1
	for x := 2 + y; x < 5; x++ { //<<<<< var,9,15,9,15,newVar,pass
		y++
	}
	fmt.Printf("y = %d\n", y)
}
