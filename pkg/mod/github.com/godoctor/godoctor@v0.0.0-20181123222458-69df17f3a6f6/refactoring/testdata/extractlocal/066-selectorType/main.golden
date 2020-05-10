package main

import "fmt"

func main() {
	var p *X
	p.y = 10
	p.name = "bob"
	apple := p.name //<<<<< var,9,13,9,17,newVar,fail
	if p.name != "" {
		fmt.Printf("this is a selector: %s", apple)
	}
}

type X struct {
	y    int
	name string
}
