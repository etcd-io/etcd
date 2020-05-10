package main

import "fmt"

func main() {

	var apple math

	apple.x = 10
	apple.y = 5

	switch {
	case apple.y < apple.x: // <<<<< var,13,13,13,13,newVar,fail
		fmt.Printf("this is a test and only a test :D")
	case apple.y > 10:
		fmt.Printf("welcome to crazy town")
	default:
	}
}

type math struct {
	x int
	y int
}
