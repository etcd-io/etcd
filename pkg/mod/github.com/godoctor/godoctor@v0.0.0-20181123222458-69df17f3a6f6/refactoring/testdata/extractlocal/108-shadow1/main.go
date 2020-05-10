package main

import "fmt"

func fn() {
}

func main() {
	var1 := 1
	var2 := 2 // <<<<< var,10,10,10,10,fn,fail
	fmt.Printf("%d %d\n", var1, var2)
}
