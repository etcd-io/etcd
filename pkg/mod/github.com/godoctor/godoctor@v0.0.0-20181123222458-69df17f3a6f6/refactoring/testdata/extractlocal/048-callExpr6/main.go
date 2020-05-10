package main

import "fmt"

func main() {
	fmt.Println("Hello, 世界")
}

type Apple struct {
}

func (a *Apple) length() string {
	return "gets length"
}

func (a *Apple) orange() {
	c := ""
	b := (*a).length() // <<<<< var,18,7,18,10,newVar,pass
	if c == b {
		fmt.Println("ka;ldskjf")
	}
}
