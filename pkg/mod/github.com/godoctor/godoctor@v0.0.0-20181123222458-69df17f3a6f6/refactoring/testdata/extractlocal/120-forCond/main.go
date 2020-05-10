package main

import (
	"fmt"
)

func main() {
	y := 1
	z := 2
	for x := 2 + y; x < y+z; x++ { //<<<<< var,10,22,10,24,newVar,pass
		x = x + 1 // Reassigning x does not affect the condition
		y := 1000 // This y shadows y in the condition
		fmt.Printf("x = %d and local y = %d\n", x, y)
	}
	fmt.Printf("y = %d\n", y)
}
