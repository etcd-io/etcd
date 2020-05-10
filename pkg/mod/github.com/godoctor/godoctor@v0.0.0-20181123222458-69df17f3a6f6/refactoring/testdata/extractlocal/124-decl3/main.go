package main

import "fmt"

var x = []int{1 + 2, 4, 5} //<<<<< var,5,15,5,19,newVar,fail

func main() {
	fmt.Println(x)
}
