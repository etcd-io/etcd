package main

import (
	"fmt"
)

func main() {
	y := 1
	for x := 2 + y; x < y; x++ { //<<<<< var,9,22,9,23,newVar,fail
		if false {
			y++ // Dataflow analysis thinks this might execute
		}
	}
	fmt.Printf("y = %d\n", y)
}
