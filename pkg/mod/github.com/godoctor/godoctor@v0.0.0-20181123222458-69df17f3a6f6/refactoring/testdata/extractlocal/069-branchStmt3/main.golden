package main

import "fmt"

func main() {
	apple := 10
	orange := 3

AppleLoop:
	for i := 0; i < apple; i++ {
		for index2 := 0; index2 < orange; index2++ {
			if index2 == 3 {
				continue AppleLoop //<<<<< var,13,5,13,13,newVar,fail
			}
			orange++
		}
	}
	fmt.Printf("the apple is: %i and the orange is %i", apple, orange)
}
