package main

import "fmt"

func main() {
	x := 5.0
	if i, err := x, false; i < 10 { // <<<<< var,7,24,7,30,newVar,fail
		if err == true {
			fmt.Println(err)
		}
		fmt.Println("divisible by 5:")
	}
}
