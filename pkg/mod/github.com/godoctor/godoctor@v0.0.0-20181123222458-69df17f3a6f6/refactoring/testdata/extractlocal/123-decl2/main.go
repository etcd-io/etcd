package main

import "fmt"

func main() {
	// This should be extractable but is currently not allowed
	var x []int = []int{1 + 2, 4, 5} //<<<<< var,6,22,6,26,newVar,fail
	fmt.Println(x)
}
