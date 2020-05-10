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
	case x + x:
		fmt.Println(x + x)
	case x * y: // <<<<< var,18,6,18,10,newVar,pass
		fmt.Println(x * y)
	default:
		fmt.Println("didn't work")
	}
}
