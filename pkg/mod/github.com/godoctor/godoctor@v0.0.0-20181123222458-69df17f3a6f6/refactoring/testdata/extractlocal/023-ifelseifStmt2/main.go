package main

import "fmt"

func main() {
	x := 5
	if x < 0 {

	} else if x > 18 {
		fmt.Println("divisible by 5:")
	} else if x < 10 { // <<<<< var,11,11,11,17,newVar,pass
		fmt.Println("divisible by 5:")
	}
}
