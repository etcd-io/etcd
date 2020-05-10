package main

import "fmt"

func main() {
	apple := 10

	if apple < 10 {
		goto L //<<<<< var,9,3,9,7,newVar,fail
		apple = 2
	} else {
		goto J
	}
L:
	fmt.Printf("apple is less than 10")

J:
	fmt.Printf("apple is greater than 10")
}
