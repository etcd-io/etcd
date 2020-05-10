package main

import (
	"fmt"
	"strconv"
)

func main() {
	fieldBase := uint64(2)
	n := []int64{1, 2, 4, 5, 6, 7, 5, 8}
	m := uint64(n[0]) * uint64(n[0])
	m = (m + fieldBase) +
		2*uint64(n[0])*uint64(n[6]) + // <<<<< var,13,3,13,4,newVar,pass

		2*uint64(n[1])*uint64(n[5]) +
		2*uint64(n[2])*uint64(n[4]) +
		uint64(n[3])*uint64(n[3])
	fmt.Println("number = " + strconv.FormatUint(m, int(fieldBase)))
}
