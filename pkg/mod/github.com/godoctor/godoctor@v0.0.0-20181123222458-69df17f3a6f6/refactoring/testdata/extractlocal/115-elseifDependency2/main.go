package main

import "fmt"

func main() {
	x := 1
	if x != 1 {
	} else if x = 2; x != 2 {
	} else if x == 2 { // <<<<< var,9,12,9,13,newVar,fail
		fmt.Println("Success")
	}
	// This should not pass since the extracted "x" is declared in the
	// first else-if block.
}
