package main

import "fmt"

func main() {
	x := 5
	if x < 10 { // <<<<< var,7,5,7,6,newVar,pass
		fmt.Println("divisible by 5:")
	}
}
