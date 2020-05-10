package main

import "fmt"

func main() {
	x := make([]string, 5)
	for _, name := range x { // <<<<< var,7,6,7,13,newVar,fail
		if name != "" {
			fmt.Println("there's a name in there")
		}
	}
}
