package main

import "fmt"

func main() {
	m := map[int]int{1: 2, 3: 4}
	for _, v := range m {
		// <<<<< var,7,6,7,6,variable,fail
		// <<<<< var,7,9,7,9,variable,fail
		fmt.Printf("%d\n", v)
	}
}
