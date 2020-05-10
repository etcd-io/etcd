package main

import "fmt"

func main() {
	var1 := 1
	var2 := 2 // <<<<< var,7,10,7,10,var1,fail
	fmt.Printf("%d %d\n", var1, var2)
}
