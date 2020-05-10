package main

import "fmt"

func main() {
	x := 5
	if x < 0 {

	} else if x < 10 { // <<<<< var,9,11,9,17,newVar,pass
		fmt.Println("divisible by 5:")
	}
}
