package main

import (
	"fmt"
	"math"
)

func main() {
	y := 2.0
	var x float64
	if math.Mod(y, 5.0) == 0 {
		x = y // <<<<< var,12,3,12,4,newVar,fail
		fmt.Println("divisible by 5:")
		if x > y {
			fmt.Println("")
		}
	}
}
