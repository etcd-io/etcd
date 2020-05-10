package main

import (
	"fmt"
	"strconv"
)

func main() {
	x := 2
	y := 5
	var choice string
	fmt.Println("please choose: x + x, x * y?")
	fmt.Scanf(choice)
	iChoice, _ := strconv.Atoi(choice)
	switch iChoice {
	case x + x, x + y, x - y: // <<<<< var,16,14,16,19,newVar,pass
		fmt.Println(x + x)
	case x * y:
		fmt.Println(x * y)
	default:
		fmt.Println("didn't work")
	}
}
