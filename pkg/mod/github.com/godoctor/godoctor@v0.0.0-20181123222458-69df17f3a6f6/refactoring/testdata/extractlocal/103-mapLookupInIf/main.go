package main

import "fmt"

func main() {
	m := map[int]int{1: 11, 2: 22}
	if v, found := m[2]; found { // <<<<< var,7,17,7,20,newVar,fail
		fmt.Println(v)
	}
}
