package main

import "fmt"

func main() {
	x := 5
	if x < 10 { // <<<<< var,7,5,7,12,newVar,fail
		fmt.Println("divisible by 5:")
	}
}
