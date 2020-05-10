package main

import "fmt"

func main() {
	y := 6
	x := make([]string, 5)
	if y == 6 {
		for _, name := range x {
			value, exists := isThere(name)
			if exists { // <<<<< var,11,7,11,13,newVar,pass
				fmt.Println("there's a name in there")
			}
			if value != "" {
				fmt.Println("nothing there")
			}
		}
	} else {
		fmt.Println("double if stmt")
	}
}

func isThere(a string) (string, bool) {
	if a != "" {
		return a, true
	}
	return a, false
}
