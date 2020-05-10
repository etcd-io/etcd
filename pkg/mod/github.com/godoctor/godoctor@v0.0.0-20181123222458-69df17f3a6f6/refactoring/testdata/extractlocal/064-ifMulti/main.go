package main

import "fmt"

func main() {
	x := 2
	y := 5
	if z := x + y; z != 0 { //<<<<< var,8,22,8,23,newVar,pass
		fmt.Println("got this test to work right.")
	}
	fmt.Println("please choose: x + x, x * y?")
	fmt.Printf("x is %s, y is %s", x, y)
}
