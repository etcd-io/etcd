package main

import "fmt"

func main() {
	y := 2
	if y != 0 {
		x := 5
		if x < 10 { // <<<<< var,9,5,9,6,newVar,pass
			fmt.Println("divisible by 5:")
		}
	}
}
