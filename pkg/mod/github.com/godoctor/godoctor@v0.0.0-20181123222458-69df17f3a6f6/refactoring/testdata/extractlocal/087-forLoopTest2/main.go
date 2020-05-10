package main

import (
	"fmt"
	"math"
)

func main() {
	slicer := make([]int, 10)
	for x := 2; x < len(slicer); x++ { // <<<<< var,10,31,10,34,newVar,fail
		x++
	}
	if math.Mod(10, 5) == 0 {
		fmt.Println("divisible by 5:")
	}
}
