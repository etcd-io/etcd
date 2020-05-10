package main

import "fmt"

func main() {
	x := 5.0
	if i, err := x, false; i < 10 { // <<<<< var,7,25,7,31,i < 10,newVar,fail
		if err == true {
			fmt.Println(err)
		}
		fmt.Println("divisible by 5:")
	}
	if i, err := x+5, false; i > 5 {
		if err == true {
			fmt.Println("adskf")
		}
	}
	if i, err := x+3, false; i > 6 {
		if err == true {
			fmt.Println("adskf")
		}
	}
}
